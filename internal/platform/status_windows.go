//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	processQueryLimitedInformation = 0x1000
	keyeventfKeyup                 = 0x0002
	vkControl                      = 0x11
	vkShift                        = 0x10
	vkMenu                         = 0x12
	vkC                            = 0x43
	vkV                            = 0x56
	vkLWin                         = 0x5B
	vkRWin                         = 0x5C
	hotkeyReleaseTimeout           = 750 * time.Millisecond
	clipboardCopyTimeout           = 500 * time.Millisecond
	clipboardPollInterval          = 20 * time.Millisecond
	clipboardPasteSettleDuration   = 200 * time.Millisecond
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procEnumWindows            = user32.NewProc("EnumWindows")
	procIsWindowVisible        = user32.NewProc("IsWindowVisible")
	procShowWindowAsync        = user32.NewProc("ShowWindowAsync")
	procKeybdEvent             = user32.NewProc("keybd_event")
	procGetAsyncKeyState       = user32.NewProc("GetAsyncKeyState")
	procOpenProcess            = kernel32.NewProc("OpenProcess")
	procCloseHandle            = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImage  = kernel32.NewProc("QueryFullProcessImageNameW")
	enumWindowForPIDCallback   = syscall.NewCallback(enumWindowForPID)
)

func CurrentStatus() Status {
	pid := getFrontmostPID()
	info := appInfoFromProcess(pid)
	return Status{
		AccessibilityTrusted: true,
		SecureInputActive:    false,
		ActiveAppName:        info.Name,
		ActiveBundleID:       info.BundleID,
		FlowEngineRunning:    false,
		Message:              "Windows support is active. Some automation features may depend on the focused application.",
	}
}

func requestAccessibilityPermission() bool {
	return true
}

func selectedText() (string, error) {
	return selectedTextFromProcess(getFrontmostPID())
}

func selectedTextFromProcess(processID int) (string, error) {
	waitForModifierKeysReleased(hotkeyReleaseTimeout)
	if processID > 0 {
		_ = activateProcess(processID)
	}

	previous, previousErr := readClipboardText()
	if err := writeClipboardText(""); err != nil {
		return "", err
	}
	sendCtrlKey(vkC)
	selected, err := waitForClipboardText(clipboardCopyTimeout)
	if previousErr == nil {
		_ = writeClipboardText(previous)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(selected), nil
}

func replaceSelectedTextInProcess(processID int, replacement string, preferPaste bool) error {
	if processID <= 0 {
		return nil
	}

	previous, previousErr := readClipboardText()
	if err := writeClipboardText(replacement); err != nil {
		return err
	}
	_ = activateProcess(processID)
	sendCtrlKey(vkV)
	time.Sleep(clipboardPasteSettleDuration)
	if previousErr == nil {
		_ = writeClipboardText(previous)
	}
	return nil
}

func activateProcess(processID int) error {
	hwnd := windowForPID(processID)
	if hwnd != 0 {
		procShowWindowAsync.Call(hwnd, 9)
		activated, _, _ := procSetForegroundWindow.Call(hwnd)
		if activated != 0 {
			time.Sleep(60 * time.Millisecond)
			return nil
		}
	}
	cmd := HideCommandWindow(exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		fmt.Sprintf("(New-Object -ComObject WScript.Shell).AppActivate(%d) | Out-Null", processID),
	))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("activate process %d: %w", processID, err)
	}
	time.Sleep(60 * time.Millisecond)
	return nil
}

func appInfoFromProcess(processID int) AppInfo {
	if processID <= 0 {
		return AppInfo{}
	}
	path := processPath(processID)
	if path == "" {
		return AppInfo{}
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return AppInfo{
		Name:     name,
		BundleID: strings.ToLower(filepath.Base(path)),
		Path:     path,
	}
}

func appInfoFromBundlePath(path string) AppInfo {
	path = strings.TrimSpace(path)
	if path == "" {
		return AppInfo{}
	}
	return AppInfo{
		Name:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		BundleID: strings.ToLower(filepath.Base(path)),
		Path:     path,
	}
}

func getFrontmostPID() int {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0
	}
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return int(pid)
}

type windowSearch struct {
	processID uint32
	hwnd      uintptr
}

func windowForPID(processID int) uintptr {
	if processID <= 0 {
		return 0
	}
	search := windowSearch{processID: uint32(processID)}
	procEnumWindows.Call(
		enumWindowForPIDCallback,
		uintptr(unsafe.Pointer(&search)),
	)
	runtime.KeepAlive(&search)
	return search.hwnd
}

func enumWindowForPID(hwnd uintptr, searchPtr uintptr) uintptr {
	search := (*windowSearch)(unsafe.Pointer(searchPtr))
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible == 0 {
		return 1
	}
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != search.processID {
		return 1
	}
	search.hwnd = hwnd
	return 0
}

func processPath(processID int) string {
	handle, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(uint32(processID)))
	if handle == 0 {
		return ""
	}
	defer procCloseHandle.Call(handle)

	buffer := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buffer))
	r1, _, _ := procQueryFullProcessImage.Call(
		handle,
		0,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 || size == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer[:size])
}

func sendCtrlKey(key uintptr) {
	procKeybdEvent.Call(vkControl, 0, 0, 0)
	procKeybdEvent.Call(key, 0, 0, 0)
	procKeybdEvent.Call(key, 0, keyeventfKeyup, 0)
	procKeybdEvent.Call(vkControl, 0, keyeventfKeyup, 0)
}

func waitForModifierKeysReleased(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for modifierKeyDown() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
}

func modifierKeyDown() bool {
	for _, key := range []uintptr{vkControl, vkShift, vkMenu, vkLWin, vkRWin} {
		state, _, _ := procGetAsyncKeyState.Call(key)
		if state&0x8000 != 0 {
			return true
		}
	}
	return false
}

func readClipboardText() (string, error) {
	return ReadClipboardText()
}

func writeClipboardText(text string) error {
	return WriteClipboardText(text)
}

func waitForClipboardText(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		text, err := readClipboardText()
		if err == nil && text != "" {
			return text, nil
		}
		if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return "", lastErr
			}
			return "", nil
		}
		time.Sleep(clipboardPollInterval)
	}
}
