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
	for _, required := range []string{
		"answer in the instruction's language",
		"Never infer the answer language from the context",
		"foreign-language context does not authorize a foreign-language answer",
	} {
		if !strings.Contains(answerPrompt, required) {
			t.Fatalf("answer-only prompt does not contain language rule %q", required)
		}
	}
	if wordCount := len(strings.Fields(answerPrompt)); wordCount > 125 {
		t.Fatalf("answer-only prompt is too long: %d words", wordCount)
	}

	editablePrompt := BuildSystemPrompt(true)
	for _, required := range []string{
		"app routing labels",
		"FORCE_REPLACE authorizes",
		"explicitly directs an edit or insertion",
		"changed or derived selection",
		"Polite or question wording",
		"Use ANSWER only for information",
		"Never label transformed selection content as ANSWER",
		"reason privately in this order",
		"usable revision or derivative",
		"Do not reveal this reasoning",
		"code, Markdown, HTML, or scripts",
		"ANSWER language",
		"foreign-language selection",
		"REPLACE language",
		"otherwise preserve the selected content's language",
		"exactly one valid JSON object",
		`{"mode":"FORCE_REPLACE|REPLACE|ANSWER","content":"completed content"}`,
		"never repeat FORCE_REPLACE, REPLACE, or ANSWER inside content",
		"no Markdown fence, commentary, or text outside the JSON object",
	} {
		if !strings.Contains(editablePrompt, required) {
			t.Fatalf("editable-selection prompt does not contain %q", required)
		}
	}
	if wordCount := len(strings.Fields(editablePrompt)); wordCount > 320 {
		t.Fatalf("editable-selection prompt is too long: %d words", wordCount)
	}
}

func TestPromptContractSerializesUntrustedContextWithCustomRules(t *testing.T) {
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
	if payload.AppRules != request.CustomPrompt {
		t.Fatalf("app rules changed during serialization: %q", payload.AppRules)
	}
	if payload.Context == nil ||
		payload.Context.Kind != ContextSelectedFile ||
		payload.Context.Content != request.ContextText ||
		payload.Context.FilePath != request.FilePath {
		t.Fatalf("unexpected serialized context: %#v", payload.Context)
	}
}

func TestPromptContractMarksImageWithoutEmbeddingItsData(t *testing.T) {
	request := AssistRequest{
		Instruction:  "Describe this.",
		ImageDataURL: "data:image/png;base64,c2NyZWVuc2hvdA==",
	}

	prompt := BuildUserPrompt(request)
	var payload promptPayload
	if err := json.Unmarshal([]byte(prompt), &payload); err != nil {
		t.Fatalf("user prompt is not valid JSON: %v", err)
	}
	if !payload.ImageAttached {
		t.Fatal("user prompt did not mark the attached image")
	}
	if strings.Contains(prompt, request.ImageDataURL) {
		t.Fatal("user prompt embedded the image data URL")
	}
}

