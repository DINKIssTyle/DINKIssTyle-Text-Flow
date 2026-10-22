package flowengine

import "testing"

func TestInvalidatePendingExpansionResetsMatcherAndGeneration(t *testing.T) {
	keyboardEngine.mu.Lock()
	originalGeneration := keyboardEngine.generation
	keyboardEngine.matcher.Reset()
	keyboardEngine.matcher.Push("stale shortcut")
	keyboardEngine.mu.Unlock()

	InvalidatePendingExpansion()

	keyboardEngine.mu.Lock()
	defer keyboardEngine.mu.Unlock()
	if got := keyboardEngine.matcher.Buffer(); got != "" {
		t.Fatalf("matcher buffer was not reset: %q", got)
	}
	if want := originalGeneration + 1; keyboardEngine.generation != want {
		t.Fatalf("generation = %d, want %d", keyboardEngine.generation, want)
	}
}
