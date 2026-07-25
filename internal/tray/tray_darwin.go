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

func New(_ *application.App, icon []byte, actions Actions) *Manager {
	manager := &Manager{actions: actions}
	activeManager = manager
	if len(icon) == 0 {
		return manager
	}
	manager.native = C.dkstSystemTrayCreate(
		(*C.uchar)(unsafe.Pointer(&icon[0])),
		C.int(len(icon)),
	)
	return manager
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
		call(manager.actions.Quit)
	}
}
