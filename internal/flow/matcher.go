package flow

import (
	"strings"
	"unicode/utf8"

	"dkst-text-flow/internal/storage"
)

type Match struct {
	Snippet storage.Snippet
}

type Matcher struct {
	buffer string
	limit  int
}

func NewMatcher(limit int) *Matcher {
	if limit <= 0 {
		limit = 64
	}
	return &Matcher{limit: limit}
}

func (m *Matcher) Push(text string) {
	m.buffer += text
	if len(m.buffer) > m.limit {
		m.buffer = m.buffer[len(m.buffer)-m.limit:]
	}
}

func (m *Matcher) Backspace() {
	if m.buffer == "" {
		return
	}
	m.buffer = m.buffer[:len(m.buffer)-1]
}

func (m *Matcher) Reset() {
	m.buffer = ""
}

func (m *Matcher) Buffer() string {
	return m.buffer
}

func (m *Matcher) Find(snippets []storage.Snippet, delimiterTyped bool) (Match, bool) {
	for _, snippet := range snippets {
		if !snippet.Enabled {
			continue
		}
		if snippet.ExpandMode == "delimiter" && !delimiterTyped {
			continue
		}

		buffer := m.buffer
		if delimiterTyped {
			buffer = trimLastRune(buffer)
		}
		shortcut := snippet.Shortcut
		if !snippet.CaseSensitive {
			buffer = strings.ToLower(buffer)
			shortcut = strings.ToLower(shortcut)
		}
		if strings.HasSuffix(buffer, shortcut) {
			return Match{Snippet: snippet}, true
		}
	}
	return Match{}, false
}

func trimLastRune(value string) string {
	if value == "" {
		return value
	}
	_, size := utf8.DecodeLastRuneInString(value)
	if size <= 0 {
		return value
	}
	return value[:len(value)-size]
}

func IsDelimiter(text string) bool {
	switch text {
	case " ", "\n", "\t", ".", ",", "!", "?", ":", ";":
		return true
	default:
		return false
	}
}
