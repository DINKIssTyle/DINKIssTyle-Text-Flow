//go:build darwin

package flowengine

/*
#cgo darwin CFLAGS: -x objective-c -fblocks
#cgo darwin LDFLAGS: -framework ApplicationServices -framework Cocoa -framework Carbon

int DKSTStartKeyboardTap(void);
void DKSTStopKeyboardTap(void);
void DKSTPostBackspaces(int count);
void DKSTPostKey(int keyCode);
int DKSTKoreanInputActive(void);
#include <stdlib.h>
void DKSTPasteText(const char *text);
void DKSTTypeText(const char *text);
char *DKSTClipboardText(void);
*/
import "C"

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unsafe"

	"dkst-text-flow/internal/flow"
	"dkst-text-flow/internal/storage"
)

type Store interface {
	ListSnippets(query string) ([]storage.Snippet, error)
	LogExpansion(snippetID int64, appBundleID string) error
	LogTyping(count int64) error
}

type engine struct {
	mu            sync.Mutex
	store         Store
	onExpansion   func(storage.Snippet)
	matcher       *flow.Matcher
	running       bool
	suppressInput bool
	typingCount   int64
	typingFlush   bool
	generation    uint64
}

var keyboardEngine = &engine{matcher: flow.NewMatcher(96)}

func SetExpansionHandler(handler func(storage.Snippet)) {
	keyboardEngine.mu.Lock()
	keyboardEngine.onExpansion = handler
	keyboardEngine.mu.Unlock()
}

func Start(store Store) bool {
	keyboardEngine.mu.Lock()
	keyboardEngine.store = store
	keyboardEngine.matcher.Reset()
	if keyboardEngine.running {
		keyboardEngine.mu.Unlock()
		return true
	}
	keyboardEngine.mu.Unlock()

	started := C.DKSTStartKeyboardTap() == 1
	keyboardEngine.mu.Lock()
	keyboardEngine.running = started
	if started {
		keyboardEngine.generation++
	}
	keyboardEngine.mu.Unlock()
	return started
}

func Stop() {
	C.DKSTStopKeyboardTap()
	flushTypingCount()
	keyboardEngine.mu.Lock()
	keyboardEngine.running = false
	keyboardEngine.generation++
	keyboardEngine.matcher.Reset()
	keyboardEngine.suppressInput = false
	keyboardEngine.mu.Unlock()
}

func Running() bool {
	keyboardEngine.mu.Lock()
	defer keyboardEngine.mu.Unlock()
	return keyboardEngine.running
}

//export DKSTKeyboardInput
func DKSTKeyboardInput(value *C.char, backspace C.int) {
	keyboardEngine.mu.Lock()
	if keyboardEngine.suppressInput || !keyboardEngine.running {
		keyboardEngine.mu.Unlock()
		return
	}
	store := keyboardEngine.store
	matcher := keyboardEngine.matcher
	generation := keyboardEngine.generation
	keyboardEngine.mu.Unlock()
	if store == nil {
		return
	}

	if backspace == 1 {
		recordTyping(store, 1)
		keyboardEngine.mu.Lock()
		matcher.Backspace()
		keyboardEngine.mu.Unlock()
		return
	}

	text := C.GoString(value)
	if text == "" {
		return
	}
	recordTyping(store, 1)

	delimiterTyped := flow.IsDelimiter(text)
	keyboardEngine.mu.Lock()
	matcher.Push(text)
	buffer := matcher.Buffer()
	keyboardEngine.mu.Unlock()

	snippets, err := store.ListSnippets("")
	if err != nil {
		return
	}

	keyboardEngine.mu.Lock()
	match, ok := matcher.Find(snippets, delimiterTyped)
	if ok {
		matcher.Reset()
	}
	keyboardEngine.mu.Unlock()
	if !ok {
		return
	}

	if !executionActive(generation) {
		return
	}
	deleteKeys := deleteKeysForMatch(buffer, text, match.Snippet.Shortcut, match.Snippet.CaseSensitive, delimiterTyped)
	deleteCount := shortcutDeleteCount(deleteKeys)
	if delimiterTyped && strings.HasSuffix(buffer, text) {
		deleteCount += len([]rune(text))
	}

	if !executionActive(generation) {
		return
	}
	C.DKSTPostBackspaces(C.int(deleteCount))
	time.Sleep(backspaceSettleDuration(deleteCount))
	if !executeSnippetActions(renderSnippetActions(match.Snippet.Content), match.Snippet.UsePaste, generation) {
		return
	}
	keyboardEngine.mu.Lock()
	onExpansion := keyboardEngine.onExpansion
	keyboardEngine.mu.Unlock()
	if onExpansion != nil {
		onExpansion(match.Snippet)
	}
	_ = store.LogExpansion(match.Snippet.ID, "")
}

