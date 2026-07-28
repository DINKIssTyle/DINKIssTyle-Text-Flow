package platform

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
)

const maxScreenCaptureBytes = 20 * 1024 * 1024

type ScreenCaptureRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ScreenCaptureResult struct {
	DataURL  string `json:"dataUrl"`
	MimeType string `json:"mimeType"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Canceled bool   `json:"canceled"`
}

func validateScreenCaptureRect(rect ScreenCaptureRect) error {
	if rect.Width <= 1 || rect.Height <= 1 {
		return errors.New("screen capture region is empty")
	}
	return nil
}

func screenCaptureResultFromPNG(data []byte) (ScreenCaptureResult, error) {
	if len(data) == 0 {
		return ScreenCaptureResult{}, errors.New("screen capture returned an empty image")
	}
	if len(data) > maxScreenCaptureBytes {
		return ScreenCaptureResult{}, fmt.Errorf(
			"screen capture is too large (%d MB maximum)",
			maxScreenCaptureBytes/(1024*1024),
		)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ScreenCaptureResult{}, fmt.Errorf("failed to inspect screen capture: %w", err)
	}
	return ScreenCaptureResult{
		DataURL:  "data:image/png;base64," + base64.StdEncoding.EncodeToString(data),
		MimeType: "image/png",
		Width:    config.Width,
		Height:   config.Height,
	}, nil
}

func screenCaptureResultFromImage(captured image.Image) (ScreenCaptureResult, error) {
	if captured == nil {
		return ScreenCaptureResult{}, errors.New("screen capture returned no image")
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, captured); err != nil {
		return ScreenCaptureResult{}, fmt.Errorf("failed to encode screen capture: %w", err)
	}
	return screenCaptureResultFromPNG(encoded.Bytes())
}
