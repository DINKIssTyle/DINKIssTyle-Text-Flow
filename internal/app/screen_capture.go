package app

import (
	"context"
	"errors"

	"dkst-text-flow/internal/platform"
	"dkst-text-flow/internal/windowing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type ScreenRegionSelection struct {
	ScreenID       string  `json:"screenId"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
	ViewportWidth  float64 `json:"viewportWidth"`
	ViewportHeight float64 `json:"viewportHeight"`
}

func (a *App) BeginScreenRegionCapture() error {
	a.screenCaptureMu.Lock()
	if a.screenCaptureActive {
		a.screenCaptureMu.Unlock()
		return errors.New("screen capture is already active")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.screenCaptureActive = true
	a.screenCaptureCompleting = false
	a.screenCaptureContext = ctx
	a.screenCaptureCancel = cancel
	a.screenCaptureMu.Unlock()

	appInst := application.Get()
	if aiWindow, ok := appInst.Window.GetByName("ai"); ok {
		application.InvokeSync(func() {
			aiWindow.Hide()
		})
	}
	if err := a.beginPlatformScreenRegionCapture(ctx); err != nil {
		a.finishScreenRegionCapture(platform.ScreenCaptureResult{}, err)
		return err
	}
	return nil
}

func (a *App) CompleteScreenRegionCapture(selection ScreenRegionSelection) error {
	return a.completePlatformScreenRegionCapture(selection)
}

func (a *App) CancelScreenRegionCapture() {
	a.cancelScreenRegionCapture(true)
}

func (a *App) cancelScreenRegionCapture(restoreHUD bool) {
	a.screenCaptureMu.Lock()
	if !a.screenCaptureActive {
		a.screenCaptureMu.Unlock()
		return
	}
	cancel := a.screenCaptureCancel
	if !restoreHUD {
		a.screenCaptureActive = false
		a.screenCaptureCompleting = false
		a.screenCaptureContext = nil
		a.screenCaptureCancel = nil
	}
	windows := append([]application.Window(nil), a.screenCaptureWindows...)
	if !restoreHUD {
		a.screenCaptureWindows = nil
	}
	a.screenCaptureMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if restoreHUD {
		a.finishScreenRegionCapture(platform.ScreenCaptureResult{Canceled: true}, nil)
		return
	}
	for _, captureWindow := range windows {
		captureWindow.Close()
	}
}

func (a *App) finishScreenRegionCapture(result platform.ScreenCaptureResult, captureErr error) {
	a.screenCaptureMu.Lock()
	if !a.screenCaptureActive {
		a.screenCaptureMu.Unlock()
		return
	}
	cancel := a.screenCaptureCancel
	a.screenCaptureActive = false
	a.screenCaptureCompleting = false
	a.screenCaptureContext = nil
	a.screenCaptureCancel = nil
	windows := a.screenCaptureWindows
	a.screenCaptureWindows = nil
	a.screenCaptureMu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, captureWindow := range windows {
		captureWindow.Close()
	}
	application.InvokeSync(func() {
		appInst := application.Get()
		if aiWindow, ok := appInst.Window.GetByName("ai"); ok {
			aiWindow.SetAlwaysOnTop(true)
			aiWindow.UnMinimise()
			aiWindow.Show()
			windowing.ActivateForInput(aiWindow)
			aiWindow.Focus()
		}
	})

	appInst := application.Get()
	switch {
	case captureErr != nil:
		appInst.Event.Emit("ai:screenshot-error", captureErr.Error())
	case result.Canceled:
		appInst.Event.Emit("ai:screenshot-canceled")
	default:
		appInst.Event.Emit("ai:screenshot-captured", result)
	}
}

func (a *App) hideScreenCaptureWindows() {
	a.screenCaptureMu.Lock()
	windows := append([]application.Window(nil), a.screenCaptureWindows...)
	a.screenCaptureMu.Unlock()
	application.InvokeSync(func() {
		for _, captureWindow := range windows {
			captureWindow.Hide()
		}
	})
}