func recordTyping(store Store, count int64) {
	if store == nil || count <= 0 {
		return
	}

	var shouldFlush bool
	keyboardEngine.mu.Lock()
	keyboardEngine.typingCount += count
	if keyboardEngine.typingCount >= 25 {
		shouldFlush = true
	} else if !keyboardEngine.typingFlush {
		keyboardEngine.typingFlush = true
		time.AfterFunc(1200*time.Millisecond, flushTypingCount)
	}
	keyboardEngine.mu.Unlock()

	if shouldFlush {
		go flushTypingCount()
	}
}

func flushTypingCount() {
	keyboardEngine.mu.Lock()
	store := keyboardEngine.store
	count := keyboardEngine.typingCount
	keyboardEngine.typingCount = 0
	keyboardEngine.typingFlush = false
	keyboardEngine.mu.Unlock()

	if store == nil || count <= 0 {
		return
	}
	_ = store.LogTyping(count)
}

func deleteKeysForMatch(buffer string, delimiter string, shortcut string, caseSensitive bool, delimiterTyped bool) string {
	matchBuffer := buffer
	if delimiterTyped && strings.HasSuffix(matchBuffer, delimiter) {
		matchBuffer = trimLastRune(matchBuffer)
	}

	foldedBuffer := matchBuffer
	foldedShortcut := shortcut
	if !caseSensitive {
		foldedBuffer = strings.ToLower(foldedBuffer)
		foldedShortcut = strings.ToLower(foldedShortcut)
	}
	if !strings.HasSuffix(foldedBuffer, foldedShortcut) {
		return shortcut
	}

	bufferRunes := []rune(matchBuffer)
	shortcutRunes := []rune(shortcut)
	if len(shortcutRunes) == 0 || len(bufferRunes) < len(shortcutRunes) {
		return shortcut
	}

	start := len(bufferRunes) - len(shortcutRunes)
	deleteRunes := bufferRunes[start:]
	if start > 0 && isTriggerPrefix(bufferRunes[start-1]) && !isTriggerPrefix(shortcutRunes[0]) {
		deleteRunes = append([]rune{bufferRunes[start-1]}, deleteRunes...)
	}
	return string(deleteRunes)
}

func shortcutDeleteCount(deleteKeys string) int {
	return shortcutDeleteCountForInputMode(deleteKeys, C.DKSTKoreanInputActive() == 1)
}

func shortcutDeleteCountForInputMode(deleteKeys string, koreanInputActive bool) int {
	typedLength := len([]rune(deleteKeys))
	if koreanInputActive {
		koreanLength := flow.KoreanTwoSetDisplayLength(deleteKeys)
		chordedLength := flow.KoreanTwoSetChordedDisplayLength(deleteKeys)
		if chordedLength > 0 && chordedLength < koreanLength {
			koreanLength = chordedLength
		}
		if koreanLength > 0 && koreanLength < typedLength {
			return koreanLength
		}
	}
	return typedLength
}

func backspaceSettleDuration(count int) time.Duration {
	if count <= 0 {
		return 20 * time.Millisecond
	}
	return time.Duration(count)*12*time.Millisecond + 48*time.Millisecond
}

func isTriggerPrefix(value rune) bool {
	switch value {
	case '`', ';', '/', '\\':
		return true
	default:
		return false
	}
}

