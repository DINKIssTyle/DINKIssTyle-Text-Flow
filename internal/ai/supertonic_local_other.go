//go:build !darwin && !windows && !linux

package ai

import (
	"context"
	"errors"
)

// TTSModelStatus holds status for downloading models.
type TTSModelStatus struct {
	IsDownloaded bool    `json:"isDownloaded"`
	Status       string  `json:"status"`
	Progress     float64 `json:"progress"`
	CurrentFile  string  `json:"currentFile"`
	Error        string  `json:"error,omitempty"`
}

type Style struct{}

func (s *Style) Destroy() {}

type SupertonicEngine struct {
	SampleRate int
}

func (tts *SupertonicEngine) Destroy() {}

func (tts *SupertonicEngine) Synthesize(text string, lang string, style *Style, totalStep int, speed float32) ([]float32, error) {
	return nil, errors.New("local Supertonic TTS is not supported on this operating system yet")
}

func (tts *SupertonicEngine) SynthesizeContext(ctx context.Context, text string, lang string, style *Style, totalStep int, speed float32) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return tts.Synthesize(text, lang, style, totalStep, speed)
}

func LoadVoiceStyle(path string) (*Style, error) {
	return nil, errors.New("local Supertonic TTS is not supported on this operating system yet")
}

func LoadSupertonicEngine(supertonicDir string) (*SupertonicEngine, error) {
	return nil, errors.New("local Supertonic TTS is not supported on this operating system yet")
}

func CheckModelStatus(supertonicDir string) TTSModelStatus {
	return TTSModelStatus{
		IsDownloaded: false,
		Status:       "unsupported",
		Error:        "local Supertonic TTS is not supported on this operating system yet",
	}
}

func StartTTSModelDownload(supertonicDir string, onProgress func(TTSModelStatus)) error {
	return errors.New("local Supertonic TTS is not supported on this operating system yet")
}

func CancelTTSModelDownload() {}

func WriteWav(filename string, audioData []float32, sampleRate int) error {
	return errors.New("local Supertonic TTS is not supported on this operating system yet")
}
