//go:build darwin

package tray

/*
#cgo CFLAGS: -mmacosx-version-min=12.0 -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "systemtray_darwin.h"
*/
import "C"

import (
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Manager struct {
	actions Actions
	native  unsafe.Pointer
}

var activeManager *Manager

func New(_ *application.App, activeIcon []byte, pausedIcon []byte, actions Actions) *Manager {
	manager := &Manager{actions: actions}
	activeManager = manager
	if len(activeIcon) == 0 {
		return manager
	}
	var pausedIconBytes *C.uchar
	if len(pausedIcon) > 0 {
		pausedIconBytes = (*C.uchar)(unsafe.Pointer(&pausedIcon[0]))
	}
	manager.native = C.dkstSystemTrayCreate(
		(*C.uchar)(unsafe.Pointer(&activeIcon[0])),
		C.int(len(activeIcon)),
		pausedIconBytes,
		C.int(len(pausedIcon)),
	)
	return manager
}

func (m *Manager) UpdateState(state State) {
	if m == nil || m.native == nil {
		return
	}
	C.dkstSystemTrayUpdateState(
		m.native,
		C.int(boolInt(state.FlowPaused)),
		C.int(boolInt(state.Running)),
		C.int(boolInt(state.OCREnabled)),
	)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (m *Manager) Destroy() {
	if m == nil {
		return
	}
	if m.native != nil {
		C.dkstSystemTrayDestroy(m.native)
		m.native = nil
	}
	if activeManager == m {
		activeManager = nil
	}
}

//export dkstSystemTrayMenuSelected
func dkstSystemTrayMenuSelected(itemID C.int) {
	manager := activeManager
	if manager == nil {
		return
	}
	switch int(itemID) {
	case 1:
		call(manager.actions.AskAI)
	case 2:
		call(manager.actions.ShowMainWindow)
	case 3:
		call(manager.actions.ToggleFlow)
	case 4:
		call(manager.actions.Quit)
	case 5:
		call(manager.actions.OCR)
	}
}
