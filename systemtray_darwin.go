//go:build darwin

package main

/*
#cgo CFLAGS: -mmacosx-version-min=12.0 -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "systemtray_darwin.h"
*/
import "C"

import (
	"unsafe"

	"dkst-text-flow/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var nativeSystemTrayApp *App
var nativeSystemTray unsafe.Pointer

func (a *App) configureSystemTray(_ *application.App) {
	nativeSystemTrayApp = a
	if len(menuIcon) == 0 {
		return
	}
	nativeSystemTray = C.dkstSystemTrayCreate(
		(*C.uchar)(unsafe.Pointer(&menuIcon[0])),
		C.int(len(menuIcon)),
	)
}

func (a *App) destroySystemTray() {
	if nativeSystemTray != nil {
		C.dkstSystemTrayDestroy(nativeSystemTray)
		nativeSystemTray = nil
	}
	nativeSystemTrayApp = nil
}

//export dkstSystemTrayMenuSelected
func dkstSystemTrayMenuSelected(itemID C.int) {
	a := nativeSystemTrayApp
	if a == nil {
		return
	}
	switch int(itemID) {
	case 1:
		go a.showAIPrompt(platform.GetFrontmostPID(), false)
	case 2:
		go a.showMainWindow()
	case 3:
		go application.Get().Quit()
	}
}
