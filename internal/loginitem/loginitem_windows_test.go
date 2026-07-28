//go:build windows

package loginitem

import "testing"

func TestCommandTargetsExecutableMatchesQuotedCurrentPath(t *testing.T) {
	executable := `C:\Program Files\DKST Text Flow\DKST Text Flow.exe`
	command := quoteCommandPath(executable)

	if !commandTargetsExecutable(command, executable) {
		t.Fatalf("commandTargetsExecutable(%q, %q) = false, want true", command, executable)
	}
}

func TestCommandTargetsExecutableRejectsStalePath(t *testing.T) {
	executable := `C:\Program Files\DKST Text Flow\DKST Text Flow.exe`
	command := quoteCommandPath(`C:\Old\DKST Text Flow.exe`)

	if commandTargetsExecutable(command, executable) {
		t.Fatalf("commandTargetsExecutable(%q, %q) = true, want false", command, executable)
	}
}
