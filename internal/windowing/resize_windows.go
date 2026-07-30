//go:build windows

package windowing

import "github.com/wailsapp/wails/v3/pkg/application"

func ResizeFromTop(window application.Window, width, height int) {
	window.SetSize(width, height)
}

func ResizeFromBottom(window application.Window, width, height int) {
	x, y := window.Position()
	_, currentHeight := window.Size()
	window.SetPosition(x, y+currentHeight-height)
	window.SetSize(width, height)
}
