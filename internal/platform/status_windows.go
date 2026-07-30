//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	winapi "golang.org/x/sys/windows"
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
	swRestore                      = 9
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
	procIsIconic               = user32.NewProc("IsIconic")
	procShowWindowAsync        = user32.NewProc("ShowWindowAsync")
	procKeybdEvent             = user32.NewProc("keybd_event")
	procGetAsyncKeyState       = user32.NewProc("GetAsyncKeyState")
	procOpenProcess            = kernel32.NewProc("OpenProcess")
	procCloseHandle            = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImage  = kernel32.NewProc("QueryFullProcessImageNameW")
	enumWindowForPIDCallback   = syscall.NewCallback(enumWindowForPID)
	ole32                      = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx         = ole32.NewProc("CoInitializeEx")
	procCoUninitialize         = ole32.NewProc("CoUninitialize")
	procCoCreateInstance       = ole32.NewProc("CoCreateInstance")

	clsidCUIAutomation = syscall.GUID{
		Data1: 0xff48dba4, Data2: 0x605d, Data3: 0x4e70,
		Data4: [8]byte{0x99, 0xbe, 0x01, 0x36, 0xc3, 0xd5, 0x00, 0x24},
	}
	iidIUIAutomation = syscall.GUID{
		Data1: 0x30c3d08a, Data2: 0xae8d, Data3: 0x4f77,
		Data4: [8]byte{0xa8, 0xa7, 0xa5, 0x77, 0x05, 0xc7, 0xec, 0xd8},
	}
	replacementClipboardState = struct {
		sync.Mutex
		generation    uint64
		snapshot      string
		snapshotValid bool
		pending       bool
	}{}
)

func CurrentStatus() Status {
	pid := getFrontmostPID()
	info := appInfoFromProcess(pid)
	return Status{
		AccessibilityTrusted:   true,
		ScreenRecordingGranted: true,
		SecureInputActive:      false,
		ActiveAppName:          info.Name,
		ActiveBundleID:         info.BundleID,
		FlowEngineRunning:      false,
		Message:                "Windows support is active. Some automation features may depend on the focused application.",
	}
}

func requestAccessibilityPermission() bool {
	return true
}

func requestScreenRecordingPermission() bool {
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

	replacementClipboardState.Lock()
	defer replacementClipboardState.Unlock()

	previous, previousErr := readClipboardText()
	if err := writeClipboardText(replacement); err != nil {
		return err
	}
	_ = activateProcess(processID)
	sendCtrlKey(vkV)
	replacementClipboardState.generation++
	generation := replacementClipboardState.generation
	if !replacementClipboardState.pending {
		replacementClipboardState.snapshot = previous
		replacementClipboardState.snapshotValid = previousErr == nil
		replacementClipboardState.pending = true
	}
	go restoreReplacementClipboard(generation, replacement)
	return nil
}

func insertTextAtCursorInProcess(processID int, text string) error {
	return replaceSelectedTextInProcess(processID, text, true)
}

func restoreReplacementClipboard(generation uint64, replacement string) {
	time.Sleep(clipboardPasteSettleDuration)

	replacementClipboardState.Lock()
	defer replacementClipboardState.Unlock()
	if generation != replacementClipboardState.generation {
		return
	}
	snapshot := replacementClipboardState.snapshot
	snapshotValid := replacementClipboardState.snapshotValid
	replacementClipboardState.snapshot = ""
	replacementClipboardState.snapshotValid = false
	replacementClipboardState.pending = false

	current, err := readClipboardText()
	if snapshotValid && err == nil && current == replacement {
		_ = writeClipboardText(snapshot)
	}
}

