//go:build windows

package speech

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

func StartNative(text string) (*exec.Cmd, error) {
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-Command",
		"$encoded = [Console]::In.ReadToEnd(); $text = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded)); $voice = New-Object -ComObject SAPI.SpVoice; [void]$voice.Speak($text)",
	)
	cmd.Stdin = strings.NewReader(base64.StdEncoding.EncodeToString([]byte(text)))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Windows speech command: %w", err)
	}
	return cmd, nil
}

func StartAudioPlayback(path string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("local Supertonic audio playback is not supported on Windows yet")
}
