package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeAppleIntelligenceClient struct {
	instructions string
	prompt       string
	response     string
	err          error
	cancelled    bool
}

func (c *fakeAppleIntelligenceClient) Generate(instructions string, prompt string) (string, error) {
	c.instructions = instructions
	c.prompt = prompt
	return c.response, c.err
}

func (c *fakeAppleIntelligenceClient) Status() AppleIntelligenceStatus {
	return AppleIntelligenceStatus{
		Available: true,
		State:     AppleIntelligenceStateAvailable,
	}
}

func (c *fakeAppleIntelligenceClient) Cancel() {
	c.cancelled = true
}

func TestRunAppleIntelligenceAssistUsesSharedPromptContract(t *testing.T) {
	client := &fakeAppleIntelligenceClient{
		response: "A substantive answer.",
	}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderAppleIntelligence

	result, err := RunAppleIntelligenceAssist(client, settings, AssistRequest{
		Instruction: "Answer this question.",
		ContextKind: ContextNone,
	})
	if err != nil {
		t.Fatalf("RunAppleIntelligenceAssist returned an error: %v", err)
	}
	if result.Intent != IntentQuestion || result.SupportReport != "A substantive answer." {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !strings.Contains(client.instructions, "Return only the answer") {
		t.Fatal("Apple Intelligence did not receive the shared system prompt")
	}
	var payload promptPayload
	if err := json.Unmarshal([]byte(client.prompt), &payload); err != nil {
		t.Fatalf("Apple Intelligence did not receive a JSON user prompt: %v", err)
	}
	if payload.Instruction != "Answer this question." {
		t.Fatalf("unexpected user instruction: %q", payload.Instruction)
	}
}

func TestRunAppleIntelligenceAssistDoesNotRequireEndpointOrModel(t *testing.T) {
	client := &fakeAppleIntelligenceClient{
		response: "Draft text.",
	}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderAppleIntelligence
	settings.Endpoint = ""
	settings.Model = ""

	result, err := RunAppleIntelligenceAssist(client, settings, AssistRequest{
		Instruction: "Draft text.",
		ContextKind: ContextNone,
	})
	if err != nil {
		t.Fatalf("Apple Intelligence should not require an endpoint or model: %v", err)
	}
	if result.Intent != IntentQuestion || result.SupportReport != "Draft text." {
		t.Fatalf("requests without an editable selection must be answers: %#v", result)
	}
}

func TestRunAppleIntelligenceAssistCanReplaceEditableSelection(t *testing.T) {
	client := &fakeAppleIntelligenceClient{
		response: "REPLACE\nCorrected text.",
	}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderAppleIntelligence

	result, err := RunAppleIntelligenceAssist(client, settings, AssistRequest{
		Instruction: "Correct the selected text.",
		ContextKind: ContextSelectedText,
		ContextText: "Corected text.",
		CanReplace:  true,
	})
	if err != nil {
		t.Fatalf("RunAppleIntelligenceAssist returned an error: %v", err)
	}
	if result.Intent != IntentEdit || result.Replacement != "Corrected text." {
		t.Fatalf("unexpected editable-selection result: %#v", result)
	}
	if !strings.Contains(client.instructions, "Transformation requests default to REPLACE") {
		t.Fatal("Apple Intelligence did not receive the editable-selection contract")
	}
}

func TestRunAppleIntelligenceAssistPropagatesGenerationError(t *testing.T) {
	expected := errors.New("model unavailable")
	client := &fakeAppleIntelligenceClient{err: expected}
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderAppleIntelligence

	_, err := RunAppleIntelligenceAssist(client, settings, AssistRequest{Instruction: "Hello"})
	if !errors.Is(err, expected) {
		t.Fatalf("expected generation error, got %v", err)
	}
}

func TestNormalizeSettingsPreservesAppleIntelligenceProvider(t *testing.T) {
	settings := NormalizeSettings(Settings{Provider: ProviderAppleIntelligence})
	if settings.Provider != ProviderAppleIntelligence {
		t.Fatalf("provider was normalized to %q", settings.Provider)
	}
}