func TestPromptContractMarksExplicitEditDirectivesForForcedReplacement(t *testing.T) {
	tests := []struct {
		instruction string
		want        bool
	}{
		{instruction: "이 문장을 수정해", want: true},
		{instruction: "선택문을 영어 문장으로 교체해", want: true},
		{instruction: "문장을 개선해", want: true},
		{instruction: "작성", want: true},
		{instruction: "이 주제로 글을 써줘", want: true},
		{instruction: "내용을 완성해줘", want: true},
		{instruction: "Fix the grammar", want: true},
		{instruction: "Improve this sentence", want: true},
		{instruction: "Write an article about this", want: true},
		{instruction: "Complete this draft", want: true},
		{instruction: "Please edit this", want: true},
		{instruction: "この文章を修正して", want: true},
		{instruction: "修改这句话", want: true},
		{instruction: "Translate to English", want: false},
		{instruction: "What does this mean?", want: false},
	}

	for _, test := range tests {
		t.Run(test.instruction, func(t *testing.T) {
			request := AssistRequest{
				Instruction: test.instruction,
				ContextKind: ContextSelectedText,
				ContextText: "selected",
				CanReplace:  true,
			}
			if got := RequiresForcedReplacement(request); got != test.want {
				t.Fatalf("RequiresForcedReplacement() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPromptContractDoesNotForceWithoutSelectedText(t *testing.T) {
	request := AssistRequest{
		Instruction: "Insert this",
		ContextKind: ContextNone,
		CanReplace:  true,
	}
	if RequiresForcedReplacement(request) {
		t.Fatal("request without selected text must not force replacement")
	}
}

func TestPromptContractSerializesRequiredForcedMode(t *testing.T) {
	request := AssistRequest{
		Instruction: "선택문을 수정해",
		ContextKind: ContextSelectedText,
		ContextText: "selected",
		CanReplace:  true,
	}

	var payload promptPayload
	if err := json.Unmarshal([]byte(BuildUserPrompt(request)), &payload); err != nil {
		t.Fatalf("user prompt is not valid JSON: %v", err)
	}
	if payload.RequiredMode != "FORCE_REPLACE" {
		t.Fatalf("unexpected required mode: %q", payload.RequiredMode)
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

func TestPromptContractParsesForcedReplacement(t *testing.T) {
	result := ParseAssistResult("FORCE_REPLACE\nCorrected text.", true)
	if result.Intent != IntentEdit ||
		result.Replacement != "Corrected text." ||
		!result.ForceReplace {
		t.Fatalf("unexpected forced replacement result: %#v", result)
	}
}

func TestPromptContractDoesNotForceOrdinaryReplacement(t *testing.T) {
	result := ParseAssistResult("REPLACE\nTranslated text.", true)
	if result.Intent != IntentEdit ||
		result.Replacement != "Translated text." ||
		result.ForceReplace {
		t.Fatalf("unexpected ordinary replacement result: %#v", result)
	}
}

func TestPromptContractStripsRoutingMetadataWithoutSelectionPermission(t *testing.T) {
	raw := "FORCE_REPLACE\nCorrected text."
	result := ParseAssistResult(raw, false)
	if result.Intent != IntentQuestion ||
		result.SupportReport != "Corrected text." ||
		result.Replacement != "" ||
		result.ForceReplace {
		t.Fatalf("unexpected answer-only result: %#v", result)
	}
}

func TestPromptContractEnforcesExplicitEditDirectiveWhenModelAnswers(t *testing.T) {
	request := AssistRequest{
		Instruction: "이 문장으로 교체해",
		ContextKind: ContextSelectedText,
		ContextText: "old text",
		CanReplace:  true,
	}
	result := ParseAssistResultForRequest("ANSWER\nNew text.", request)
	if result.Intent != IntentEdit ||
		result.Replacement != "New text." ||
		!result.ForceReplace ||
		result.SupportReport != "" {
		t.Fatalf("unexpected enforced replacement result: %#v", result)
	}
}

func TestPromptContractParsesAnswerMarker(t *testing.T) {
	result := ParseAssistResult("ANSWER\nThe selected text means this.", true)
	if result.Intent != IntentQuestion || result.SupportReport != "The selected text means this." {
		t.Fatalf("unexpected answer result: %#v", result)
	}
}

func TestPromptContractParsesJSONResponse(t *testing.T) {
	raw, err := json.Marshal(structuredAssistResponse{
		Mode:    "replace",
		Content: "Corrected text.",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := ParseAssistResult(string(raw), true)
	if result.Intent != IntentEdit || result.Replacement != "Corrected text." {
		t.Fatalf("unexpected JSON result: %#v", result)
	}
}

func TestPromptContractRepairsRedundantJSONClosingBrace(t *testing.T) {
	content := "If you say the president's re-election is impossible, that's enough, right?\n\n" +
		"It's because you guys are the ones who try to break that system!!"
	raw, err := json.Marshal(structuredAssistResponse{
		Mode:    "REPLACE",
		Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '}')

	result := ParseAssistResult(string(raw), true)
	if result.Intent != IntentEdit ||
		result.Replacement != content ||
		result.SupportReport != "" {
		t.Fatalf("redundant closing brace leaked routing JSON: %#v", result)
	}
}

func TestPromptContractStripsRepeatedModeFromJSONContent(t *testing.T) {
	raw, err := json.Marshal(structuredAssistResponse{
		Mode:    "replace",
		Content: "REPLACE\n韓文真的是一種很棒的文字。",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := ParseAssistResult(string(raw), true)
	if result.Intent != IntentEdit || result.Replacement != "韓文真的是一種很棒的文字。" {
		t.Fatalf("unexpected repeated-mode result: %#v", result)
	}
}

func TestPromptContractDoesNotInsertMalformedResponseForExplicitEdit(t *testing.T) {
	raw := `{"mode":"REPLACE","content":"unterminated}`
	request := AssistRequest{
		Instruction: "이 문장으로 교체해",
		ContextKind: ContextSelectedText,
		ContextText: "old text",
		CanReplace:  true,
	}
	result := ParseAssistResultForRequest(raw, request)
	if result.Intent != IntentQuestion ||
		result.SupportReport != raw ||
		result.Replacement != "" ||
		result.ForceReplace {
		t.Fatalf("malformed response must not be inserted: %#v", result)
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
			name: "repeated marker",
			raw:  "REPLACE\nREPLACE\nCorrected text.",
			want: "Corrected text.",
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

func TestPromptContractParsesJSONWithTrailingCharacters(t *testing.T) {
	raw := `{"mode":"FORCE_REPLACE","content":"Hello"}뇨`
	result := ParseAssistResult(raw, true)

	if result.Intent != IntentEdit || result.Replacement != "Hello" || !result.ForceReplace {
		t.Fatalf("failed to parse JSON envelope with trailing characters: %#v", result)
	}
}

func TestPromptContractParsesJSONWithoutModeField(t *testing.T) {
	raw := "```json\n{\n  \"content\": \"애플의 여정은 정말 대단했어요!\"\n}\n```"
	result := ParseAssistResult(raw, false)

	if result.Intent != IntentQuestion || result.SupportReport != "애플의 여정은 정말 대단했어요!" {
		t.Fatalf("failed to parse JSON envelope without mode field: %#v", result)
	}
}
