//go:build windows

package loginitem

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "DKST Text Flow"
)

func Enabled() bool {
	executable, err := currentExecutable()
	if err != nil {
		return false
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	command, _, err := key.GetStringValue(runValueName)
	if err != nil {
		return false
	}
	return commandTargetsExecutable(command, executable)
}

func SetEnabled(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enabled {
		err := key.DeleteValue(runValueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}

	executable, err := currentExecutable()
	if err != nil {
		return err
	}
	return key.SetStringValue(runValueName, quoteCommandPath(executable))
}

func currentExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(executable)
}

func quoteCommandPath(path string) string {
	return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
}

func commandTargetsExecutable(command string, executable string) bool {
	target := firstCommandPath(strings.TrimSpace(command))
	if target == "" {
		return false
	}

	targetAbs, err := filepath.Abs(target)
	if err == nil {
		target = targetAbs
	}
	executableAbs, err := filepath.Abs(executable)
	if err == nil {
		executable = executableAbs
	}
	return strings.EqualFold(filepath.Clean(target), filepath.Clean(executable))
}

func firstCommandPath(command string) string {
	if command == "" {
		return ""
	}
	if command[0] != '"' {
		for index, r := range command {
			if r == ' ' || r == '\t' {
				return command[:index]
			}
		}
		return command
	}

	end := strings.IndexRune(command[1:], '"')
	if end < 0 {
		return ""
	}
	return command[1 : end+1]
}
