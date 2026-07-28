//go:build windows

package platform

import (
	"context"
	"image"

	"github.com/kbinani/screenshot"
)

func CaptureScreenRegion(ctx context.Context, rect ScreenCaptureRect) (ScreenCaptureResult, error) {
	if err := validateScreenCaptureRect(rect); err != nil {
		return ScreenCaptureResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return ScreenCaptureResult{Canceled: true}, nil
	}
	captured, err := screenshot.CaptureRect(image.Rect(
		rect.X,
		rect.Y,
		rect.X+rect.Width,
		rect.Y+rect.Height,
	))
	if err != nil {
		return ScreenCaptureResult{}, err
	}
	return screenCaptureResultFromImage(captured)
}
