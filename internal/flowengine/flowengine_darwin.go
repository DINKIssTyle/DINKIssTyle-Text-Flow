//go:build darwin

package flowengine

/*
#cgo darwin CFLAGS: -x objective-c -fblocks
#cgo darwin LDFLAGS: -framework ApplicationServices -framework Cocoa -framework Carbon

int DKSTStartKeyboardTap(void);
int DKSTKeyboardTapRunning(void);
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
	events        chan keyboardInput
	workerOnce    sync.Once
}

type keyboardInput struct {
	text       string
	backspace  bool
	generation uint64
}

var keyboardEngine = &engine{
	matcher: flow.NewMatcher(96),
	events:  make(chan keyboardInput, 256),
}

func SetExpansionHandler(handler func(storage.Snippet)) {
	keyboardEngine.mu.Lock()
	keyboardEngine.onExpansion = handler
	keyboardEngine.mu.Unlock()
}

func Start(store Store) bool {
	keyboardEngine.mu.Lock()
	keyboardEngine.store = store
	keyboardEngine.mu.Unlock()
	keyboardEngine.workerOnce.Do(func() {
		go keyboardEngine.processKeyboardEvents()
	})

	started := C.DKSTStartKeyboardTap() == 1
	keyboardEngine.mu.Lock()
	if keyboardEngine.running != started {
		keyboardEngine.generation++
		keyboardEngine.matcher.Reset()
	}
	keyboardEngine.running = started
	keyboardEngine.mu.Unlock()
	return started
}

func Stop() {
	// Cancel queued and in-flight work before tearing down the native tap.
	keyboardEngine.mu.Lock()
	keyboardEngine.running = false
	keyboardEngine.generation++
	keyboardEngine.matcher.Reset()
	keyboardEngine.mu.Unlock()
	C.DKSTStopKeyboardTap()
	keyboardEngine.flushTypingCount()
}

func Running() bool {
	return keyboardEngine.updateRunningState(C.DKSTKeyboardTapRunning() == 1)
}

func (e *engine) updateRunningState(tapRunning bool) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running && !tapRunning {
		e.running = false
		e.generation++
		e.matcher.Reset()
	}
	return e.running
}

// InvalidatePendingExpansion prevents a match captured before a snippet
// mutation from being executed after that mutation completes.
func InvalidatePendingExpansion() {
	keyboardEngine.mu.Lock()
	keyboardEngine.generation++
	keyboardEngine.matcher.Reset()
	keyboardEngine.mu.Unlock()
}

//export DKSTKeyboardInput
func DKSTKeyboardInput(value *C.char, backspace C.int) {
	keyboardEngine.enqueueKeyboardInput(keyboardInput{
		text: C.GoString(value), backspace: backspace == 1,
	})
}

// The event-tap callback must never wait for storage, clipboard access or output.
func (e *engine) enqueueKeyboardInput(input keyboardInput) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.suppressInput || !e.running {
		return
	}
	input.generation = e.generation
	select {
	case e.events <- input:
	default:
		// A missing key makes both the buffer and all queued matches unreliable.
		e.generation++
		e.matcher.Reset()
	}
}

func (e *engine) processKeyboardEvents() {
	for input := range e.events {
		e.handleKeyboardInput(input)
	}
}

func (e *engine) handleKeyboardInput(input keyboardInput) {
	e.mu.Lock()
	if e.suppressInput || !e.running || input.generation != e.generation {
		e.mu.Unlock()
		return
	}
	store := e.store
	if store == nil {
		e.mu.Unlock()
		return
	}
	if input.backspace {
		e.matcher.Backspace()
		e.mu.Unlock()
		e.recordTyping(store, 1)
		return
	}
	text := input.text
	if text == "" {
		e.mu.Unlock()
		return
	}
	e.matcher.Push(text)
	buffer := e.matcher.Buffer()
	e.mu.Unlock()
	e.recordTyping(store, 1)

	snippets, err := store.ListSnippets("")
	if err != nil {
		return
	}

	e.mu.Lock()
	// Stop, restart, deletion and queue overflow can invalidate a pending lookup.
	if !e.running || input.generation != e.generation {
		e.mu.Unlock()
		return
	}
	match, ok := e.matcher.Find(snippets, flow.IsDelimiter(text))
	if ok {
		e.matcher.Reset()
		e.suppressInput = true
	}
	e.mu.Unlock()
	if !ok {
		return
	}
	defer func() {
		e.mu.Lock()
		e.suppressInput = false
		e.mu.Unlock()
	}()

	deleteKeys := deleteKeysForMatch(buffer, text, match.Snippet.Shortcut, match.Snippet.CaseSensitive, match.ConsumedDelimiter)
	deleteCount := shortcutDeleteCount(deleteKeys)
	if match.ConsumedDelimiter && strings.HasSuffix(buffer, text) {
		deleteCount += len([]rune(text))
	}
	if !e.executionActive(input.generation) {
		return
	}
	C.DKSTPostBackspaces(C.int(deleteCount))
	time.Sleep(backspaceSettleDuration(deleteCount))
	if !e.executeSnippetActions(renderSnippetActions(match.Snippet.Content), match.Snippet.UsePaste, input.generation) {
		return
	}
	e.mu.Lock()
	// Usage logging must not extend the interval in which real input is ignored.
	e.suppressInput = false
	onExpansion := e.onExpansion
	e.mu.Unlock()
	if onExpansion != nil {
		onExpansion(match.Snippet)
	}
	_ = store.LogExpansion(match.Snippet.ID, "")
}

func (e *engine) recordTyping(store Store, count int64) {
	if store == nil || count <= 0 {
		return
	}

	var shouldFlush bool
	e.mu.Lock()
	e.typingCount += count
	if e.typingCount >= 25 {
		shouldFlush = true
	} else if !e.typingFlush {
		e.typingFlush = true
		time.AfterFunc(1200*time.Millisecond, e.flushTypingCount)
	}
	e.mu.Unlock()

	if shouldFlush {
		go e.flushTypingCount()
	}
}

func (e *engine) flushTypingCount() {
	e.mu.Lock()
	store := e.store
	count := e.typingCount
	e.typingCount = 0
	e.typingFlush = false
	e.mu.Unlock()

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

func (e *engine) executeSnippetActions(actions []snippetAction, usePaste bool, generation uint64) bool {
	for _, action := range actions {
		if !e.executionActive(generation) {
			return false
		}
		if action.text != "" {
			if usePaste {
				text := C.CString(action.text)
				C.DKSTPasteText(text)
				C.free(unsafe.Pointer(text))
				time.Sleep(24 * time.Millisecond)
			} else {
				// Bound native calls so pausing also cancels a long text action.
				runes := []rune(action.text)
				for start := 0; start < len(runes); start += 32 {
					if !e.executionActive(generation) {
						return false
					}
					end := min(start+32, len(runes))
					text := C.CString(string(runes[start:end]))
					C.DKSTTypeText(text)
					C.free(unsafe.Pointer(text))
				}
				time.Sleep(16 * time.Millisecond)
			}
			continue
		}
		if action.keyCode > 0 {
			C.DKSTPostKey(C.int(action.keyCode))
			time.Sleep(12 * time.Millisecond)
		}
	}
	return e.executionActive(generation)
}

func (e *engine) executionActive(generation uint64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running && e.generation == generation
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
