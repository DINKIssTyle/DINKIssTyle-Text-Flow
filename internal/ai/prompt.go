package ai

import (
	"regexp"
	"strings"
)

const (
	IntentEdit      = "edit"
	IntentQuestion  = "question"
	IntentAmbiguous = "ambiguous"
)

// --- Pre-compiled Regex ---
var fencedBlockPattern = regexp.MustCompile(`(?is)^` + "```" + `[a-z0-9_-]*\s*\n([\s\S]*?)\n` + "```" + `$`)

// --- Prompt Definitions ---

var commonSystemPromptLines = []string{
	"You operate as a completely stateless agent. You do not retain memory of previous interactions or understand continuous context.",
	"Treat every request as an isolated, independent task.",
	"You have no access to the internet, real-time data, the current time, or geographic location. Do not attempt to guess this information.",
	"You must respond in the same language as the user.",
	"Privately reason through intent before answering, but never output your reasoning or chain of thought.",
}

var editIntentDecisionLines = []string{
	"## Intent decision protocol",
	"Classify by the requested outcome and its destination, not by keywords, grammar, politeness, or the language in which the instruction is written.",
	"Before choosing an intent, privately perform this decision audit in order. Do not reveal the audit or any reasoning:",
	"A. State the concrete outcome the instruction asks the assistant to produce.",
	"B. Determine the intended destination of that outcome: the AI window for the user to read, or the current document/app for insertion or replacement.",
	"C. Check whether the assistant is being asked to exercise judgment (such as evaluating possibilities or reaching a decision) and merely report the result to the user.",
	"D. Check separately for explicit evidence that the user wants the selected text changed or wants new content inserted into the current app.",
	"E. Resolve conflicts by giving explicit destination evidence priority. A requested judgment without explicit document-editing evidence belongs in the AI window.",
	"F. Only after completing A-E, emit the intent and final response in the required XML format.",
	"1. **Conversation outcome**: Select <intent>question</intent> when the requested result is an answer to be shown to the user in the AI window. This includes asking the assistant to evaluate alternatives, make a choice or decision, recommend an option, state a preference, explain, identify, compare, judge, or report a conclusion.",
	"2. **Document outcome**: Select <intent>edit</intent> only when the requested result is content meant to be inserted into the current app or a modification of the selected text itself. This includes creating an insertion-ready artifact and transforming, correcting, translating, formatting, or replacing document text.",
	"3. **Destination overrides surface form**: A conversational request can produce usable text and still be <intent>question</intent>. A request phrased as a question can still be <intent>edit</intent> when its actual goal is document insertion or transformation.",
	"4. **Decision precedence**: A request to choose, decide, recommend, rank, or select among possibilities is <intent>question</intent> unless the instruction explicitly asks to write that decision into the document or replace selected text with it.",
	"5. **Selected text is context, not an automatic edit target**: Referring to or asking about selected text does not imply permission to replace it.",
	"6. **Ambiguity resolution**: Infer the destination from the whole instruction. Use <intent>ambiguous</intent> only when there is no reasonable evidence that the result belongs either in the AI window or in the current document.",
}

// --- Functions ---

func BuildSystemPrompt(hasContext bool, customPrompt string) string {
	customPrompt = strings.TrimSpace(customPrompt)
	lines := append([]string{}, commonSystemPromptLines...)

	// Role & Context Injection (App-specific grounding)
	lines = append(lines, "You are the core AI engine for DKST Text Flow, an advanced macOS text expansion utility.")

	if hasContext {
		lines = append(lines,
			"Decide intent using only the user's <instruction>. Do not let the content, topic, tone, or language of <selected_text> affect intent classification.",
			"Use <selected_text> only as the target text to edit or as evidence when answering a question.",
		)
		lines = append(lines, editIntentDecisionLines...)
		lines = append(lines,
			"## OUTPUT FORMAT",
			"Return exactly THREE XML blocks in this strict order: <intent>...</intent><support_report>...</support_report><replacement>...</replacement>.",
			"Do not output any <reasoning> block or private analysis.",
			"<support_report> is shown directly to the user as a status or answer. Do NOT describe, summarize, or classify the user's request here.",
			"If intent is edit, <replacement> MUST contain ONLY the final replacement text. NEVER put the edited, translated, or converted text in <support_report>.",
			"When editing <selected_text>, preserve the original line breaks and line count in <replacement> unless explicitly asked to reformat or summarize.",
			"If <selected_text> has multiple lines, edit each line in place and keep each original line as a corresponding line in <replacement>.",
			"If intent is question or ambiguous, leave <replacement></replacement> empty and put the direct final answer in <support_report>.",
			"When answering a question about selected text, use the text as context instead of saying what the user is asking.",
			"Do NOT use markdown code fences (```) inside <replacement>.",
		)
	} else {
		lines = append(lines,
			"There is no selected text or selected file.",
			"Answer simple questions or draft new text for the user.",
		)
		lines = append(lines, editIntentDecisionLines...)
		lines = append(lines,
			"## OUTPUT FORMAT",
			"Return exactly THREE XML blocks in this strict order: <intent>...</intent><support_report>...</support_report><replacement>...</replacement>.",
			"Do not output any <reasoning> block or private analysis.",
			"Use <intent>edit</intent> when the user asks you to input, write, draft, generate, or create text that should be inserted at the cursor.",
			"If intent is edit, put ONLY the text to insert in <replacement> and keep <support_report> to a very short status sentence.",
			"If intent is question, leave <replacement></replacement> empty, and put the direct final answer in <support_report>.",
			"Do NOT use markdown code fences (```) inside <replacement>.",
		)
	}

	return appendCustomPrompt(lines, customPrompt)
}

