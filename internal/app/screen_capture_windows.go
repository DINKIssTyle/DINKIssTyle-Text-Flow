//go:build windows

package app

import (
	"context"
	"time"

	"dkst-text-flow/internal/platform"
)

func (a *App) beginPlatformScreenRegionCapture(
	_ context.Context,
	_ screenCapturePurpose,
) error {
	return a.beginScreenRegionCaptureOverlay()
}

func (a *App) completePlatformScreenRegionCapture(selection ScreenRegionSelection) error {
	ctx, placement, err := a.prepareOverlayScreenRegionCapture(selection)
	if err != nil {
		return err
	}
	if placement == nil {
		return nil
	}
	rect := platform.ScreenCaptureRect{
		X:      placement.PhysicalBounds.X,
		Y:      placement.PhysicalBounds.Y,
		Width:  placement.PhysicalBounds.Width,
		Height: placement.PhysicalBounds.Height,
	}

	a.hideScreenCaptureWindows()
	go func() {
		time.Sleep(120 * time.Millisecond)
		result, err := platform.CaptureScreenRegion(ctx, rect)
		a.finishScreenRegionCapture(result, err)
	}()
	return nil
}
