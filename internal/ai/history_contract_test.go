package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

type queuedChatClient struct {
	endpoints []string
	bodies    []string
	responses []string
}

func (client *queuedChatClient) MakeRequest(
	endpoint string,
	_ map[string]string,
	body string,
) (string, error) {
	client.endpoints = append(client.endpoints, endpoint)
	client.bodies = append(client.bodies, body)
	response := client.responses[0]
	client.responses = client.responses[1:]
	return response, nil
}

func TestDefaultSettingsUseTenHistoryTurns(t *testing.T) {
	settings := DefaultSettings()
	if settings.HistoryEnabled {
		t.Fatal("history must be opt-in by default")
	}
	if settings.HistoryCount != 10 {
		t.Fatalf("unexpected default history count: %d", settings.HistoryCount)
	}
	if !settings.TTSShowAudioActions {
		t.Fatal("synthesized audio actions must be visible by default")
	}

	settings.HistoryCount = 0
	if normalized := NormalizeSettings(settings); normalized.HistoryCount != 10 {
		t.Fatalf("zero history count was not normalized: %d", normalized.HistoryCount)
	}
	settings.HistoryCount = 101
	if normalized := NormalizeSettings(settings); normalized.HistoryCount != 100 {
		t.Fatalf("history count was not capped: %d", normalized.HistoryCount)
	}
}

func TestOpenAIHistoryReplaysOnlyRecentTurns(t *testing.T) {
	client := &queuedChatClient{responses: []string{
		chatCompletionResponse(t, "ANSWER\nfirst answer"),
		chatCompletionResponse(t, "ANSWER\nsecond answer"),
		chatCompletionResponse(t, "ANSWER\nthird answer"),
	}}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Model = "test-model"
	settings.HistoryEnabled = true
	settings.HistoryCount = 1
	history := NewConversationHistory()

	requests := []AssistRequest{
		{Instruction: "first question", ContextKind: ContextNone},
		{Instruction: "second question", ContextKind: ContextNone},
		{Instruction: "third question", ContextKind: ContextNone},
	}
	for _, request := range requests {
		if _, err := RunAssistWithHistory(client, settings, request, history); err != nil {
			t.Fatalf("RunAssistWithHistory returned an error: %v", err)
		}
	}

	var second chatRequest
	if err := json.Unmarshal([]byte(client.bodies[1]), &second); err != nil {
		t.Fatal(err)
	}
	if len(second.Messages) != 4 {
		t.Fatalf("second request message count = %d, want 4", len(second.Messages))
	}
	if second.Messages[1].Content != BuildUserPrompt(requests[0]) ||
		second.Messages[2].Content != "ANSWER\nfirst answer" {
		t.Fatalf("second request did not include the first turn: %#v", second.Messages)
	}

	var third chatRequest
	if err := json.Unmarshal([]byte(client.bodies[2]), &third); err != nil {
		t.Fatal(err)
	}
	if len(third.Messages) != 4 {
		t.Fatalf("third request message count = %d, want 4", len(third.Messages))
	}
	if third.Messages[1].Content != BuildUserPrompt(requests[1]) ||
		third.Messages[2].Content != "ANSWER\nsecond answer" {
		t.Fatalf("third request did not keep only the most recent turn: %#v", third.Messages)
	}
}

func TestLMStudioHistoryUsesStatefulChatAndRollsOverAtLimit(t *testing.T) {
	client := &queuedChatClient{responses: []string{
		lmStudioResponse(t, "ANSWER\nfirst answer", "resp_first"),
		lmStudioResponse(t, "ANSWER\nsecond answer", "resp_second"),
		lmStudioResponse(t, "ANSWER\nthird answer", "resp_third"),
	}}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderLMStudio
	settings.Endpoint = "localhost:1234/v1/chat/completions"
	settings.Model = "local-model"
	settings.HistoryEnabled = true
	settings.HistoryCount = 2
	history := NewConversationHistory()

	for _, instruction := range []string{"first", "second", "third"} {
		if _, err := RunAssistWithHistory(client, settings, AssistRequest{
			Instruction: instruction,
			ContextKind: ContextNone,
		}, history); err != nil {
			t.Fatalf("RunAssistWithHistory returned an error: %v", err)
		}
	}

	for _, endpoint := range client.endpoints {
		if endpoint != "http://localhost:1234/api/v1/chat" {
			t.Fatalf("unexpected LM Studio endpoint: %q", endpoint)
		}
	}

	var first, second, third lmStudioChatRequest
	for index, target := range []*lmStudioChatRequest{&first, &second, &third} {
		if err := json.Unmarshal([]byte(client.bodies[index]), target); err != nil {
			t.Fatal(err)
		}
		if !target.Store {
			t.Fatalf("request %d did not enable stateful storage", index)
		}
	}
	if first.PreviousResponseID != "" {
		t.Fatalf("first request unexpectedly continued a chat: %q", first.PreviousResponseID)
	}
	if first.SystemPrompt == "" {
		t.Fatal("first request did not include a system prompt")
	}
	if second.PreviousResponseID != "resp_first" {
		t.Fatalf("second request previous_response_id = %q", second.PreviousResponseID)
	}
	if second.SystemPrompt != "" {
		t.Fatal("continued request unexpectedly repeated the system prompt")
	}
	var secondFields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(client.bodies[1]), &secondFields); err != nil {
		t.Fatal(err)
	}
	if _, exists := secondFields["system_prompt"]; exists {
		t.Fatal("continued request serialized the system_prompt field")
	}
	if third.PreviousResponseID != "" {
		t.Fatalf("third request should start a new bounded chain: %q", third.PreviousResponseID)
	}
	if third.SystemPrompt == "" {
		t.Fatal("new bounded chain did not include a system prompt")
	}
}

