package ai

import (
	"encoding/json"
	"errors"
	"strings"
)

type ChatClient interface {
	MakeRequest(endpoint string, headers map[string]string, body string) (string, error)
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Text string `json:"text"`
	} `json:"choices"`
}

func RunAssist(client ChatClient, settings Settings, request AssistRequest) (AssistResult, error) {
	settings = NormalizeSettings(settings)
	if !settings.Enabled {
		return AssistResult{}, errors.New("AI assistant is disabled")
	}
	if strings.TrimSpace(settings.Model) == "" {
		return AssistResult{}, errors.New("AI model is required")
	}
	if strings.TrimSpace(request.Instruction) == "" {
		return AssistResult{}, errors.New("AI instruction is required")
	}
	if client == nil {
		return AssistResult{}, errors.New("AI client is required")
	}

	canReplace := CanReplaceSelectedText(request)
	payload := chatRequest{
		Model: settings.Model,
		Messages: []chatMessage{
			{Role: "system", Content: BuildSystemPrompt(canReplace)},
			{Role: "user", Content: BuildUserPrompt(request)},
		},
	}
	if settings.Temperature > 0 {
		payload.Temperature = &settings.Temperature
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return AssistResult{}, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if settings.APIKey != "" {
		headers["Authorization"] = "Bearer " + settings.APIKey
	}

	responseText, err := client.MakeRequest(ChatCompletionsEndpoint(settings.Endpoint), headers, string(body))
	if err != nil {
		return AssistResult{}, err
	}

	content, err := ExtractChatContent(responseText)
	if err != nil {
		return AssistResult{}, err
	}
	return ParseAssistResultForRequest(content, request), nil
}

func ExtractChatContent(responseText string) (string, error) {
	var response chatResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", errors.New("AI response did not include choices")
	}
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		content = strings.TrimSpace(response.Choices[0].Text)
	}
	if content == "" {
		return "", errors.New("AI response did not include content")
	}
	return content, nil
}

func ChatCompletionsEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/v1/chat/completions") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/chat/completions") {
		return endpoint
	}
	if strings.HasSuffix(endpoint, "/v1") {
		return endpoint + "/chat/completions"
	}
	return endpoint + "/v1/chat/completions"
}
