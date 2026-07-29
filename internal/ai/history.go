package ai

import (
	"strconv"
	"strings"
	"sync"
)

type conversationTurn struct {
	User      string
	Assistant string
}

// ConversationHistory keeps AI context in memory for the current app session.
// OpenAI-compatible providers replay recent turns. LM Studio keeps only the
// response ID needed to continue its server-managed stateful chat.
type ConversationHistory struct {
	mu sync.Mutex

	signature    string
	requestScope string

	openAITurns []conversationTurn

	lmStudioResponseID   string
	lmStudioSystemPrompt string
	lmStudioTurnCount    int
}

func NewConversationHistory() *ConversationHistory {
	return &ConversationHistory{}
}

func (history *ConversationHistory) Reset() {
	if history == nil {
		return
	}
	history.mu.Lock()
	defer history.mu.Unlock()
	history.resetLocked()
}

func (history *ConversationHistory) prepareLocked(settings Settings) {
	signature := conversationSignature(settings)
	if history.signature != signature {
		history.resetLocked()
		history.signature = signature
	}
}

// prepareRequestLocked prevents app-specific instructions and answers from one
// application from being replayed as conversation history in another
// application. The custom prompt is part of the scope because an app may use
// different rules for selected-text and no-selection requests.
func (history *ConversationHistory) prepareRequestLocked(request AssistRequest) {
	scope := conversationRequestScope(request)
	if history.requestScope != "" && history.requestScope != scope {
		history.resetTurnsLocked()
	}
	history.requestScope = scope
}

func (history *ConversationHistory) resetLocked() {
	history.signature = ""
	history.requestScope = ""
	history.resetTurnsLocked()
}

func (history *ConversationHistory) resetTurnsLocked() {
	history.openAITurns = nil
	history.lmStudioResponseID = ""
	history.lmStudioSystemPrompt = ""
	history.lmStudioTurnCount = 0
}

func (history *ConversationHistory) openAIMessagesLocked(limit int) []chatMessage {
	start := len(history.openAITurns) - limit
	if start < 0 {
		start = 0
	}
	messages := make([]chatMessage, 0, (len(history.openAITurns)-start)*2)
	for _, turn := range history.openAITurns[start:] {
		messages = append(messages,
			chatMessage{Role: "user", Content: turn.User},
			chatMessage{Role: "assistant", Content: turn.Assistant},
		)
	}
	return messages
}

func (history *ConversationHistory) appendOpenAITurnLocked(user, assistant string, limit int) {
	history.openAITurns = append(history.openAITurns, conversationTurn{
		User:      user,
		Assistant: assistant,
	})
	if overflow := len(history.openAITurns) - limit; overflow > 0 {
		history.openAITurns = append([]conversationTurn(nil), history.openAITurns[overflow:]...)
	}
}

func (history *ConversationHistory) prepareLMStudioTurnLocked(
	limit int,
	systemPrompt string,
) string {
	systemPrompt = strings.TrimSpace(systemPrompt)
	if history.lmStudioTurnCount >= limit ||
		(history.lmStudioSystemPrompt != "" &&
			history.lmStudioSystemPrompt != systemPrompt) {
		history.lmStudioResponseID = ""
		history.lmStudioSystemPrompt = ""
		history.lmStudioTurnCount = 0
	}
	if history.lmStudioResponseID == "" {
		history.lmStudioSystemPrompt = systemPrompt
	}
	return history.lmStudioResponseID
}

func (history *ConversationHistory) commitLMStudioTurnLocked(responseID string) {
	history.lmStudioResponseID = strings.TrimSpace(responseID)
	history.lmStudioTurnCount++
}

func conversationSignature(settings Settings) string {
	return strings.Join([]string{
		settings.Provider,
		strings.TrimSpace(settings.Endpoint),
		strings.TrimSpace(settings.Model),
		strconv.Itoa(settings.HistoryCount),
	}, "\x00")
}

func conversationRequestScope(request AssistRequest) string {
	appID := strings.ToLower(strings.TrimSpace(request.AppBundleID))
	if appID == "" {
		appID = strings.ToLower(strings.TrimSpace(request.AppName))
	}
	return strings.Join([]string{
		appID,
		strings.TrimSpace(request.CustomPrompt),
	}, "\x00")
}
