//go:build darwin

package app

import (
	"context"
	"errors"
	"math"
	"time"

	"dkst-text-flow/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (a *App) beginPlatformScreenRegionCapture(
	ctx context.Context,
	purpose screenCapturePurpose,
) error {
	if purpose == screenCapturePurposeFloating {
		return a.beginScreenRegionCaptureOverlay()
	}
	go func() {
		time.Sleep(140 * time.Millisecond)
		result, err := platform.CaptureScreenRegion(ctx, platform.ScreenCaptureRect{})
		a.finishScreenRegionCapture(result, err)
	}()
	return nil
}

func (a *App) completePlatformScreenRegionCapture(selection ScreenRegionSelection) error {
	a.screenCaptureMu.Lock()
	purpose := a.screenCapturePurpose
	a.screenCaptureMu.Unlock()
	if purpose != screenCapturePurposeFloating {
		return errors.New("macOS uses the native screen region selector")
	}

	ctx, placement, err := a.prepareOverlayScreenRegionCapture(selection)
	if err != nil {
		return err
	}
	if placement == nil {
		return nil
	}
	screen := application.Get().Screen.GetByID(placement.ScreenID)
	if screen == nil {
		return errors.New("the selected display is no longer available")
	}
	pixelWidth := placement.PixelWidth
	pixelHeight := placement.PixelHeight
	if pixelWidth <= 1 || pixelHeight <= 1 {
		pixelWidth = int(math.Round(float64(placement.LogicalBounds.Width) * float64(screen.ScaleFactor)))
		pixelHeight = int(math.Round(float64(placement.LogicalBounds.Height) * float64(screen.ScaleFactor)))
	}
	rect := platform.ScreenCaptureRect{
		DisplayID:   placement.ScreenID,
		X:           placement.LogicalBounds.X,
		Y:           placement.LogicalBounds.Y,
		Width:       placement.LogicalBounds.Width,
		Height:      placement.LogicalBounds.Height,
		PixelWidth:  pixelWidth,
		PixelHeight: pixelHeight,
	}
	a.hideScreenCaptureWindows()
	go func() {
		time.Sleep(120 * time.Millisecond)
		result, captureErr := platform.CaptureScreenRegion(ctx, rect)
		a.finishScreenRegionCapture(result, captureErr)
	}()
	return nil
}
