package ai

import (
	"encoding/json"
	"testing"
)

type recordingChatClient struct {
	body     string
	response string
}

func (c *recordingChatClient) MakeRequest(_ string, _ map[string]string, body string) (string, error) {
	c.body = body
	return c.response, nil
}

func TestRunAssistUsesAnswerOnlyContractWithoutEditableSelection(t *testing.T) {
	client := &recordingChatClient{response: chatCompletionResponse(t, "완성된 응답")}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Model = "test-model"

	result, err := RunAssist(client, settings, AssistRequest{
		Instruction: "답변해 줘",
		ContextKind: ContextNone,
	})
	if err != nil {
		t.Fatalf("RunAssist returned an error: %v", err)
	}
	if result.Intent != IntentQuestion || result.SupportReport != "완성된 응답" {
		t.Fatalf("unexpected answer-only result: %#v", result)
	}

	var payload chatRequest
	if err := json.Unmarshal([]byte(client.body), &payload); err != nil {
		t.Fatalf("chat request is invalid JSON: %v", err)
	}
	if len(payload.Messages) != 2 {
		t.Fatalf("unexpected message count: %d", len(payload.Messages))
	}
	if payload.Messages[0].Content != BuildSystemPrompt(false) {
		t.Fatal("chat request did not use the answer-only system prompt")
	}
}

func TestRunAssistUsesStructuredContractForEditableSelection(t *testing.T) {
	content := "```go\nfunc main() {}\n```"
	structuredContent, err := json.Marshal(structuredAssistResponse{
		Mode:    "REPLACE",
		Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &recordingChatClient{response: chatCompletionResponse(t, string(structuredContent))}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Model = "test-model"

	result, err := RunAssist(client, settings, AssistRequest{
		Instruction: "Fix the selected Go code.",
		ContextKind: ContextSelectedText,
		ContextText: "func main( {",
		CanReplace:  true,
	})
	if err != nil {
		t.Fatalf("RunAssist returned an error: %v", err)
	}
	if result.Intent != IntentEdit || result.Replacement != content || !result.ForceReplace {
		t.Fatalf("unexpected editable-selection result: %#v", result)
	}

	var payload chatRequest
	if err := json.Unmarshal([]byte(client.body), &payload); err != nil {
		t.Fatalf("chat request is invalid JSON: %v", err)
	}
	if payload.Messages[0].Content != BuildSystemPrompt(true) {
		t.Fatal("chat request did not use the editable-selection system prompt")
	}
}

func chatCompletionResponse(t *testing.T, content string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"content": content,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
