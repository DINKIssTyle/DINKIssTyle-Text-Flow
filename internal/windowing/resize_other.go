//go:build !darwin

package windowing

import "github.com/wailsapp/wails/v3/pkg/application"

func ResizeFromTop(window application.Window, width, height int) {
	window.SetSize(width, height)
}
