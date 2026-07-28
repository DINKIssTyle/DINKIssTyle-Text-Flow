package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

type contractAppleIntelligenceClient struct {
	instructions string
	response     string
}

func TestAppleAssistRejectsScreenshotInput(t *testing.T) {
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderAppleIntelligence

	_, err := RunAppleIntelligenceAssist(
		&contractAppleIntelligenceClient{},
		settings,
		AssistRequest{
			Instruction:  "Describe this.",
			ImageDataURL: "data:image/png;base64,c2NyZWVuc2hvdA==",
		},
	)
	if err == nil || !strings.Contains(err.Error(), "does not support screenshot") {
		t.Fatalf("expected screenshot support error, got %v", err)
	}
}

func (c *contractAppleIntelligenceClient) Generate(instructions string, _ string) (string, error) {
	c.instructions = instructions
	return c.response, nil
}

func (c *contractAppleIntelligenceClient) Status() AppleIntelligenceStatus {
	return AppleIntelligenceStatus{Available: true, State: AppleIntelligenceStateAvailable}
}

func (c *contractAppleIntelligenceClient) Cancel() {}

func TestAppleAssistUsesTheSameModeSpecificPromptContract(t *testing.T) {
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Provider = ProviderAppleIntelligence

	answerClient := &contractAppleIntelligenceClient{response: "Direct answer"}
	answer, err := RunAppleIntelligenceAssist(answerClient, settings, AssistRequest{
		Instruction: "Answer this.",
	})
	if err != nil {
		t.Fatalf("answer-only request failed: %v", err)
	}
	if answer.Intent != IntentQuestion || answer.SupportReport != "Direct answer" {
		t.Fatalf("unexpected answer result: %#v", answer)
	}
	if answerClient.instructions != BuildSystemPrompt(false) {
		t.Fatal("Apple Intelligence did not use the answer-only prompt")
	}

	editResponse, err := json.Marshal(structuredAssistResponse{
		Mode:    "REPLACE",
		Content: "Corrected text.",
	})
	if err != nil {
		t.Fatal(err)
	}
	editClient := &contractAppleIntelligenceClient{
		response: string(editResponse),
	}
	edit, err := RunAppleIntelligenceAssist(editClient, settings, AssistRequest{
		Instruction: "Correct this.",
		ContextKind: ContextSelectedText,
		ContextText: "Corect this.",
		CanReplace:  true,
	})
	if err != nil {
		t.Fatalf("editable-selection request failed: %v", err)
	}
	if edit.Intent != IntentEdit || edit.Replacement != "Corrected text." || !edit.ForceReplace {
		t.Fatalf("unexpected edit result: %#v", edit)
	}
	if editClient.instructions != BuildSystemPrompt(true) {
		t.Fatal("Apple Intelligence did not use the editable-selection prompt")
	}
}
