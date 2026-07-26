//go:build windows

package flowengine

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"dkst-text-flow/internal/flow"
	"dkst-text-flow/internal/platform"
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
	hook          uintptr
	threadID      uint32
}

type kbdllhookstruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct {
		X int32
		Y int32
	}
}

type keyboardInput struct {
	text      string
	backspace bool
	reset     bool
}

const (
	whKeyboardLL         = 13
	wmKeyDown            = 0x0100
	wmSysKeyDown         = 0x0104
	wmQuit               = 0x0012
	keyeventfKeyup       = 0x0002
	vkBack               = 0x08
	vkTab                = 0x09
	vkReturn             = 0x0D
	vkShift              = 0x10
	vkControl            = 0x11
	vkMenu               = 0x12
	vkEscape             = 0x1B
	vkSpace              = 0x20
	vkEnd                = 0x23
	vkHome               = 0x24
	vkLeft               = 0x25
	vkUp                 = 0x26
	vkRight              = 0x27
	vkDown               = 0x28
	vkPageUp             = 0x21
	vkPageDown           = 0x22
	vkDelete             = 0x2E
	vkV                  = 0x56
	syntheticInputMarker = 0x444B5354
)

var (
	keyboardEngine          = &engine{matcher: flow.NewMatcher(96)}
	keyboardEvents          = make(chan keyboardInput, 256)
	keyboardWorkerOnce      sync.Once
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procGetMessage          = user32.NewProc("GetMessageW")
	procPostThreadMessage   = user32.NewProc("PostThreadMessageW")
	procGetKeyboardState    = user32.NewProc("GetKeyboardState")
	procToUnicode           = user32.NewProc("ToUnicode")
	procGetKeyState         = user32.NewProc("GetKeyState")
	procKeybdEvent          = user32.NewProc("keybd_event")
	procGetCurrentThreadID  = kernel32.NewProc("GetCurrentThreadId")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	keyboardProcCallback    = syscall.NewCallback(keyboardProc)
	snippetTagPattern       = regexp.MustCompile(`\{\{([^{}]+)\}\}`)
)

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

	keyboardWorkerOnce.Do(func() {
		go processKeyboardEvents()
	})

	started := make(chan bool, 1)
	go runKeyboardHook(started)
	return <-started
}

func Stop() {
	keyboardEngine.mu.Lock()
	hook := keyboardEngine.hook
	threadID := keyboardEngine.threadID
	keyboardEngine.mu.Unlock()

	if hook != 0 {
		procUnhookWindowsHookEx.Call(hook)
	}
	if threadID != 0 {
		procPostThreadMessage.Call(uintptr(threadID), wmQuit, 0, 0)
	}
	flushTypingCount()

	keyboardEngine.mu.Lock()
	keyboardEngine.running = false
	keyboardEngine.generation++
	keyboardEngine.hook = 0
	keyboardEngine.threadID = 0
	keyboardEngine.matcher.Reset()
	keyboardEngine.mu.Unlock()
}

func Running() bool {
	keyboardEngine.mu.Lock()
	defer keyboardEngine.mu.Unlock()
	return keyboardEngine.running
}

func runKeyboardHook(started chan<- bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	threadID, _, _ := procGetCurrentThreadID.Call()
	moduleHandle, _, _ := procGetModuleHandle.Call(0)
	hook, _, hookErr := procSetWindowsHookEx.Call(whKeyboardLL, keyboardProcCallback, moduleHandle, 0)
	if hook == 0 {
		fmt.Printf("failed to start Windows keyboard hook: %v\n", hookErr)
		started <- false
		return
	}

	keyboardEngine.mu.Lock()
	keyboardEngine.hook = hook
	keyboardEngine.threadID = uint32(threadID)
	keyboardEngine.running = true
	keyboardEngine.generation++
	keyboardEngine.mu.Unlock()
	started <- true

	var message msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
	}

	keyboardEngine.mu.Lock()
	if keyboardEngine.running {
		keyboardEngine.generation++
	}
	keyboardEngine.running = false
	keyboardEngine.hook = 0
	keyboardEngine.threadID = 0
	keyboardEngine.mu.Unlock()
}

