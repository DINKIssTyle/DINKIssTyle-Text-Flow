//go:build windows

package speech

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"dkst-text-flow/internal/winprocess"
)

const windowsVoiceListScript = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$voices = @()
$synth = $null
try {
    Add-Type -AssemblyName System.Speech
    $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
    $voices = @(
        $synth.GetInstalledVoices() |
        Where-Object { $_.Enabled } |
        ForEach-Object {
            [PSCustomObject]@{
                id = $_.VoiceInfo.Id
                name = $_.VoiceInfo.Name
                language = $_.VoiceInfo.Culture.Name
                gender = $_.VoiceInfo.Gender.ToString()
            }
        }
    )
} catch {
    $tokenPaths = @(
        'HKCU:\SOFTWARE\Microsoft\Speech\Voices\Tokens',
        'HKLM:\SOFTWARE\Microsoft\Speech\Voices\Tokens'
    )
    foreach ($tokenPath in $tokenPaths) {
        if (-not (Test-Path -LiteralPath $tokenPath)) {
            continue
        }
        foreach ($tokenKey in Get-ChildItem -LiteralPath $tokenPath) {
            $token = Get-ItemProperty -LiteralPath $tokenKey.PSPath
            $attributes = Get-ItemProperty -LiteralPath ($tokenKey.PSPath + '\Attributes') -ErrorAction SilentlyContinue
            $language = ''
            if ($attributes.Language) {
                try {
                    $languageCode = ($attributes.Language -split ';')[0]
                    $language = [Globalization.CultureInfo]::GetCultureInfo([Convert]::ToInt32($languageCode, 16)).Name
                } catch {}
            }
            $voices += [PSCustomObject]@{
                id = $tokenKey.PSChildName
                name = $token.'(default)'
                language = $language
                gender = $attributes.Gender
            }
        }
    }
} finally {
    if ($null -ne $synth) {
        $synth.Dispose()
    }
}
ConvertTo-Json -InputObject @($voices | Sort-Object language, name -Unique) -Compress
`

const windowsSpeakScript = `
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Speech
$encoded = [Console]::In.ReadToEnd()
$json = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($encoded))
$payload = ConvertFrom-Json $json
$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
try {
    if ($payload.voiceId) {
        $selected = $synth.GetInstalledVoices() |
            Where-Object { $_.Enabled -and $_.VoiceInfo.Id -eq $payload.voiceId } |
            Select-Object -First 1
        if ($null -eq $selected) {
            throw "The selected Windows voice is no longer installed."
        }
        $synth.SelectVoice($selected.VoiceInfo.Name)
    }
    $synth.Speak([string]$payload.text)
} finally {
    $synth.Dispose()
}
`

func ListNativeVoices() ([]Voice, error) {
	cmd := winprocess.HideWindow(exec.Command(
		"powershell.exe",
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

	cmd := winprocess.HideWindow(exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		windowsSpeakScript,
	))
	cmd.Stdin = strings.NewReader(base64.StdEncoding.EncodeToString(payload))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Windows speech command: %w", err)
	}
	return cmd, nil
}

func StartAudioPlayback(path string) (*exec.Cmd, error) {
	cmd := winprocess.HideWindow(exec.Command(
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
