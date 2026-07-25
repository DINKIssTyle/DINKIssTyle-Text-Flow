//go:build darwin

package speech

import (
	"fmt"
	"os/exec"
)

func StartNative(text string) (*exec.Cmd, error) {
	cmd := exec.Command("say", text)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start say command: %w", err)
	}
	return cmd, nil
}

func StartAudioPlayback(path string) (*exec.Cmd, error) {
	cmd := exec.Command("afplay", path)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start afplay command: %w", err)
	}
	return cmd, nil
}
