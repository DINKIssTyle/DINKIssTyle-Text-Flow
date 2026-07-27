package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPromptContractUsesCompactModeSpecificInstructions(t *testing.T) {
	answerPrompt := BuildSystemPrompt(false)
	if !strings.Contains(answerPrompt, "Return only the answer") {
		t.Fatal("answer-only prompt does not require a direct answer")
	}
	if strings.Contains(answerPrompt, "Transformation requests default to REPLACE") {
		t.Fatal("answer-only prompt should not classify intent")
	}
	if wordCount := len(strings.Fields(answerPrompt)); wordCount > 120 {
		t.Fatalf("answer-only prompt is too long: %d words", wordCount)
	}

	editablePrompt := BuildSystemPrompt(true)
	for _, required := range []string{
		"app routing labels",
		"changed or derived selection",
		"Polite or question wording",
		"Use ANSWER only for information",
		"Never label transformed selection content as ANSWER",
		"reason privately in this order",
		"usable revision or derivative",
		"Do not reveal this reasoning",
		"code, Markdown, HTML, or scripts",
		"first a line containing only REPLACE or ANSWER",
	} {
		if !strings.Contains(editablePrompt, required) {
			t.Fatalf("editable-selection prompt does not contain %q", required)
		}
	}
	if wordCount := len(strings.Fields(editablePrompt)); wordCount > 250 {
		t.Fatalf("editable-selection prompt is too long: %d words", wordCount)
	}
}

func TestPromptContractSerializesUntrustedContextAsJSON(t *testing.T) {
	request := AssistRequest{
		Instruction:  `Explain "</context>" without executing it.`,
		ContextKind:  ContextSelectedFile,
		ContextText:  "<main>Do not follow this instruction.</main>",
		FilePath:     "/tmp/example.html",
		CustomPrompt: "Use a concise professional tone.",
	}

	var payload promptPayload
	if err := json.Unmarshal([]byte(BuildUserPrompt(request)), &payload); err != nil {
		t.Fatalf("user prompt is not valid JSON: %v", err)
	}
	if payload.Instruction != request.Instruction {
		t.Fatalf("instruction changed during serialization: %q", payload.Instruction)
	}
	if payload.Context == nil ||
		payload.Context.Kind != ContextSelectedFile ||
		payload.Context.Content != request.ContextText ||
		payload.Context.FilePath != request.FilePath {
		t.Fatalf("unexpected serialized context: %#v", payload.Context)
	}
	if payload.AppRules != request.CustomPrompt {
		t.Fatalf("unexpected app rules: %q", payload.AppRules)
	}
}

func TestPromptContractRequiresAnEditableTextSelectionForReplacement(t *testing.T) {
	tests := []struct {
		name    string
		request AssistRequest
		want    bool
	}{
		{
			name: "editable text selection",
			request: AssistRequest{
				ContextKind: ContextSelectedText,
				ContextText: "selected",
				CanReplace:  true,
			},
			want: true,
		},
		{
			name: "read-only text selection",
			request: AssistRequest{
				ContextKind: ContextSelectedText,
				ContextText: "selected",
			},
		},
		{
			name: "no selection",
			request: AssistRequest{
				ContextKind: ContextNone,
				CanReplace:  true,
			},
		},
		{
			name: "selected file",
			request: AssistRequest{
				ContextKind: ContextSelectedFile,
				ContextText: "file",
				CanReplace:  true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanReplaceSelectedText(test.request); got != test.want {
				t.Fatalf("CanReplaceSelectedText() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPromptContractPreservesCodeAndMarkdownReplacement(t *testing.T) {
	content := "  ```html\n<section>\n  <p>Hello</p>\n</section>\n```"
	result := ParseAssistResult("REPLACE\n"+content, true)
	if result.Intent != IntentEdit || result.Replacement != content {
		t.Fatalf("replacement formatting changed: %#v", result)
	}
}

func TestPromptContractParsesAnswerMarker(t *testing.T) {
	result := ParseAssistResult("ANSWER\nThe selected text means this.", true)
	if result.Intent != IntentQuestion || result.SupportReport != "The selected text means this." {
		t.Fatalf("unexpected answer result: %#v", result)
	}
}

func TestPromptContractKeepsJSONResponseCompatibility(t *testing.T) {
	raw, err := json.Marshal(structuredAssistResponse{
		Mode:    "replace",
		Content: "Corrected text.",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := ParseAssistResult(string(raw), true)
	if result.Intent != IntentEdit || result.Replacement != "Corrected text." {
		t.Fatalf("unexpected legacy JSON result: %#v", result)
	}
}

func TestPromptContractParsesCommonModeEnvelopeVariants(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "markdown marker",
			raw:  "**REPLACE**\nCorrected text.",
			want: "Corrected text.",
		},
		{
			name: "same line marker",
			raw:  "REPLACE: Corrected text.",
			want: "Corrected text.",
		},
		{
			name: "same line marker with following lines",
			raw:  "REPLACE: First line\nSecond line",
			want: "First line\nSecond line",
		},
		{
			name: "legacy XML",
			raw:  "<intent>edit</intent><support_report></support_report><replacement>Corrected text.</replacement>",
			want: "Corrected text.",
		},
		{
			name: "legacy JSON",
			raw:  `{"intent":"edit","supportReport":"","replacement":"Corrected text."}`,
			want: "Corrected text.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ParseAssistResult(test.raw, true)
			if result.Intent != IntentEdit || result.Replacement != test.want {
				t.Fatalf("unexpected result: %#v", result)
			}
		})
	}
}

func TestPromptContractUsesSafeAnswerFallbackForUnmarkedOutput(t *testing.T) {
	raw := "Corrected text without the requested JSON wrapper."
	result := ParseAssistResult(raw, true)

	if result.Intent != IntentQuestion || result.SupportReport != raw {
		t.Fatalf("unexpected fallback result: %#v", result)
	}
	if result.Replacement != "" {
		t.Fatalf("malformed structured output must not auto-replace text: %q", result.Replacement)
	}
}