func activateProcess(processID int) error {
	if processID <= 0 {
		return nil
	}
	if getFrontmostPID() == processID {
		return nil
	}

	hwnd := windowForPID(processID)
	if hwnd != 0 {
		minimized, _, _ := procIsIconic.Call(hwnd)
		if minimized != 0 {
			procShowWindowAsync.Call(hwnd, swRestore)
		}
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
		name := processNameFromSnapshot(processID)
		if name == "" {
			return AppInfo{}
		}
		return AppInfo{
			Name:     strings.TrimSuffix(name, filepath.Ext(name)),
			BundleID: strings.ToLower(name),
		}
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

// processNameFromSnapshot keeps app-specific matching available when Windows
// denies full path access, which can happen across integrity levels (for
// example, when the focused app is running as administrator).
func processNameFromSnapshot(processID int) string {
	if processID <= 0 {
		return ""
	}
	snapshot, err := winapi.CreateToolhelp32Snapshot(winapi.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer winapi.CloseHandle(snapshot)

	entry := winapi.ProcessEntry32{Size: uint32(unsafe.Sizeof(winapi.ProcessEntry32{}))}
	if err := winapi.Process32First(snapshot, &entry); err != nil {
		return ""
	}
	for {
		if entry.ProcessID == uint32(processID) {
			return syscall.UTF16ToString(entry.ExeFile[:])
		}
		if err := winapi.Process32Next(snapshot, &entry); err != nil {
			return ""
		}
	}
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

func getVTable(pUnk uintptr) *[100]uintptr {
	if pUnk == 0 {
		return nil
	}
	return (*[100]uintptr)(*(*unsafe.Pointer)(unsafe.Pointer(pUnk)))
}

func isFocusedElementEditableForProcess(processID int) bool {
	hwnd := windowForPID(processID)
	if hwnd == 0 {
		return false
	}
	var threadID uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&threadID)))
	if threadID == 0 {
		return false
	}

	type guiThreadInfo struct {
		cbSize        uint32
		flags         uint32
		hwndActive    uintptr
		hwndFocus     uintptr
		hwndCapture   uintptr
		hwndMenuOwner uintptr
		hwndMoveSize  uintptr
		hwndCaret     uintptr
		rcCaret       [4]int32
	}

	var info guiThreadInfo
	info.cbSize = uint32(unsafe.Sizeof(info))
	procGetGUIThreadInfo := user32.NewProc("GetGUIThreadInfo")
	ret, _, _ := procGetGUIThreadInfo.Call(uintptr(threadID), uintptr(unsafe.Pointer(&info)))
	if ret != 0 && info.hwndCaret != 0 {
		return true
	}

	const (
		clsctxInprocServer = 1
		coinitApartment    = 2
		uiaValuePatternID  = 10002
	)

	hr, _, _ := procCoInitializeEx.Call(0, coinitApartment)
	if int32(hr) >= 0 {
		defer procCoUninitialize.Call()

		var pAutomation uintptr
		hrCreate, _, _ := procCoCreateInstance.Call(
			uintptr(unsafe.Pointer(&clsidCUIAutomation)),
			0,
			clsctxInprocServer,
			uintptr(unsafe.Pointer(&iidIUIAutomation)),
			uintptr(unsafe.Pointer(&pAutomation)),
		)

		if int32(hrCreate) >= 0 && pAutomation != 0 {
			defer func() {
				vt := getVTable(pAutomation)
				if vt != nil {
					syscall.SyscallN(vt[2], pAutomation)
				}
			}()

			var pElement uintptr
			vtAuto := getVTable(pAutomation)
			hrFocused, _, _ := syscall.SyscallN(vtAuto[7], pAutomation, uintptr(unsafe.Pointer(&pElement)))

			if int32(hrFocused) >= 0 && pElement != 0 {
				defer func() {
					vtElem := getVTable(pElement)
					if vtElem != nil {
						syscall.SyscallN(vtElem[2], pElement)
					}
				}()

				var pValuePattern uintptr
				vtElem := getVTable(pElement)
				hrPattern, _, _ := syscall.SyscallN(vtElem[17], pElement, uiaValuePatternID, uintptr(unsafe.Pointer(&pValuePattern)))

				if int32(hrPattern) >= 0 && pValuePattern != 0 {
					defer func() {
						vtVal := getVTable(pValuePattern)
						if vtVal != nil {
							syscall.SyscallN(vtVal[2], pValuePattern)
						}
					}()

					var isReadOnly int32 = 1
					vtVal := getVTable(pValuePattern)
					hrReadOnly, _, _ := syscall.SyscallN(vtVal[5], pValuePattern, uintptr(unsafe.Pointer(&isReadOnly)))

					if int32(hrReadOnly) >= 0 {
						return isReadOnly == 0
					}
				}
			}
		}
	}

	if info.hwndFocus != 0 {
		procGetWindowLong := user32.NewProc("GetWindowLongW")
		const gwlStyle = ^uintptr(15)
		const esReadonly = 0x0800
		style, _, _ := procGetWindowLong.Call(info.hwndFocus, gwlStyle)
		if style != 0 && (style&esReadonly) != 0 {
			return false
		}
	}

	return false
}
