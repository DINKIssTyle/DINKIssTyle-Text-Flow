//go:build darwin

package flowengine

import (
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dkst-text-flow/internal/flow"
	"dkst-text-flow/internal/storage"
)

func TestNativeKeyboardTapLifecycleAndSyntheticInput(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "keyboard-tap-test")
	compile := exec.Command("clang", "-fblocks", "-framework", "ApplicationServices", "-framework", "Cocoa", "-framework", "Carbon", "testdata/keyboard_tap_darwin.m", "-o", executable)
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile native keyboard test: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native keyboard test: %v\n%s", err, output)
	}
}

type keyboardTestStore struct {
	list func() ([]storage.Snippet, error)
}

func (s *keyboardTestStore) ListSnippets(string) ([]storage.Snippet, error) { return s.list() }
func (*keyboardTestStore) LogTyping(int64) error                            { return nil }
func (*keyboardTestStore) LogExpansion(int64, string) error                 { return nil }

func newKeyboardTestEngine(capacity int) *engine {
	return &engine{matcher: flow.NewMatcher(96), events: make(chan keyboardInput, capacity), running: true, generation: 7}
}

func TestInputCallbackDoesNotWaitForStorage(t *testing.T) {
	e := newKeyboardTestEngine(4)
	lookupStarted := make(chan struct{})
	releaseLookup := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseLookup) }) })
	var startedOnce sync.Once
	e.store = &keyboardTestStore{list: func() ([]storage.Snippet, error) {
		startedOnce.Do(func() { close(lookupStarted) })
		<-releaseLookup
		return nil, nil
	}}
	workerDone := make(chan struct{})
	go func() {
		e.processKeyboardEvents()
		close(workerDone)
	}()
	e.enqueueKeyboardInput(keyboardInput{text: "a"})
	select {
	case <-lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("worker did not start storage lookup")
	}
	callbackDone := make(chan struct{})
	go func() {
		e.enqueueKeyboardInput(keyboardInput{text: "b"})
		e.enqueueKeyboardInput(keyboardInput{text: "c"})
		close(callbackDone)
	}()
	select {
	case <-callbackDone:
	case <-time.After(time.Second):
		t.Fatal("callback waited for a blocked storage lookup")
	}
	releaseOnce.Do(func() { close(releaseLookup) })
	close(e.events)
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("worker did not drain queued input")
	}
	if got := e.matcher.Buffer(); got != "abc" {
		t.Fatalf("input order = %q, want abc", got)
	}
}

func TestQueueOverflowInvalidatesQueuedInput(t *testing.T) {
	e := newKeyboardTestEngine(1)
	e.store = &keyboardTestStore{list: func() ([]storage.Snippet, error) {
		t.Fatal("stale input reached storage")
		return nil, nil
	}}
	e.matcher.Push("old")
	e.enqueueKeyboardInput(keyboardInput{text: "a"})
	e.enqueueKeyboardInput(keyboardInput{text: "dropped"})
	if e.generation != 8 || e.matcher.Buffer() != "" {
		t.Fatal("overflow did not invalidate the buffered sequence")
	}
	e.handleKeyboardInput(<-e.events)
	if got := e.matcher.Buffer(); got != "" {
		t.Fatalf("stale queued key changed the matcher: %q", got)
	}
	e.store = &keyboardTestStore{list: func() ([]storage.Snippet, error) { return nil, nil }}
	e.enqueueKeyboardInput(keyboardInput{text: "b"})
	e.handleKeyboardInput(<-e.events)
	if got := e.matcher.Buffer(); got != "b" {
		t.Fatalf("fresh input after overflow = %q, want b", got)
	}
}

func TestStoppedOrSuppressedEngineRejectsInput(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		e := newKeyboardTestEngine(1)
		e.running = !stopped
		e.suppressInput = !stopped
		e.enqueueKeyboardInput(keyboardInput{text: "a"})
		if len(e.events) != 0 {
			t.Fatal("inactive engine queued input")
		}
	}
}

func TestTapHealthInvalidatesPendingWork(t *testing.T) {
	e := newKeyboardTestEngine(1)
	e.matcher.Push("old")
	if !e.updateRunningState(true) || e.generation != 7 {
		t.Fatal("healthy tap changed engine state")
	}
	if e.updateRunningState(false) || e.generation != 8 || e.matcher.Buffer() != "" {
		t.Fatal("disabled tap did not cancel pending work")
	}
	if e.updateRunningState(false) || e.generation != 8 {
		t.Fatal("repeated health check invalidated work again")
	}
	if e.updateRunningState(true) {
		t.Fatal("health check restarted a stopped engine")
	}
	if e.executeSnippetActions([]snippetAction{{text: "must not be emitted"}}, false, 7) {
		t.Fatal("stale expansion executed")
	}
}
