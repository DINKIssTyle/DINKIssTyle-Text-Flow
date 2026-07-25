//go:build darwin

package speech

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var macVoiceLinePattern = regexp.MustCompile(`^(.+?)\s{2,}([[:alnum:]_-]+)\s+#`)

func ListNativeVoices() ([]Voice, error) {
	output, err := exec.Command("say", "-v", "?").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list macOS speech voices: %w", err)
	}

	voices := make([]Voice, 0)
	for _, line := range strings.Split(string(output), "\n") {
		match := macVoiceLinePattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		name := strings.TrimSpace(match[1])
		voices = append(voices, Voice{
			ID:       name,
			Name:     name,
			Language: match[2],
		})
	}
	return voices, nil
}

func StartNative(text string, voiceID string) (*exec.Cmd, error) {
	args := make([]string, 0, 3)
	if voiceID = strings.TrimSpace(voiceID); voiceID != "" {
		args = append(args, "-v", voiceID)
	}
	args = append(args, text)
	cmd := exec.Command("say", args...)
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
