//go:build windows

package speech

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"dkst-text-flow/internal/platform"
)

const windowsVoiceListScript = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
Add-Type -AssemblyName System.Runtime.WindowsRuntime
[Windows.Media.SpeechSynthesis.SpeechSynthesizer, Windows.Media.SpeechSynthesis, ContentType=WindowsRuntime] > $null
$voices = @(
    [Windows.Media.SpeechSynthesis.SpeechSynthesizer]::AllVoices |
    ForEach-Object {
        [PSCustomObject]@{
            id = $_.Id
            name = $_.DisplayName
            language = $_.Language
            gender = $_.Gender.ToString()
        }
    }
)
ConvertTo-Json -InputObject @($voices | Sort-Object language, name) -Compress
`

const windowsSpeakScript = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
Add-Type -AssemblyName System.Runtime.WindowsRuntime
[Windows.Media.SpeechSynthesis.SpeechSynthesizer, Windows.Media.SpeechSynthesis, ContentType=WindowsRuntime] > $null

function Await-WinRT($operation, [Type]$resultType) {
    $method = [System.WindowsRuntimeSystemExtensions].GetMethods() |
        Where-Object {
            $_.Name -eq 'AsTask' -and
            $_.IsGenericMethod -and
            $_.GetParameters().Count -eq 1
        } |
        Select-Object -First 1
    $task = $method.MakeGenericMethod($resultType).Invoke($null, @($operation))
    $task.Wait()
    return $task.Result
}

$encoded = [Console]::In.ReadToEnd()
$json = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded))
$payload = ConvertFrom-Json $json
$tempPath = [IO.Path]::Combine(
    [IO.Path]::GetTempPath(),
    'dkst-os-tts-' + [Guid]::NewGuid().ToString('N') + '.wav'
)
$synth = New-Object Windows.Media.SpeechSynthesis.SpeechSynthesizer
try {
    if ($payload.voiceId) {
        $selected = [Windows.Media.SpeechSynthesis.SpeechSynthesizer]::AllVoices |
            Where-Object { $_.Id -eq $payload.voiceId } |
            Select-Object -First 1
        if ($null -eq $selected) {
            throw "The selected Windows voice is no longer installed."
        }
        $synth.Voice = $selected
    }

    $speechStream = Await-WinRT (
        $synth.SynthesizeTextToStreamAsync([string]$payload.text)
    ) ([Windows.Media.SpeechSynthesis.SpeechSynthesisStream])
    try {
        $input = [System.IO.WindowsRuntimeStreamExtensions]::AsStreamForRead($speechStream)
        try {
            $output = [IO.File]::Create($tempPath)
            try {
                $input.CopyTo($output)
            } finally {
                $output.Dispose()
            }
        } finally {
            $input.Dispose()
        }
    } finally {
        $speechStream.Dispose()
    }

    $player = New-Object System.Media.SoundPlayer $tempPath
    $player.Load()
    [Console]::Out.WriteLine('READY')
    [Console]::Out.Flush()
    $player.PlaySync()
} finally {
    $synth.Dispose()
    Remove-Item -LiteralPath $tempPath -Force -ErrorAction SilentlyContinue
}
`

func ListNativeVoices() ([]Voice, error) {
	cmd := platform.HideCommandWindow(exec.Command(
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		windowsVoiceListScript,
	))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list Windows speech voices: %w", err)
	}

	var voices []Voice
	if err := json.Unmarshal(output, &voices); err != nil {
		return nil, fmt.Errorf("failed to decode Windows speech voices: %w", err)
	}
	return voices, nil
}

func StartNative(text string, voiceID string) (*exec.Cmd, error) {
	payload, err := json.Marshal(struct {
		Text    string `json:"text"`
		VoiceID string `json:"voiceId"`
	}{
		Text:    text,
		VoiceID: strings.TrimSpace(voiceID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare Windows speech request: %w", err)
	}

	cmd := platform.HideCommandWindow(exec.Command(
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		windowsSpeakScript,
	))
	cmd.Stdin = strings.NewReader(base64.StdEncoding.EncodeToString(payload))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to capture Windows speech readiness: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Windows speech command: %w", err)
	}

	reader := bufio.NewReader(stdout)
	ready, readErr := reader.ReadString('\n')
	if readErr != nil || strings.TrimSpace(ready) != "READY" {
		waitErr := cmd.Wait()
		detail := strings.TrimSpace(stderr.String())
		if detail == "" && waitErr != nil {
			detail = waitErr.Error()
		}
		if detail == "" {
			detail = "the Windows speech engine stopped before playback"
		}
		return nil, fmt.Errorf("failed to synthesize Windows OS speech: %s", detail)
	}
	go func() {
		_, _ = io.Copy(io.Discard, reader)
	}()
	return cmd, nil
}

func StartAudioPlayback(path string) (*exec.Cmd, error) {
	cmd := platform.HideCommandWindow(exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"$encoded = [Console]::In.ReadToEnd(); $path = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded)); $player = New-Object System.Media.SoundPlayer $path; $player.PlaySync()",
	))
	cmd.Stdin = strings.NewReader(base64.StdEncoding.EncodeToString([]byte(path)))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Windows audio playback: %w", err)
	}
	return cmd, nil
}
