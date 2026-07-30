//go:build !darwin && !windows

package app

import (
	"context"
	"errors"
)

func (a *App) beginPlatformScreenRegionCapture(
	_ context.Context,
	_ screenCapturePurpose,
) error {
	return errors.New("screen region capture is not supported on this platform")
}

func (a *App) completePlatformScreenRegionCapture(_ ScreenRegionSelection) error {
	return errors.New("screen region capture is not supported on this platform")
}
