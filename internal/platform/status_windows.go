//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"dkst-text-flow/internal/winclipboard"
)

const (
	processQueryLimitedInformation = 0x1000
	keyeventfKeyup                 = 0x0002
	vkControl                      = 0x11
	vkC                            = 0x43
	vkV                            = 0x56
)

var (
	user32                     = syscall.NewLazyDLL("user32.dll")
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procKeybdEvent             = user32.NewProc("keybd_event")
	procOpenProcess            = kernel32.NewProc("OpenProcess")
	procCloseHandle            = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImage  = kernel32.NewProc("QueryFullProcessImageNameW")
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
	if processID > 0 {
		_ = activateProcess(processID)
	}

	previous, previousErr := readClipboardText()
	sendCtrlKey(vkC)
	time.Sleep(140 * time.Millisecond)
	selected, err := readClipboardText()
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
	time.Sleep(160 * time.Millisecond)
	if previousErr == nil {
		_ = writeClipboardText(previous)
	}
	return nil
}

func activateProcess(processID int) error {
	hwnd := foregroundWindowForPID(processID)
	if hwnd != 0 {
		procSetForegroundWindow.Call(hwnd)
		time.Sleep(60 * time.Millisecond)
		return nil
	}
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-Command",
		fmt.Sprintf("(New-Object -ComObject WScript.Shell).AppActivate(%d) | Out-Null", processID),
	)
	_ = cmd.Run()
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

func foregroundWindowForPID(processID int) uintptr {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0
	}
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if int(pid) == processID {
		return hwnd
	}
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

func readClipboardText() (string, error) {
	return winclipboard.ReadText()
}

func writeClipboardText(text string) error {
	return winclipboard.WriteText(text)
}
