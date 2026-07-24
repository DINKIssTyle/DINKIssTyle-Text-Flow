package ai

import (
	"errors"
	"strings"
)

const (
	AppleIntelligenceStateAvailable         = "available"
	AppleIntelligenceStateDeviceNotEligible = "device_not_eligible"
	AppleIntelligenceStateNotEnabled        = "not_enabled"
	AppleIntelligenceStateModelNotReady     = "model_not_ready"
	AppleIntelligenceStateOSUnsupported     = "os_unsupported"
	AppleIntelligenceStateSDKUnavailable    = "sdk_unavailable"
	AppleIntelligenceStateHelperUnavailable = "helper_unavailable"
	AppleIntelligenceStateUnavailable       = "unavailable"
	AppleIntelligenceStateChecking          = "checking"
)

type AppleIntelligenceStatus struct {
	Available bool   `json:"available"`
	State     string `json:"state"`
	Detail    string `json:"detail,omitempty"`
}

type AppleIntelligenceClient interface {
	Generate(instructions string, prompt string) (string, error)
	Status() AppleIntelligenceStatus
	Cancel()
}

func RunAppleIntelligenceAssist(client AppleIntelligenceClient, settings Settings, request AssistRequest) (AssistResult, error) {
	settings = NormalizeSettings(settings)
	if !settings.Enabled {
		return AssistResult{}, errors.New("AI assistant is disabled")
	}
	if strings.TrimSpace(request.Instruction) == "" {
		return AssistResult{}, errors.New("AI instruction is required")
	}
	if client == nil {
		return AssistResult{}, errors.New("Apple Intelligence client is required")
	}

	hasContext := request.ContextKind != ContextNone && strings.TrimSpace(request.ContextText) != ""
	content, err := client.Generate(
		BuildSystemPrompt(hasContext, request.CustomPrompt),
		BuildUserPrompt(request),
	)
	if err != nil {
		return AssistResult{}, err
	}
	return ParseAssistResult(content), nil
}
