//go:build !darwin && !windows && !linux

package speech

import (
	"fmt"
	"os/exec"
)

func ListNativeVoices() ([]Voice, error) {
	return nil, fmt.Errorf("OS TTS is not supported on this operating system yet")
}

func StartNative(text string, voiceID string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("OS TTS is not supported on this operating system yet")
}

func StartAudioPlayback(path string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("local Supertonic audio playback is not supported on this operating system yet")
}