func trimLastRune(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	return string(runes[:len(runes)-1])
}

type snippetAction struct {
	text    string
	keyCode int
}

var snippetTagPattern = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

func executeSnippetActions(actions []snippetAction, usePaste bool, generation uint64) bool {
	if !executionActive(generation) {
		return false
	}
	keyboardEngine.mu.Lock()
	keyboardEngine.suppressInput = true
	keyboardEngine.mu.Unlock()
	defer func() {
		keyboardEngine.mu.Lock()
		keyboardEngine.suppressInput = false
		keyboardEngine.mu.Unlock()
	}()

	for _, action := range actions {
		if !executionActive(generation) {
			return false
		}
		if action.text != "" {
			text := C.CString(action.text)
			if usePaste {
				C.DKSTPasteText(text)
				time.Sleep(24 * time.Millisecond)
			} else {
				C.DKSTTypeText(text)
				time.Sleep(16 * time.Millisecond)
			}
			C.free(unsafe.Pointer(text))
			continue
		}
		if action.keyCode > 0 {
			C.DKSTPostKey(C.int(action.keyCode))
			time.Sleep(12 * time.Millisecond)
		}
	}
	return executionActive(generation)
}

func executionActive(generation uint64) bool {
	keyboardEngine.mu.Lock()
	defer keyboardEngine.mu.Unlock()
	return keyboardEngine.running && keyboardEngine.generation == generation
}

func renderSnippetActions(content string) []snippetAction {
	matches := snippetTagPattern.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return []snippetAction{{text: content}}
	}

	actions := []snippetAction{}
	cursor := 0
	for _, match := range matches {
		if match[0] > cursor {
			appendSnippetText(&actions, content[cursor:match[0]])
		}
		tag := strings.TrimSpace(content[match[2]:match[3]])
		if handled, actionText, keyCode := renderSnippetTag(tag); handled {
			if actionText != "" {
				appendSnippetText(&actions, actionText)
			}
			if keyCode > 0 {
				actions = append(actions, snippetAction{keyCode: keyCode})
			}
		} else {
			appendSnippetText(&actions, content[match[0]:match[1]])
		}
		cursor = match[1]
	}
	if cursor < len(content) {
		appendSnippetText(&actions, content[cursor:])
	}
	if len(actions) == 0 {
		return []snippetAction{{text: ""}}
	}
	return actions
}

func appendSnippetText(actions *[]snippetAction, value string) {
	if value == "" {
		return
	}
	last := len(*actions) - 1
	if last >= 0 && (*actions)[last].keyCode == 0 {
		(*actions)[last].text += value
		return
	}
	*actions = append(*actions, snippetAction{text: value})
}

func renderSnippetTag(tag string) (bool, string, int) {
	normalized := strings.ToLower(strings.TrimSpace(tag))
	if strings.HasPrefix(normalized, "date:") {
		return true, time.Now().Format(tag[len("date:"):]), 0
	}
	if strings.HasPrefix(normalized, "time:") {
		return true, time.Now().Format(tag[len("time:"):]), 0
	}
	switch normalized {
	case "clipboard", "paste":
		return true, clipboardText(), 0
	case "space", "spacebar":
		return true, " ", 0
	}
	if keyCode, ok := snippetKeyCode(normalized); ok {
		return true, "", keyCode
	}
	return false, "", 0
}

func clipboardText() string {
	value := C.DKSTClipboardText()
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}

func snippetKeyCode(tag string) (int, bool) {
	switch strings.ReplaceAll(tag, " ", "") {
	case "tab":
		return 48, true
	case "return", "enter":
		return 36, true
	case "esc", "escape":
		return 53, true
	case "home":
		return 115, true
	case "end":
		return 119, true
	case "pageup", "pgup":
		return 116, true
	case "pagedown", "pgdn":
		return 121, true
	case "up", "arrowup":
		return 126, true
	case "down", "arrowdown":
		return 125, true
	case "left", "arrowleft":
		return 123, true
	case "right", "arrowright":
		return 124, true
	default:
		return 0, false
	}
}
