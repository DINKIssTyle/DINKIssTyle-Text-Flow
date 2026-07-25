//go:build linux

package speech

import (
	"fmt"
	"os/exec"
	"strings"
)

func ListNativeVoices() ([]Voice, error) {
	for _, binary := range []string{"espeak-ng", "espeak"} {
		if _, err := exec.LookPath(binary); err != nil {
			continue
		}
		output, err := exec.Command(binary, "--voices").Output()
		if err != nil {
			return nil, fmt.Errorf("failed to list Linux speech voices: %w", err)
		}
		voices := make([]Voice, 0)
		for index, line := range strings.Split(string(output), "\n") {
			if index == 0 {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			voices = append(voices, Voice{
				ID:       fields[3],
				Name:     strings.ReplaceAll(fields[3], "_", " "),
				Language: fields[1],
				Gender:   strings.TrimPrefix(fields[2], "--/"),
			})
		}
		return voices, nil
	}
	if _, err := exec.LookPath("spd-say"); err == nil {
		return []Voice{{ID: "", Name: "System default"}}, nil
	}
	return nil, fmt.Errorf("Linux OS TTS requires spd-say, espeak-ng, or espeak")
}

func StartNative(text string, voiceID string) (*exec.Cmd, error) {
	voiceID = strings.TrimSpace(voiceID)
	candidates := []struct {
		binary string
		args   []string
	}{
		{binary: "spd-say", args: []string{"--wait", text}},
		{binary: "espeak-ng", args: []string{text}},
		{binary: "espeak", args: []string{text}},
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate.binary); err != nil {
			continue
		}
		args := candidate.args
		if voiceID != "" && candidate.binary != "spd-say" {
			args = append([]string{"-v", voiceID}, args...)
		}
		cmd := exec.Command(candidate.binary, args...)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start Linux OS TTS: %w", err)
		}
		return cmd, nil
	}
	return nil, fmt.Errorf("Linux OS TTS requires spd-say, espeak-ng, or espeak")
}

func StartAudioPlayback(path string) (*exec.Cmd, error) {
	for _, candidate := range [][]string{
		{"paplay", path},
		{"aplay", path},
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", path},
	} {
		if _, err := exec.LookPath(candidate[0]); err != nil {
			continue
		}
		cmd := exec.Command(candidate[0], candidate[1:]...)
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start Linux audio playback: %w", err)
		}
		return cmd, nil
	}
	return nil, fmt.Errorf("Linux audio playback requires paplay, aplay, or ffplay")
}
