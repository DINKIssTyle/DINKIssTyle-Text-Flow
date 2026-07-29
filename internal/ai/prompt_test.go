package ai

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestBuildSystemPromptUsesCompactModeSpecificContracts(t *testing.T) {
	answerPrompt := BuildSystemPrompt(false)
	if !strings.Contains(answerPrompt, "Return only the answer") {
		t.Fatal("answer-only prompt does not require a direct answer")
	}
	if strings.Contains(answerPrompt, "Transformation requests default to REPLACE") {
		t.Fatal("answer-only prompt should not spend tokens on intent classification")
	}
	if wordCount := len(strings.Fields(answerPrompt)); wordCount > 120 {
		t.Fatalf("answer-only prompt is too long: %d words", wordCount)
	}

	editablePrompt := BuildSystemPrompt(true)
	required := []string{
		"Transformation requests default to REPLACE",
		"Use ANSWER only",
		"FORCE_REPLACE authorizes",
		"reason privately in this order",
		"Do not reveal this reasoning",
		"code, Markdown, HTML, or scripts",
		"exactly one valid JSON object",
		`{"mode":"FORCE_REPLACE|REPLACE|ANSWER","content":"completed content"}`,
	}
	for _, text := range required {
		if !strings.Contains(editablePrompt, text) {
			t.Fatalf("editable-selection prompt does not contain %q", text)
		}
	}
	if wordCount := len(strings.Fields(editablePrompt)); wordCount > 345 {
		t.Fatalf("editable-selection prompt is too long: %d words", wordCount)
	}
}

func TestBuildSystemPromptInstructionsAreEnglishOnly(t *testing.T) {
	koreanCharacters := regexp.MustCompile(`[가-힣]`)
	for _, prompt := range []string{
		BuildSystemPrompt(false),
		BuildSystemPrompt(true),
	} {
		if koreanCharacters.MatchString(prompt) {
			t.Fatal("system prompt contains Korean-language instructions")
		}
	}
}

func TestBuildSystemPromptForRequestPrioritizesCustomPrompt(t *testing.T) {
	request := AssistRequest{
		CustomPrompt: "  Always use a concise professional tone.  ",
	}
	prompt := BuildSystemPromptForRequest(request)
	customPrompt := strings.TrimSpace(request.CustomPrompt)
	basePrompt := BuildSystemPrompt(false)

	if !strings.HasPrefix(prompt, "HIGHEST-PRIORITY USER-CONFIGURED RULES:\n"+customPrompt) {
		t.Fatalf("custom rules were not placed first in the system prompt: %q", prompt)
	}
	if strings.Index(prompt, customPrompt) > strings.Index(prompt, basePrompt) {
		t.Fatal("custom rules were placed after the base prompt")
	}
	if !strings.Contains(prompt, "Apply these rules before the current instruction and conversation history") {
		t.Fatal("custom rules were not assigned highest semantic priority")
	}
	if !strings.Contains(prompt, basePrompt) {
		t.Fatal("custom rules replaced the mandatory base prompt")
	}
	if got := BuildSystemPromptForRequest(AssistRequest{}); got != basePrompt {
		t.Fatal("empty custom rules changed the base system prompt")
	}
}

func TestBuildUserPromptSerializesContextWithoutDuplicatingCustomRules(t *testing.T) {
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
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(BuildUserPrompt(request)), &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["appRules"]; exists {
		t.Fatal("custom rules were duplicated in the lower-priority user prompt")
	}
}

func TestCanReplaceSelectedTextRequiresEditableSelection(t *testing.T) {
	tests := []struct {
		name    string
		request AssistRequest
		want    bool
	}{
		{
			name: "editable selected text",
			request: AssistRequest{
				ContextKind: ContextSelectedText,
				ContextText: "selected",
				CanReplace:  true,
			},
			want: true,
		},
		{
			name: "read only selected text",
			request: AssistRequest{
				ContextKind: ContextSelectedText,
				ContextText: "selected",
				CanReplace:  false,
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
		{
			name: "empty selected text",
			request: AssistRequest{
				ContextKind: ContextSelectedText,
				ContextText: " \n ",
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

func TestParseAssistResultAlwaysAnswersWithoutEditableSelection(t *testing.T) {
	raw := "```markdown\n# Result\n\n`code`\n```"
	result := ParseAssistResult(raw, false)

	if result.Intent != IntentQuestion {
		t.Fatalf("unexpected intent: %q", result.Intent)
	}
	if result.SupportReport != raw {
		t.Fatalf("answer formatting changed:\nwant: %q\n got: %q", raw, result.SupportReport)
	}
	if result.Replacement != "" {
		t.Fatalf("answer-only result included a replacement: %q", result.Replacement)
	}
}

func TestParseAssistResultPreservesStructuredReplacementExactly(t *testing.T) {
	content := "  ```html\n<section>\n  <p>Hello</p>\n</section>\n```"
	result := ParseAssistResult("REPLACE\n"+content, true)
	if result.Intent != IntentEdit {
		t.Fatalf("unexpected intent: %q", result.Intent)
	}
	if result.Replacement != content {
		t.Fatalf("replacement formatting changed:\nwant: %q\n got: %q", content, result.Replacement)
	}
	if result.SupportReport != "" {
		t.Fatalf("replacement result included an answer: %q", result.SupportReport)
	}
}

func TestParseAssistResultAcceptsFencedJSONEnvelope(t *testing.T) {
	raw := "```json\n{\"mode\":\"answer\",\"content\":\"Direct answer\"}\n```"
	result := ParseAssistResult(raw, true)

	if result.Intent != IntentQuestion || result.SupportReport != "Direct answer" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseAssistResultFallsBackToSafeAnswerForMalformedJSON(t *testing.T) {
	raw := "Corrected text without the requested JSON wrapper."
	result := ParseAssistResult(raw, true)

	if result.Intent != IntentQuestion || result.SupportReport != raw {
		t.Fatalf("unexpected fallback result: %#v", result)
	}
	if result.Replacement != "" {
		t.Fatalf("malformed structured output must not auto-replace text: %q", result.Replacement)
	}
}