func keyboardProc(nCode int, wParam uintptr, lParam uintptr) uintptr {
	if nCode >= 0 && (wParam == wmKeyDown || wParam == wmSysKeyDown) {
		event := (*kbdllhookstruct)(unsafe.Pointer(lParam))
		if event.DwExtraInfo != syntheticInputMarker {
			enqueueKeyEvent(event)
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

func enqueueKeyEvent(event *kbdllhookstruct) {
	input := keyboardInput{}
	switch event.VkCode {
	case vkBack:
		input.backspace = true
	case vkEscape, vkHome, vkEnd, vkLeft, vkUp, vkRight, vkDown, vkPageUp, vkPageDown, vkDelete:
		input.reset = true
	default:
		input.text = keyText(event)
		if input.text == "" {
			return
		}
	}

	select {
	case keyboardEvents <- input:
	default:
		keyboardEngine.mu.Lock()
		keyboardEngine.matcher.Reset()
		keyboardEngine.mu.Unlock()
	}
}

func processKeyboardEvents() {
	for input := range keyboardEvents {
		handleKeyboardInput(input)
	}
}

func handleKeyboardInput(input keyboardInput) {
	keyboardEngine.mu.Lock()
	suppressed := keyboardEngine.suppressInput
	store := keyboardEngine.store
	matcher := keyboardEngine.matcher
	running := keyboardEngine.running
	keyboardEngine.mu.Unlock()
	if suppressed || store == nil || !running {
		return
	}

	if input.reset {
		keyboardEngine.mu.Lock()
		matcher.Reset()
		keyboardEngine.mu.Unlock()
		return
	}

	if input.backspace {
		recordTyping(store, 1)
		keyboardEngine.mu.Lock()
		matcher.Backspace()
		keyboardEngine.mu.Unlock()
		return
	}

	text := input.text
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

	keyboardEngine.mu.Lock()
	generation := keyboardEngine.generation
	onExpansion := keyboardEngine.onExpansion
	keyboardEngine.mu.Unlock()
	if !executionActive(generation) {
		return
	}
	if onExpansion != nil {
		onExpansion(match.Snippet)
	}

	deleteKeys := deleteKeysForMatch(buffer, text, match.Snippet.Shortcut, match.Snippet.CaseSensitive, delimiterTyped)
	deleteCount := shortcutDeleteCount(deleteKeys)
	if delimiterTyped && strings.HasSuffix(buffer, text) {
		deleteCount += len([]rune(text))
	}

	keyboardEngine.mu.Lock()
	keyboardEngine.suppressInput = true
	keyboardEngine.mu.Unlock()
	go func(snippet storage.Snippet, count int, generation uint64) {
		defer func() {
			keyboardEngine.mu.Lock()
			keyboardEngine.suppressInput = false
			keyboardEngine.mu.Unlock()
		}()
		if !executionActive(generation) {
			return
		}
		postBackspaces(count)
		time.Sleep(backspaceSettleDuration(count))
		if !executeSnippetActions(renderSnippetActions(snippet.Content), snippet.UsePaste, generation) {
			return
		}
		_ = store.LogExpansion(snippet.ID, "")
	}(match.Snippet, deleteCount, generation)
}

func keyText(event *kbdllhookstruct) string {
	if keyDown(vkControl) || keyDown(vkMenu) {
		return ""
	}
	switch event.VkCode {
	case vkSpace:
		return " "
	case vkTab:
		return "\t"
	case vkReturn:
		return "\n"
	}

	var state [256]byte
	ok, _, _ := procGetKeyboardState.Call(uintptr(unsafe.Pointer(&state[0])))
	if ok == 0 {
		return ""
	}

	var buffer [8]uint16
	result, _, _ := procToUnicode.Call(
		uintptr(event.VkCode),
		uintptr(event.ScanCode),
		uintptr(unsafe.Pointer(&state[0])),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		0,
	)
	if int32(result) <= 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer[:result])
}

func keyDown(vk uintptr) bool {
	state, _, _ := procGetKeyState.Call(vk)
	return state&0x8000 != 0
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
	return len([]rune(deleteKeys))
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
	keyCode uintptr
}

func executeSnippetActions(actions []snippetAction, usePaste bool, generation uint64) bool {
	for _, action := range actions {
		if !executionActive(generation) {
			return false
		}
		if action.text != "" {
			pasteText(action.text)
			time.Sleep(24 * time.Millisecond)
			continue
		}
		if action.keyCode > 0 {
			postKey(action.keyCode)
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

func renderSnippetTag(tag string) (bool, string, uintptr) {
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

func snippetKeyCode(tag string) (uintptr, bool) {
	switch strings.ReplaceAll(tag, " ", "") {
	case "tab":
		return vkTab, true
	case "return", "enter":
		return vkReturn, true
	case "esc", "escape":
		return vkEscape, true
	case "home":
		return vkHome, true
	case "end":
		return vkEnd, true
	case "pageup", "pgup":
		return vkPageUp, true
	case "pagedown", "pgdn":
		return vkPageDown, true
	case "up", "arrowup":
		return vkUp, true
	case "down", "arrowdown":
		return vkDown, true
	case "left", "arrowleft":
		return vkLeft, true
	case "right", "arrowright":
		return vkRight, true
	default:
		return 0, false
	}
}

func postBackspaces(count int) {
	for i := 0; i < count; i++ {
		postKey(vkBack)
		time.Sleep(8 * time.Millisecond)
	}
}

func postKey(vk uintptr) {
	procKeybdEvent.Call(vk, 0, 0, syntheticInputMarker)
	procKeybdEvent.Call(vk, 0, keyeventfKeyup, syntheticInputMarker)
}

func pasteText(text string) {
	previous := clipboardText()
	if err := setClipboardText(text); err != nil {
		return
	}
	procKeybdEvent.Call(vkControl, 0, 0, syntheticInputMarker)
	postKey(vkV)
	procKeybdEvent.Call(vkControl, 0, keyeventfKeyup, syntheticInputMarker)
	time.Sleep(200 * time.Millisecond)
	_ = setClipboardText(previous)
}

func clipboardText() string {
	text, err := platform.ReadClipboardText()
	if err != nil {
		return ""
	}
	return text
}

func setClipboardText(text string) error {
	return platform.WriteClipboardText(text)
}
