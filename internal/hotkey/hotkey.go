package hotkey

import (
	"errors"
	"strings"
)

type Shortcut struct {
	Key       string
	Command   bool
	Control   bool
	Option    bool
	Shift     bool
	Canonical string
}

func Parse(value string) (Shortcut, error) {
	parts := strings.Split(value, "+")
	shortcut := Shortcut{}

	for _, part := range parts {
		token := normalizeToken(part)
		switch token {
		case "":
			continue
		case "cmd", "command", "meta":
			shortcut.Command = true
		case "ctrl", "control":
			shortcut.Control = true
		case "opt", "option", "alt":
			shortcut.Option = true
		case "shift":
			shortcut.Shift = true
		default:
			if shortcut.Key != "" {
				return Shortcut{}, errors.New("hotkey can include only one non-modifier key")
			}
			shortcut.Key = canonicalKey(token)
		}
	}

	if shortcut.Key == "" {
		return Shortcut{}, errors.New("hotkey key is required")
	}
	if !shortcut.Command && !shortcut.Control && !shortcut.Option && !shortcut.Shift {
		return Shortcut{}, errors.New("hotkey requires at least one modifier")
	}

	shortcut.Canonical = formatCanonical(shortcut)
	return shortcut, nil
}

func normalizeToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "+")
	return strings.ToLower(value)
}

func canonicalKey(value string) string {
	switch value {
	case " ":
		return "Space"
	case "esc", "escape":
		return "Esc"
	case "return":
		return "Enter"
	case "up", "arrowup":
		return "Up"
	case "down", "arrowdown":
		return "Down"
	case "left", "arrowleft":
		return "Left"
	case "right", "arrowright":
		return "Right"
	default:
		if len(value) == 1 {
			return strings.ToUpper(value)
		}
		return strings.ToUpper(value[:1]) + value[1:]
	}
}

func formatCanonical(shortcut Shortcut) string {
	parts := make([]string, 0, 5)
	if shortcut.Command {
		parts = append(parts, "Cmd")
	}
	if shortcut.Control {
		parts = append(parts, "Ctrl")
	}
	if shortcut.Option {
		parts = append(parts, "Option")
	}
	if shortcut.Shift {
		parts = append(parts, "Shift")
	}
	parts = append(parts, shortcut.Key)
	return strings.Join(parts, "+")
}
