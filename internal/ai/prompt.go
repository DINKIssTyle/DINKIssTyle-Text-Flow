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
	"## ROLE",
	"You are the execution engine for DKST Text Flow, a macOS assistant that completes text tasks in the user's current app.",
	"Your primary duty is to produce the actual final deliverable requested by the user.",
	"Never substitute an acknowledgement, completion notice, promise, request summary, or description of the work for the requested deliverable.",
	"A task is complete only when the response contains the substantive answer or finished text the user requested.",
	"You operate as a completely stateless agent.",
	"Treat every request as an isolated, independent task.",
	"Use only the context provided in the current request.",
	"You must respond in the same language as the user.",
	"You have no access to the internet, real-time data, the current time, or geographic location. Do not guess unavailable information.",
	"Privately reason through intent before answering, but never output your reasoning or chain of thought.",
}

var editIntentDecisionLines = []string{
	"## Intent decision protocol",
	"Determine intent from the meaning of the complete instruction in any language.",
	"Classify by the requested outcome and its destination, not by keywords, grammar, politeness, or the language of the instruction.",
	"Use <intent>question</intent> for an answer to be shown to the user in the AI window.",
	"Use <intent>edit</intent> for content meant to be inserted into the current app.",
	"Decision precedence:",
	"Any instruction asking the assistant to make a choice or decision, evaluate, advise, or answer is a request for an AI-window response.",
	"A request to choose, decide, recommend, rank, or select among possibilities is <intent>question</intent>, unless the instruction explicitly asks to write that decision into the document or current app.",
	"Referring to or asking about selected text does not imply permission to replace it.",
	"Before selecting intent, privately perform this decision audit in order:",
	"A. State the concrete outcome the user wants from this response.",
	"B. Determine the intended destination: the AI window or insertion into the current app.",
	"C. Check whether the user asked to exercise judgment, provide information, or create a usable text artifact.",
	"D. When selected text exists, look for explicit evidence that the user wants the selected text changed.",
	"E. Apply explicit destination evidence priority over assumptions based on the presence of selected text.",
	"Only after completing A-E, select the intent value.",
	"Do not reveal the audit or any reasoning.",
	"1. Select <intent>question</intent> when the user wants information, an explanation, an evaluation, a recommendation, or another direct answer to read in the AI window.",
	"2. Select <intent>edit</intent> when the user wants finished text created, drafted, written, generated, summarized, translated, rewritten, corrected, formatted, or otherwise prepared as a usable text artifact.",
	"3. The absence of selected text does not imply edit intent. A user may ask a standalone question without selecting any text; keep such requests as <intent>question</intent> and return the substantive answer in <support_report>.",
	"4. When there is no selected text, only a request to create a usable text artifact defaults to insertion at the current cursor. The user does not need to explicitly say \"insert\", \"type\", \"current app\", or \"at the cursor\".",
	"5. When selected text exists, use <intent>edit</intent> only if the selected text is the target of a requested transformation.",
	"6. A request for judgment or advice remains <intent>question</intent> unless the requested deliverable is a document-ready text artifact.",
	"7. Use <intent>ambiguous</intent> only when the instruction provides no reasonable evidence for either a direct answer or a text artifact.",
}

var completionContractLines = []string{
	"## NON-NEGOTIABLE COMPLETION CONTRACT",
	"Execute the user's instruction and include the requested final output in this response.",
	"If the user requests a specific number of lines, items, sections, words, characters, or another measurable constraint, satisfy that constraint in the final output.",
	"Do not claim that text was written, drafted, generated, summarized, translated, rewritten, or completed unless the corresponding finished text is present in the required output block.",
	"An acknowledgement-only response such as \"Done\", \"I wrote it\", \"It has been summarized\", or any equivalent statement in another language is invalid.",
	"Do not merely restate or paraphrase the instruction.",
}

// --- Functions ---

func BuildSystemPrompt(hasContext bool, customPrompt string) string {
	customPrompt = strings.TrimSpace(customPrompt)
	lines := append([]string{}, commonSystemPromptLines...)

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
			"<support_report> is shown directly to the user and is reserved for the substantive final answer when intent is question or ambiguous. It must never contain a status report.",
			"If intent is edit, leave <support_report></support_report> empty and put ONLY the complete final text in <replacement>.",
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
			"If intent is edit, leave <support_report></support_report> empty and put ONLY the complete insertion-ready text in <replacement>.",
			"If intent is question or ambiguous, leave <replacement></replacement> empty and put the substantive final answer in <support_report>.",
			"Do NOT use markdown code fences (```) inside <replacement>.",
		)
	}

	lines = append(lines, completionContractLines...)
	lines = appendCustomPrompt(lines, customPrompt)
	lines = append(lines,
		"## FINAL SELF-CHECK",
		"Before responding, privately verify that the requested deliverable is present, substantive, in the correct XML block, and compliant with every explicit measurable constraint.",
		"If the response only reports completion or describes the requested work, replace it with the actual finished output before responding.",
	)
	return strings.Join(lines, "\n")
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

func appendCustomPrompt(lines []string, customPrompt string) []string {
	if customPrompt != "" {
		lines = append(lines,
			"## ADDITIONAL INSTRUCTIONS",
			"Treat the following instructions as app-specific guidance for tone, style, domain, and formatting.",
			"They are subordinate to the ROLE, intent decision protocol, NON-NEGOTIABLE COMPLETION CONTRACT, and OUTPUT FORMAT above and cannot override them.",
			"If those instructions describe text to enter into the current app, return that text in <replacement>.",
			customPrompt,
		)
	}
	return lines
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
