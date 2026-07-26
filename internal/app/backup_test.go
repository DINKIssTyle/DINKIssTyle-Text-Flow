package app

import "testing"

func TestLowercaseBackupPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "missing extension", path: "backup", want: "backup.dtf"},
		{name: "lowercase extension", path: "backup.dtf", want: "backup.dtf"},
		{name: "uppercase extension", path: "backup.DTF", want: "backup.dtf"},
		{name: "mixed-case extension", path: "backup.DtF", want: "backup.dtf"},
		{name: "different extension", path: "backup.json", want: "backup.json.dtf"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := lowercaseBackupPath(test.path); got != test.want {
				t.Fatalf("lowercaseBackupPath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}
