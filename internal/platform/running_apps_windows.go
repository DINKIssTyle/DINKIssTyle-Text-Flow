//go:build windows

package platform

import (
	"encoding/json"
	"sort"
	"strings"

	"os/exec"
)

type runningWindowsProcess struct {
	ID          int    `json:"Id"`
	ProcessName string `json:"ProcessName"`
	Path        string `json:"Path"`
}

func listRunningApps() []AppInfo {
	command := HideCommandWindow(exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		"@(Get-Process | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object Id, ProcessName, Path) | ConvertTo-Json -Compress",
	))
	output, err := command.Output()
	if err != nil {
		return []AppInfo{}
	}

	var processes []runningWindowsProcess
	if err := json.Unmarshal(output, &processes); err != nil {
		return []AppInfo{}
	}

	seen := make(map[string]struct{}, len(processes))
	apps := make([]AppInfo, 0, len(processes))
	for _, process := range processes {
		info := appInfoFromProcess(process.ID)
		if info.Name == "" {
			info.Name = strings.TrimSpace(process.ProcessName)
		}
		if info.Path == "" {
			info.Path = strings.TrimSpace(process.Path)
		}
		if info.BundleID == "" && info.Name != "" {
			info.BundleID = strings.ToLower(info.Name + ".exe")
		}
		key := strings.ToLower(info.BundleID)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		apps = append(apps, info)
	}
	sort.SliceStable(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps
}
