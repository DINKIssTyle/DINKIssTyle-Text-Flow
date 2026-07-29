package app

import (
	"path/filepath"
	"testing"

	"dkst-text-flow/internal/ai"
	"dkst-text-flow/internal/storage"
)

func TestCustomPromptForRequestPrefersMatchingAppProfileAndFallsBackToCommon(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "ai-prompts.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := &App{store: store}
	if _, err := app.saveAIPromptSettings(AIPromptSettings{
		Common: AIPromptRule{
			UseSelectedText:     true,
			RunWithoutSelection: true,
			SelectedTextPrompt:  "common selected rule",
			NoSelectionPrompt:   "common no-selection rule",
		},
		Profiles: []AIPromptProfile{
			{
				ID:          "terminal",
				AppName:     "Terminal",
				AppBundleID: "com.apple.Terminal",
				AIPromptRule: AIPromptRule{
					UseSelectedText:     true,
					RunWithoutSelection: true,
					SelectedTextPrompt:  "terminal selected rule",
					NoSelectionPrompt:   "terminal no-selection rule",
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		request ai.AssistRequest
		want    string
	}{
		{
			name: "matching app with selected text",
			request: ai.AssistRequest{
				AppBundleID: "com.apple.Terminal",
				ContextKind: ai.ContextSelectedText,
				ContextText: "selected",
			},
			want: "terminal selected rule",
		},
		{
			name: "matching app without selected text",
			request: ai.AssistRequest{
				AppBundleID: "com.apple.Terminal",
				ContextKind: ai.ContextNone,
			},
			want: "terminal no-selection rule",
		},
		{
			name: "unmatched app with selected text",
			request: ai.AssistRequest{
				AppBundleID: "com.example.Other",
				ContextKind: ai.ContextSelectedText,
				ContextText: "selected",
			},
			want: "common selected rule",
		},
		{
			name: "unmatched app without selected text",
			request: ai.AssistRequest{
				AppBundleID: "com.example.Other",
				ContextKind: ai.ContextNone,
			},
			want: "common no-selection rule",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := app.customPromptForRequest(test.request); got != test.want {
				t.Fatalf("customPromptForRequest() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCustomPromptForRequestHonorsContextEnableFlags(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "ai-prompt-flags.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	app := &App{store: store}
	if _, err := app.saveAIPromptSettings(AIPromptSettings{
		Common: AIPromptRule{
			SelectedTextPrompt: "disabled selected rule",
			NoSelectionPrompt:  "disabled no-selection rule",
		},
	}); err != nil {
		t.Fatal(err)
	}

	selected := ai.AssistRequest{
		ContextKind: ai.ContextSelectedText,
		ContextText: "selected",
	}
	if got := app.customPromptForRequest(selected); got != "" {
		t.Fatalf("disabled selected-text rule was applied: %q", got)
	}
	if got := app.customPromptForRequest(ai.AssistRequest{ContextKind: ai.ContextNone}); got != "" {
		t.Fatalf("disabled no-selection rule was applied: %q", got)
	}
}
