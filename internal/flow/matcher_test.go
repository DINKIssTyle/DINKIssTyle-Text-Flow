package flow

import (
	"testing"

	"dkst-text-flow/internal/storage"
)

func TestInstantShortcutEndingInDelimiter(t *testing.T) {
	for _, shortcut := range []string{"addr", "addr;", "addr:", "addr!", "addr?", "addr.", "addr,"} {
		t.Run(shortcut, func(t *testing.T) {
			matcher := NewMatcher(96)
			snippets := []storage.Snippet{{Shortcut: shortcut, Enabled: true, ExpandMode: "instant"}}
			for index, value := range shortcut {
				text := string(value)
				matcher.Push(text)
				match, ok := matcher.Find(snippets, IsDelimiter(text))
				if want := index == len(shortcut)-1; ok != want {
					t.Fatalf("match at index %d = %v, want %v", index, ok, want)
				}
				if ok && match.ConsumedDelimiter {
					t.Fatal("instant match consumed a character belonging to the shortcut")
				}
			}
		})
	}
}

func TestDelimiterShortcutRequiresExtraDelimiter(t *testing.T) {
	for _, shortcut := range []string{"addr", "addr;", "addr!"} {
		for _, delimiter := range []string{" ", "\n", "\t", ";"} {
			t.Run(shortcut+"/"+delimiter, func(t *testing.T) {
				matcher := NewMatcher(96)
				snippets := []storage.Snippet{{Shortcut: shortcut, Enabled: true, ExpandMode: "delimiter"}}
				matcher.Push(shortcut)
				if _, ok := matcher.Find(snippets, IsDelimiter(string(shortcut[len(shortcut)-1]))); ok {
					t.Fatal("matched before the extra delimiter")
				}
				matcher.Push(delimiter)
				match, ok := matcher.Find(snippets, true)
				if !ok || !match.ConsumedDelimiter {
					t.Fatalf("delimiter match = %+v, %v", match, ok)
				}
			})
		}
	}
}

func TestMatcherHonorsEnabledAndCaseSensitive(t *testing.T) {
	for _, test := range []struct {
		name          string
		enabled       bool
		caseSensitive bool
		want          bool
	}{
		{"disabled", false, false, false},
		{"case sensitive", true, true, false},
		{"case insensitive", true, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			matcher := NewMatcher(96)
			matcher.Push("ADDR;")
			_, ok := matcher.Find([]storage.Snippet{{Shortcut: "addr;", Enabled: test.enabled, CaseSensitive: test.caseSensitive, ExpandMode: "instant"}}, true)
			if ok != test.want {
				t.Fatalf("matched = %v, want %v", ok, test.want)
			}
		})
	}
}
