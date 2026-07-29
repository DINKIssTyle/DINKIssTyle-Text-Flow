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
	IconDataURL string `json:"IconDataUrl"`
}

func listRunningApps() []AppInfo {
	command := HideCommandWindow(exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		windowsRunningAppsPowerShell,
	))
	output, err := command.Output()
	if err != nil {
		return []AppInfo{}
	}

	processes, err := decodeRunningWindowsProcesses(output)
	if err != nil {
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
		if info.IconDataURL == "" {
			info.IconDataURL = strings.TrimSpace(process.IconDataURL)
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

func decodeRunningWindowsProcesses(output []byte) ([]runningWindowsProcess, error) {
	output = []byte(strings.TrimSpace(string(output)))
	if len(output) == 0 || string(output) == "null" {
		return []runningWindowsProcess{}, nil
	}
	if output[0] == '[' {
		var processes []runningWindowsProcess
		if err := json.Unmarshal(output, &processes); err != nil {
			return nil, err
		}
		return processes, nil
	}
	var process runningWindowsProcess
	if err := json.Unmarshal(output, &process); err != nil {
		return nil, err
	}
	return []runningWindowsProcess{process}, nil
}

const windowsRunningAppsPowerShell = `
$ErrorActionPreference = 'SilentlyContinue'
Add-Type -AssemblyName System.Drawing
@(
    Get-Process |
        Where-Object { $_.MainWindowHandle -ne 0 } |
        ForEach-Object {
            $processPath = $_.Path
            $iconDataUrl = ''
            if ($processPath) {
                try {
                    $icon = [System.Drawing.Icon]::ExtractAssociatedIcon($processPath)
                    if ($null -ne $icon) {
                        $bitmap = $icon.ToBitmap()
                        $stream = New-Object System.IO.MemoryStream
                        try {
                            $bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
                            $iconDataUrl = 'data:image/png;base64,' +
                                [Convert]::ToBase64String($stream.ToArray())
                        } finally {
                            $stream.Dispose()
                            $bitmap.Dispose()
                            $icon.Dispose()
                        }
                    }
                } catch {}
            }
            [PSCustomObject]@{
                Id = $_.Id
                ProcessName = $_.ProcessName
                Path = $processPath
                IconDataUrl = $iconDataUrl
            }
        }
) | ConvertTo-Json -Compress
`
