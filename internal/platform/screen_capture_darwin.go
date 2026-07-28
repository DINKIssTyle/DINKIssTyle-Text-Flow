//go:build darwin

package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func CaptureScreenRegion(ctx context.Context, _ ScreenCaptureRect) (ScreenCaptureResult, error) {
	tempDir, err := os.MkdirTemp("", "dkst-text-flow-screen-capture-")
	if err != nil {
		return ScreenCaptureResult{}, fmt.Errorf("failed to prepare screen capture: %w", err)
	}
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "capture.png")
	command := exec.CommandContext(
		ctx,
		"/usr/sbin/screencapture",
		"-i",
		"-x",
		"-t",
		"png",
		outputPath,
	)
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return ScreenCaptureResult{Canceled: true}, nil
		}
		if _, statErr := os.Stat(outputPath); errors.Is(statErr, os.ErrNotExist) {
			return ScreenCaptureResult{Canceled: true}, nil
		}
		return ScreenCaptureResult{}, fmt.Errorf("screen capture failed: %w", err)
	}

	data, err := os.ReadFile(outputPath)
	if errors.Is(err, os.ErrNotExist) || len(data) == 0 {
		return ScreenCaptureResult{Canceled: true}, nil
	}
	if err != nil {
		return ScreenCaptureResult{}, fmt.Errorf("failed to read screen capture: %w", err)
	}
	return screenCaptureResultFromPNG(data)
}
