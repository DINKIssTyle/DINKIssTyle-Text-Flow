package app

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestFloatingCaptureWindowBoundsPreserveContentPosition(t *testing.T) {
	content := application.Rect{X: 100, Y: 200, Width: 320, Height: 180}
	padding := 16

	window := floatingCaptureWindowBounds(content, padding)
	if window.X != 84 || window.Y != 184 ||
		window.Width != 352 || window.Height != 212 {
		t.Fatalf("unexpected padded window bounds: %#v", window)
	}
	if window.X+padding != content.X || window.Y+padding != content.Y {
		t.Fatalf("content position moved: window=%#v content=%#v", window, content)
	}
}

func TestFloatingCaptureWindowBoundsLeaveMacStyleBoundsUnchanged(t *testing.T) {
	content := application.Rect{X: 100, Y: 200, Width: 320, Height: 180}
	if got := floatingCaptureWindowBounds(content, 0); got != content {
		t.Fatalf("zero padding changed bounds: got %#v, want %#v", got, content)
	}
}
