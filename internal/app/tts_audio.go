package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dkst-text-flow/internal/speech"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const noSynthesizedTTSAudioError = "no synthesized TTS audio is available"

type ttsAudioDialogStrings struct {
	title      string
	message    string
	buttonText string
}

func ttsAudioStrings(language string) ttsAudioDialogStrings {
	if language == "ko" {
		return ttsAudioDialogStrings{
			title:      "TTS 음성 저장",
			message:    "합성된 음성을 저장할 위치를 선택하세요.",
			buttonText: "음성 저장",
		}
	}
	return ttsAudioDialogStrings{
		title:      "Save TTS Audio",
		message:    "Choose where to save the synthesized speech.",
		buttonText: "Save Audio",
	}
}

func (a *App) setLastTTSAudio(data []byte) {
	a.ttsAudioMu.Lock()
	a.lastTTSAudio = append(a.lastTTSAudio[:0], data...)
	a.ttsAudioMu.Unlock()
}

func (a *App) clearLastTTSAudio() {
	a.ttsAudioMu.Lock()
	a.lastTTSAudio = nil
	a.ttsAudioMu.Unlock()
}

func (a *App) lastTTSAudioSnapshot() []byte {
	a.ttsAudioMu.RLock()
	data := append([]byte(nil), a.lastTTSAudio...)
	a.ttsAudioMu.RUnlock()
	return data
}

// ReplayLastTTSAudio plays the most recently completed local Supertonic WAV.
func (a *App) ReplayLastTTSAudio() error {
	data := a.lastTTSAudioSnapshot()
	if len(data) == 0 {
		return errors.New(noSynthesizedTTSAudioError)
	}

	ctx, speechID := a.beginSpeaking()
	playbackStarted := false
	defer func() {
		if !playbackStarted {
			a.finishSpeaking(speechID, nil)
		}
	}()

	path, err := writeTemporaryTTSAudio(data)
	if err != nil {
		return err
	}
	if err := a.startTemporaryTTSAudioPlayback(ctx, speechID, path); err != nil {
		return err
	}
	playbackStarted = true
	return nil
}

// SaveLastTTSAudio saves the most recently completed local Supertonic WAV.
// It returns false when the save dialog is cancelled.
func (a *App) SaveLastTTSAudio(language string) (bool, error) {
	data := a.lastTTSAudioSnapshot()
	if len(data) == 0 {
		return false, errors.New(noSynthesizedTTSAudioError)
	}

	text := ttsAudioStrings(language)
	defaultFilename := fmt.Sprintf(
		"DKST-Text-Flow-TTS-%s.wav",
		time.Now().Format("2006-01-02-150405"),
	)
	saveDialog := application.Get().Dialog.SaveFile()
	saveDialog.SetOptions(&application.SaveFileDialogOptions{
		CanCreateDirectories: true,
		AllowOtherFileTypes:  false,
		Title:                text.title,
		Message:              text.message,
		Filename:             defaultFilename,
		ButtonText:           text.buttonText,
		Filters: []application.FileFilter{{
			DisplayName: "WAV audio (*.wav)",
			Pattern:     "*.wav",
		}},
	})
	path, err := saveDialog.PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return false, nil
	}
	path = wavFilePath(path)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return false, fmt.Errorf("failed to save synthesized speech: %w", err)
	}
	return true, nil
}

func wavFilePath(path string) string {
	extension := filepath.Ext(path)
	if strings.EqualFold(extension, ".wav") {
		return strings.TrimSuffix(path, extension) + ".wav"
	}
	return path + ".wav"
}

func writeTemporaryTTSAudio(data []byte) (string, error) {
	tempFile, err := os.CreateTemp("", "dkst-tts-*.wav")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary TTS audio: %w", err)
	}
	path := tempFile.Name()
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("failed to write temporary TTS audio: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("failed to close temporary TTS audio: %w", err)
	}
	return path, nil
}

func (a *App) startTemporaryTTSAudioPlayback(
	ctx context.Context,
	speechID uint64,
	path string,
) error {
	if err := ctx.Err(); err != nil {
		_ = os.Remove(path)
		return err
	}
	cmd, err := speech.StartAudioPlayback(path)
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	if !a.attachSpeakingCommand(speechID, cmd) {
		stopSpeakingCommand(cmd)
		_ = os.Remove(path)
		return context.Canceled
	}
	go func() {
		_ = cmd.Wait()
		_ = os.Remove(path)
		a.finishSpeaking(speechID, cmd)
	}()
	return nil
}