func TestLMStudioHistoryStartsNewChainWhenSystemPromptChanges(t *testing.T) {
	client := &queuedChatClient{responses: []string{
		lmStudioResponse(t, "first answer", "resp_first"),
		lmStudioResponse(t, `{"mode":"REPLACE","content":"second answer"}`, "resp_second"),
		lmStudioResponse(t, `{"mode":"REPLACE","content":"third answer"}`, "resp_third"),
	}}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderLMStudio
	settings.Model = "local-model"
	settings.HistoryEnabled = true
	settings.HistoryCount = 10
	history := NewConversationHistory()

	requests := []AssistRequest{
		{Instruction: "first", ContextKind: ContextNone},
		{
			Instruction: "summarize",
			ContextKind: ContextSelectedText,
			ContextText: "selected text",
			CanReplace:  true,
		},
		{
			Instruction: "rewrite",
			ContextKind: ContextSelectedText,
			ContextText: "selected text",
			CanReplace:  true,
		},
	}
	for _, request := range requests {
		if _, err := RunAssistWithHistory(client, settings, request, history); err != nil {
			t.Fatalf("RunAssistWithHistory returned an error: %v", err)
		}
	}

	var first, second, third lmStudioChatRequest
	for index, target := range []*lmStudioChatRequest{&first, &second, &third} {
		if err := json.Unmarshal([]byte(client.bodies[index]), target); err != nil {
			t.Fatal(err)
		}
	}
	if first.SystemPrompt != BuildSystemPrompt(false) {
		t.Fatal("first request did not include the read-only system prompt")
	}
	if second.PreviousResponseID != "" {
		t.Fatalf("changed system prompt continued the old chain: %q", second.PreviousResponseID)
	}
	if second.SystemPrompt != BuildSystemPrompt(true) {
		t.Fatal("new chain did not include the editable-selection system prompt")
	}
	if third.PreviousResponseID != "resp_second" {
		t.Fatalf("unchanged system prompt did not continue the new chain: %q", third.PreviousResponseID)
	}
	if third.SystemPrompt != "" {
		t.Fatal("continued request unexpectedly repeated the system prompt")
	}
}

func TestLMStudioStatefulResponseRequiresResponseID(t *testing.T) {
	response := lmStudioResponse(t, "ANSWER\nhello", "")
	if _, _, err := ExtractLMStudioChatContent(response); err == nil ||
		!strings.Contains(err.Error(), "response_id") {
		t.Fatalf("expected response_id error, got %v", err)
	}
}

func TestLMStudioStatefulChatSendsScreenshotAsImageInput(t *testing.T) {
	client := &queuedChatClient{responses: []string{
		lmStudioResponse(t, "Screenshot description", "resp_image"),
	}}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderLMStudio
	settings.Model = "vision-model"
	settings.HistoryEnabled = true
	history := NewConversationHistory()
	imageDataURL := "data:image/png;base64,c2NyZWVuc2hvdA=="

	if _, err := RunAssistWithHistory(client, settings, AssistRequest{
		ImageDataURL: imageDataURL,
	}, history); err != nil {
		t.Fatalf("RunAssistWithHistory returned an error: %v", err)
	}

	var payload struct {
		Input []lmStudioChatInput `json:"input"`
	}
	if err := json.Unmarshal([]byte(client.bodies[0]), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Input) != 2 ||
		payload.Input[0].Type != "text" ||
		payload.Input[1].Type != "image" ||
		payload.Input[1].DataURL != imageDataURL {
		t.Fatalf("unexpected LM Studio multimodal input: %#v", payload.Input)
	}
}

func TestLMStudioChatEndpointNormalizesSupportedInputs(t *testing.T) {
	for _, endpoint := range []string{
		"localhost:1234",
		"http://localhost:1234/v1",
		"http://localhost:1234/v1/chat/completions",
		"http://localhost:1234/api/v1",
		"http://localhost:1234/api/v1/chat",
	} {
		if actual := LMStudioChatEndpoint(endpoint); actual != "http://localhost:1234/api/v1/chat" {
			t.Fatalf("LMStudioChatEndpoint(%q) = %q", endpoint, actual)
		}
	}
}

func lmStudioResponse(t *testing.T, content, responseID string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"output": []any{
			map[string]any{
				"type":    "message",
				"content": content,
			},
		},
		"response_id": responseID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
