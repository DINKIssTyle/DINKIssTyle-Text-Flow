//go:build !darwin && !windows

package platform

import (
	"context"
	"errors"
)

func CaptureScreenRegion(_ context.Context, _ ScreenCaptureRect) (ScreenCaptureResult, error) {
	return ScreenCaptureResult{}, errors.New("screen region capture is not supported on this platform")
}
