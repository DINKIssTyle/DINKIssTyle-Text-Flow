//go:build !darwin

package ocr

import "errors"

func RecognizePNG(_ []byte, _ string) (Result, error) {
	return Result{}, errors.New("Apple Vision OCR is available only on macOS")
}

func SupportedLanguages() ([]string, error) {
	return []string{}, nil
}
