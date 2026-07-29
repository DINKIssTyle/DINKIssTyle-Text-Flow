package app

import (
	"path/filepath"
	"testing"
)

func TestNormalizeAIPromptAppPathByPlatform(t *testing.T) {
	tests := []struct {
		name string
		path string
		goos string
		want string
	}{
		{
			name: "mac app bundle remains supported",
			path: "/Applications/TextEdit.app/Contents/MacOS/TextEdit",
			goos: "darwin",
			want: filepath.FromSlash("/Applications/TextEdit.app"),
		},
		{
			name: "windows executable is accepted",
			path: `C:\Program Files\Notepad++\notepad++.exe`,
			goos: "windows",
			want: `C:\Program Files\Notepad++\notepad++.exe`,
		},
		{
			name: "windows non executable is rejected",
			path: `C:\Program Files\Notepad++\readme.txt`,
			goos: "windows",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeAIPromptAppPath(test.path, test.goos); got != test.want {
				t.Fatalf("normalizeAIPromptAppPath() = %q, want %q", got, test.want)
			}
		})
	}
}
