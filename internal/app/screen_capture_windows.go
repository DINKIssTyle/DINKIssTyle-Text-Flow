//go:build windows

package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"time"

	"dkst-text-flow/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (a *App) beginPlatformScreenRegionCapture(_ context.Context) error {
	appInst := application.Get()
	screens := appInst.Screen.GetAll()
	if len(screens) == 0 {
		return errors.New("no displays are available for screen capture")
	}

	var captureWindows []application.Window
	application.InvokeSync(func() {
		for index, screen := range screens {
			query := url.Values{}
			query.Set("mode", "capture")
			query.Set("screenId", screen.ID)
			captureWindow := appInst.Window.NewWithOptions(application.WebviewWindowOptions{
				Name:           fmt.Sprintf("screen-capture-%d", index),
				Title:          "Select screenshot region",
				Width:          screen.Bounds.Width,
				Height:         screen.Bounds.Height,
				AlwaysOnTop:    true,
				Hidden:         true,
				URL:            "/?" + query.Encode(),
				Frameless:      true,
				DisableResize:  true,
				BackgroundType: application.BackgroundTypeTransparent,
				Windows: application.WindowsWindow{
					DisableFramelessWindowDecorations: true,
					HiddenOnTaskbar:                   true,
				},
			})
			captureWindow.SetBounds(screen.Bounds)
			captureWindow.SetAlwaysOnTop(true)
			captureWindow.Show()
			captureWindows = append(captureWindows, captureWindow)
		}
		if len(captureWindows) > 0 {
			captureWindows[0].Focus()
		}
	})

	a.screenCaptureMu.Lock()
	if a.screenCaptureActive {
		a.screenCaptureWindows = captureWindows
		a.screenCaptureMu.Unlock()
		return nil
	}
	a.screenCaptureMu.Unlock()
	for _, captureWindow := range captureWindows {
		captureWindow.Close()
	}
	return nil
}

func (a *App) completePlatformScreenRegionCapture(selection ScreenRegionSelection) error {
	if selection.ViewportWidth <= 0 || selection.ViewportHeight <= 0 ||
		selection.Width <= 1 || selection.Height <= 1 {
		return errors.New("screen capture region is empty")
	}

	a.screenCaptureMu.Lock()
	if !a.screenCaptureActive {
		a.screenCaptureMu.Unlock()
		return errors.New("screen capture is not active")
	}
	if a.screenCaptureCompleting {
		a.screenCaptureMu.Unlock()
		return nil
	}
	a.screenCaptureCompleting = true
	ctx := a.screenCaptureContext
	a.screenCaptureMu.Unlock()

	screen := application.Get().Screen.GetByID(selection.ScreenID)
	if screen == nil {
		a.screenCaptureMu.Lock()
		a.screenCaptureCompleting = false
		a.screenCaptureMu.Unlock()
		return errors.New("the selected display is no longer available")
	}

	scaleX := float64(screen.PhysicalBounds.Width) / selection.ViewportWidth
	scaleY := float64(screen.PhysicalBounds.Height) / selection.ViewportHeight
	left := screen.PhysicalBounds.X + int(math.Round(selection.X*scaleX))
	top := screen.PhysicalBounds.Y + int(math.Round(selection.Y*scaleY))
	right := screen.PhysicalBounds.X + int(math.Round((selection.X+selection.Width)*scaleX))
	bottom := screen.PhysicalBounds.Y + int(math.Round((selection.Y+selection.Height)*scaleY))
	left = max(left, screen.PhysicalBounds.X)
	top = max(top, screen.PhysicalBounds.Y)
	right = min(right, screen.PhysicalBounds.X+screen.PhysicalBounds.Width)
	bottom = min(bottom, screen.PhysicalBounds.Y+screen.PhysicalBounds.Height)
	rect := platform.ScreenCaptureRect{
		X:      left,
		Y:      top,
		Width:  right - left,
		Height: bottom - top,
	}

	a.hideScreenCaptureWindows()
	go func() {
		time.Sleep(120 * time.Millisecond)
		result, err := platform.CaptureScreenRegion(ctx, rect)
		a.finishScreenRegionCapture(result, err)
	}()
	return nil
}
