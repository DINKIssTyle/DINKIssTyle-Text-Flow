//go:build windows

package windowing

import (
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const swRestore = 9

var (
	focusUser32                  = syscall.NewLazyDLL("user32.dll")
	procFocusGetForegroundWindow = focusUser32.NewProc("GetForegroundWindow")
	procFocusGetWindowThreadPID  = focusUser32.NewProc("GetWindowThreadProcessId")
	procFocusAttachThreadInput   = focusUser32.NewProc("AttachThreadInput")
	procFocusShowWindowAsync     = focusUser32.NewProc("ShowWindowAsync")
	procFocusBringWindowToTop    = focusUser32.NewProc("BringWindowToTop")
	procFocusSetForegroundWindow = focusUser32.NewProc("SetForegroundWindow")
	procFocusSetActiveWindow     = focusUser32.NewProc("SetActiveWindow")
	procFocusSetFocus            = focusUser32.NewProc("SetFocus")
)

func ActivateForInput(window application.Window) {
	nativeWindow := window.NativeWindow()
	if nativeWindow == nil {
		return
	}
	targetWindow := uintptr(nativeWindow)
	foregroundWindow, _, _ := procFocusGetForegroundWindow.Call()
	foregroundThread, _, _ := procFocusGetWindowThreadPID.Call(foregroundWindow, 0)
	targetThread, _, _ := procFocusGetWindowThreadPID.Call(targetWindow, 0)

	attached := foregroundThread != 0 && targetThread != 0 && foregroundThread != targetThread
	if attached {
		procFocusAttachThreadInput.Call(foregroundThread, targetThread, 1)
		defer procFocusAttachThreadInput.Call(foregroundThread, targetThread, 0)
	}

	procFocusShowWindowAsync.Call(targetWindow, swRestore)
	procFocusBringWindowToTop.Call(targetWindow)
	procFocusSetForegroundWindow.Call(targetWindow)
	procFocusSetActiveWindow.Call(targetWindow)
	procFocusSetFocus.Call(targetWindow)
}
