//go:build darwin

package app

import (
	"context"
	"errors"
	"time"

	"dkst-text-flow/internal/platform"
)

func (a *App) beginPlatformScreenRegionCapture(ctx context.Context) error {
	go func() {
		time.Sleep(140 * time.Millisecond)
		result, err := platform.CaptureScreenRegion(ctx, platform.ScreenCaptureRect{})
		a.finishScreenRegionCapture(result, err)
	}()
	return nil
}

func (a *App) completePlatformScreenRegionCapture(_ ScreenRegionSelection) error {
	return errors.New("macOS uses the native screen region selector")
}
