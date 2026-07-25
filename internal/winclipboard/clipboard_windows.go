//go:build windows

package winclipboard

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
)

const (
	readClipboardCommand  = `$text = Get-Clipboard -Raw; if ($null -eq $text) { $text = '' }; [Console]::Out.Write([Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$text)))`
	writeClipboardCommand = `$encoded = [Console]::In.ReadToEnd(); $text = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded)); Set-Clipboard -Value $text`
)

func ReadText() (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", readClipboardCommand)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read clipboard: %w %s", err, strings.TrimSpace(stderr.String()))
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(output)))
	if err != nil {
		return "", fmt.Errorf("decode clipboard text: %w", err)
	}
	return string(decoded), nil
}

func WriteText(text string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", writeClipboardCommand)
	cmd.Stdin = strings.NewReader(base64.StdEncoding.EncodeToString([]byte(text)))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write clipboard: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
