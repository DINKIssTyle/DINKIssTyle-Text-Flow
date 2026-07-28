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
	Content any    `json:"content"`
}

type openAIChatContentPart struct {
	Type     string              `json:"type"`
	Text     string              `json:"text,omitempty"`
	ImageURL *openAIChatImageURL `json:"image_url,omitempty"`
}

type openAIChatImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
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
	return RunAssistWithHistory(client, settings, request, nil)
}

func RunAssistWithHistory(
	client ChatClient,
	settings Settings,
	request AssistRequest,
	history *ConversationHistory,
) (AssistResult, error) {
	settings = NormalizeSettings(settings)
	if !settings.Enabled {
		return AssistResult{}, errors.New("AI assistant is disabled")
	}
	if strings.TrimSpace(settings.Model) == "" {
		return AssistResult{}, errors.New("AI model is required")
	}
	request.Instruction = strings.TrimSpace(request.Instruction)
	if request.Instruction == "" && strings.TrimSpace(request.ImageDataURL) != "" {
		request.Instruction = "Describe this screenshot."
	}
	if request.Instruction == "" {
		return AssistResult{}, errors.New("AI instruction is required")
	}
	if request.ImageDataURL != "" && !isSupportedImageDataURL(request.ImageDataURL) {
		return AssistResult{}, errors.New("screen capture image data is invalid")
	}
	if client == nil {
		return AssistResult{}, errors.New("AI client is required")
	}

	if !settings.HistoryEnabled || history == nil {
		if history != nil {
			history.Reset()
		}
		return runOpenAICompatibleAssist(client, settings, request, nil)
	}

	history.mu.Lock()
	defer history.mu.Unlock()
	history.prepareLocked(settings)

	if settings.Provider == ProviderLMStudio {
		return runLMStudioStatefulAssist(client, settings, request, history)
	}
	return runOpenAICompatibleAssist(client, settings, request, history)
}

func runOpenAICompatibleAssist(
	client ChatClient,
	settings Settings,
	request AssistRequest,
	history *ConversationHistory,
) (AssistResult, error) {
	canReplace := CanReplaceSelectedText(request)
	userPrompt := BuildUserPrompt(request)
	messages := []chatMessage{
		{Role: "system", Content: BuildSystemPrompt(canReplace)},
	}
	if history != nil {
		messages = append(messages, history.openAIMessagesLocked(settings.HistoryCount)...)
	}
	userContent := any(userPrompt)
	if request.ImageDataURL != "" {
		userContent = []openAIChatContentPart{
			{Type: "text", Text: userPrompt},
			{
				Type: "image_url",
				ImageURL: &openAIChatImageURL{
					URL:    request.ImageDataURL,
					Detail: "auto",
				},
			},
		}
	}
	messages = append(messages, chatMessage{Role: "user", Content: userContent})

	payload := chatRequest{
		Model:    settings.Model,
		Messages: messages,
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
	if history != nil {
		history.appendOpenAITurnLocked(userPrompt, content, settings.HistoryCount)
	}
	return ParseAssistResultForRequest(content, request), nil
}

type lmStudioChatRequest struct {
	Model              string   `json:"model"`
	Input              any      `json:"input"`
	SystemPrompt       string   `json:"system_prompt"`
	Temperature        *float64 `json:"temperature,omitempty"`
	Store              bool     `json:"store"`
	PreviousResponseID string   `json:"previous_response_id,omitempty"`
}

type lmStudioChatInput struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	DataURL string `json:"data_url,omitempty"`
}

type lmStudioChatResponse struct {
	Output []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	} `json:"output"`
	ResponseID string `json:"response_id"`
}

func runLMStudioStatefulAssist(
	client ChatClient,
	settings Settings,
	request AssistRequest,
	history *ConversationHistory,
) (AssistResult, error) {
	userPrompt := BuildUserPrompt(request)
	input := any(userPrompt)
	if request.ImageDataURL != "" {
		input = []lmStudioChatInput{
			{Type: "text", Content: userPrompt},
			{Type: "image", DataURL: request.ImageDataURL},
		}
	}
	payload := lmStudioChatRequest{
		Model:              settings.Model,
		Input:              input,
		SystemPrompt:       BuildSystemPrompt(CanReplaceSelectedText(request)),
		Store:              true,
		PreviousResponseID: history.prepareLMStudioTurnLocked(settings.HistoryCount),
	}
	if settings.Temperature > 0 {
		temperature := settings.Temperature
		if temperature > 1 {
			temperature = 1
		}
		payload.Temperature = &temperature
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

	responseText, err := client.MakeRequest(LMStudioChatEndpoint(settings.Endpoint), headers, string(body))
	if err != nil {
		return AssistResult{}, err
	}

	content, responseID, err := ExtractLMStudioChatContent(responseText)
	if err != nil {
		return AssistResult{}, err
	}
	history.commitLMStudioTurnLocked(responseID)
	return ParseAssistResultForRequest(content, request), nil
}

func isSupportedImageDataURL(value string) bool {
	for _, prefix := range []string{
		"data:image/png;base64,",
		"data:image/jpeg;base64,",
		"data:image/webp;base64,",
	} {
		if strings.HasPrefix(value, prefix) && len(value) > len(prefix) {
			return true
		}
	}
	return false
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

func ExtractLMStudioChatContent(responseText string) (string, string, error) {
	var response lmStudioChatResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return "", "", err
	}
	messages := make([]string, 0, len(response.Output))
	for _, output := range response.Output {
		if output.Type != "message" {
			continue
		}
		if content := strings.TrimSpace(output.Content); content != "" {
			messages = append(messages, content)
		}
	}
	if len(messages) == 0 {
		return "", "", errors.New("LM Studio response did not include message output")
	}
	responseID := strings.TrimSpace(response.ResponseID)
	if !strings.HasPrefix(responseID, "resp_") {
		return "", "", errors.New("LM Studio stateful response did not include a valid response_id")
	}
	return strings.Join(messages, "\n"), responseID, nil
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

func LMStudioChatEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")
	for _, suffix := range []string{
		"/api/v1/chat",
		"/api/v1",
		"/v1/chat/completions",
		"/chat/completions",
		"/v1",
	} {
		if strings.HasSuffix(endpoint, suffix) {
			endpoint = strings.TrimSuffix(endpoint, suffix)
			break
		}
	}
	return endpoint + "/api/v1/chat"
}