func BuildUserPrompt(request AssistRequest) string {
	sections := []string{
		"## Instruction",
		wrapTag("instruction", request.Instruction),
	}

	switch request.ContextKind {
	case ContextSelectedText:
		sections = append([]string{
			"## Context",
			wrapTag("selected_text", request.ContextText),
		}, sections...)
	case ContextSelectedFile:
		sections = append([]string{
			"## Context",
			wrapTag("selected_file_path", request.FilePath),
			wrapTag("selected_file", request.ContextText),
		}, sections...)
	}

	return strings.Join(sections, "\n\n")
}

func appendCustomPrompt(lines []string, customPrompt string) string {
	if customPrompt != "" {
		lines = append(lines,
			"## ADDITIONAL INSTRUCTIONS",
			"The following instructions take precedence. Treat them as behavioral guidance for this invocation.",
			"If those instructions describe text to enter into the current app, return that text in <replacement>.",
			customPrompt,
		)
	}
	return strings.Join(lines, "\n")
}

func ParseAssistResult(rawText string) AssistResult {
	source := strings.TrimSpace(rawText)
	intent := normalizeIntent(extractTag(source, "intent"))
	supportReport := strings.TrimSpace(extractTag(source, "support_report"))
	replacement := extractTag(source, "replacement")
	supportReportFromTag := supportReport != ""

	result := AssistResult{
		Intent:        intent,
		SupportReport: supportReport,
		Replacement:   replacement,
	}

	if result.Intent == "" {
		result.Intent = IntentQuestion
	}

	if result.Intent == IntentEdit && strings.TrimSpace(result.Replacement) == "" {
		if repaired := recoverMalformedEditReplacement(source); repaired != "" {
			result.Replacement = repaired
			result.SupportReport = ""
		}
	}

	result.Replacement = stripCodeFence(result.Replacement)

	if result.Intent == IntentEdit && strings.TrimSpace(result.Replacement) == "" && supportReportFromTag && result.SupportReport != "" {
		result.Replacement = result.SupportReport
		result.SupportReport = ""
	}

	if result.SupportReport == "" && strings.TrimSpace(result.Replacement) == "" {
		if containsKnownXMLTag(source) {
			result.SupportReport = "AI response format was invalid. Please try again."
		} else {
			result.SupportReport = source
		}
	}

	return result
}

func wrapTag(tagName string, content string) string {
	return "<" + tagName + ">\n" + content + "\n</" + tagName + ">"
}

// Optimized tag extraction using strings.Index instead of RegEx compilation
func extractTag(source string, tagName string) string {
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"

	startIdx := strings.Index(source, openTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(openTag)

	endIdx := strings.Index(source[startIdx:], closeTag)
	if endIdx == -1 {
		return ""
	}

	return strings.TrimSpace(source[startIdx : startIdx+endIdx])
}

func recoverMalformedEditReplacement(source string) string {
	if replacement := extractOpenEndedTag(source, "replacement"); replacement != "" {
		return stripKnownXMLTags(replacement)
	}

	replacementClose := "</replacement>"
	closeIdx := strings.LastIndex(source, replacementClose)
	if closeIdx == -1 {
		return ""
	}

	tail := strings.TrimSpace(source[closeIdx+len(replacementClose):])
	if tail != "" {
		return stripKnownXMLTags(tail)
	}

	supportOpen := "<support_report>"
	supportIdx := strings.Index(source, supportOpen)
	if supportIdx != -1 && supportIdx < closeIdx {
		replacement := strings.TrimSpace(source[supportIdx+len(supportOpen) : closeIdx])
		return stripKnownXMLTags(replacement)
	}
	return ""
}

func extractOpenEndedTag(source string, tagName string) string {
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"

	startIdx := strings.Index(source, openTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(openTag)
	if strings.Index(source[startIdx:], closeTag) != -1 {
		return ""
	}
	return strings.TrimSpace(source[startIdx:])
}

func containsKnownXMLTag(source string) bool {
	knownTags := []string{
		"<reasoning>", "</reasoning>",
		"<intent>", "</intent>",
		"<support_report>", "</support_report>",
		"<replacement>", "</replacement>",
	}
	for _, tag := range knownTags {
		if strings.Contains(source, tag) {
			return true
		}
	}
	return false
}

func stripKnownXMLTags(value string) string {
	replacer := strings.NewReplacer(
		"<reasoning>", "", "</reasoning>", "",
		"<intent>", "", "</intent>", "",
		"<support_report>", "", "</support_report>", "",
		"<replacement>", "", "</replacement>", "",
	)
	return strings.TrimSpace(replacer.Replace(value))
}

func normalizeIntent(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case IntentEdit:
		return IntentEdit
	case IntentQuestion:
		return IntentQuestion
	case IntentAmbiguous:
		return IntentAmbiguous
	default:
		return ""
	}
}

func stripCodeFence(value string) string {
	value = strings.TrimSpace(value)
	match := fencedBlockPattern.FindStringSubmatch(value)
	if len(match) == 2 {
		return match[1]
	}
	return value
}
