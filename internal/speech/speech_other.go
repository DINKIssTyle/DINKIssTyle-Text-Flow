//go:build !darwin && !windows

package speech

import (
	"fmt"
	"os/exec"
)

func StartNative(text string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("native TTS is not supported on this operating system yet")
}

func StartAudioPlayback(path string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("local Supertonic audio playback is not supported on this operating system yet")
}
