//go:build !darwin

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func resizeWindowFromTop(window application.Window, width, height int) {
	window.SetSize(width, height)
}
