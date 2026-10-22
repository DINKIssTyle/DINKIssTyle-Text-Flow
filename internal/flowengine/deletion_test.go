package flowengine

import (
	"strings"
	"testing"

	"dkst-text-flow/internal/flow"
	"dkst-text-flow/internal/storage"
)

func TestDeletionPreservesTextBeforePunctuationShortcut(t *testing.T) {
	for _, test := range []struct {
		name, buffer, shortcut, last, mode, want string
	}{
		{"instant punctuation", "prefix addr;", "addr;", ";", "instant", "addr;"},
		{"instant with trigger prefix", "prefix /addr;", "addr;", ";", "instant", "/addr;"},
		{"delimiter", "prefix addr ", "addr", " ", "delimiter", "addr "},
		{"delimiter after punctuation", "prefix addr; ", "addr;", " ", "delimiter", "addr; "},
	} {
		t.Run(test.name, func(t *testing.T) {
			matcher := flow.NewMatcher(96)
			matcher.Push(test.buffer)
			match, ok := matcher.Find([]storage.Snippet{{Shortcut: test.shortcut, Enabled: true, ExpandMode: test.mode}}, flow.IsDelimiter(test.last))
			if !ok {
				t.Fatal("expected a match")
			}
			keys := deleteKeysForMatch(test.buffer, test.last, match.Snippet.Shortcut, match.Snippet.CaseSensitive, match.ConsumedDelimiter)
			if match.ConsumedDelimiter && strings.HasSuffix(test.buffer, test.last) {
				keys += test.last
			}
			if keys != test.want {
				t.Fatalf("deleted keys = %q, want %q", keys, test.want)
			}
		})
	}
}
