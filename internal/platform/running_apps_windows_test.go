//go:build windows

package platform

import "testing"

func TestDecodeRunningWindowsProcessesAcceptsSingleObjectAndArray(t *testing.T) {
	for _, input := range []string{
		`{"Id":42,"ProcessName":"notepad","Path":"C:\\Windows\\notepad.exe","IconDataUrl":"data:image/png;base64,AA=="}`,
		`[{"Id":42,"ProcessName":"notepad","Path":"C:\\Windows\\notepad.exe","IconDataUrl":"data:image/png;base64,AA=="}]`,
	} {
		processes, err := decodeRunningWindowsProcesses([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(processes) != 1 ||
			processes[0].ID != 42 ||
			processes[0].IconDataURL != "data:image/png;base64,AA==" {
			t.Fatalf("unexpected processes: %#v", processes)
		}
	}
}
