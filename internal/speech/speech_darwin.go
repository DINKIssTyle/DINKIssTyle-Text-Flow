//go:build darwin

package speech

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

var macVoiceLinePattern = regexp.MustCompile(`^(.+?)\s{2,}([[:alnum:]_-]+)\s+#`)

const macVoiceListScript = `
ObjC.import("AVFoundation");

function run() {
    return JSON.stringify($.AVSpeechSynthesisVoice.speechVoices.js.map(function (voice) {
        var gender = Number(voice.gender);
        var quality = Number(voice.quality);
        return {
            id: ObjC.unwrap(voice.identifier),
            name: ObjC.unwrap(voice.name),
            language: ObjC.unwrap(voice.language),
            gender: gender === 1 ? "male" : (gender === 2 ? "female" : ""),
            quality: quality === 3 ? "premium" : (quality === 2 ? "enhanced" : "default")
        };
    }));
}
`

func ListNativeVoices() ([]Voice, error) {
	output, err := exec.Command(
		"osascript",
		"-l", "JavaScript",
		"-e", macVoiceListScript,
	).Output()
	if err == nil {
		var voices []Voice
		if decodeErr := json.Unmarshal(output, &voices); decodeErr == nil {
			sort.SliceStable(voices, func(i, j int) bool {
				if voices[i].Language != voices[j].Language {
					return voices[i].Language < voices[j].Language
				}
				if voices[i].Name != voices[j].Name {
					return voices[i].Name < voices[j].Name
				}
				return voices[i].Quality > voices[j].Quality
			})
			return voices, nil
		}
	}

	return listSayVoices()
}

func listSayVoices() ([]Voice, error) {
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
			Language: strings.ReplaceAll(match[2], "_", "-"),
		})
	}
	return voices, nil
}

func StartNative(text string, voiceID string) (*exec.Cmd, error) {
	voiceID = strings.TrimSpace(voiceID)
	args := make([]string, 0, 3)
	if voiceID != "" {
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
