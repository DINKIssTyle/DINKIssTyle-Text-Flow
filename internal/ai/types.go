package ai

import (
	"runtime"
	"strings"
)

const (
	ProviderOpenAI            = "openai"
	ProviderLMStudio          = "lmstudio"
	ProviderAppleIntelligence = "apple_intelligence"

	DefaultEndpoint    = "http://localhost:1234"
	DefaultTemperature = 0.0
)

var defaultMacPasteReplacementBundleIDs = []string{
	"com.apple.iWork.Keynote",
	"com.apple.iWork.Pages",
	"com.apple.iWork.Numbers",
}

type Settings struct {
	Enabled                   bool     `json:"enabled"`
	Provider                  string   `json:"provider"`
	Endpoint                  string   `json:"endpoint"`
	Model                     string   `json:"model"`
	APIKey                    string   `json:"apiKey"`
	Temperature               float64  `json:"temperature"`
	Hotkey                    string   `json:"hotkey"`
	UseSelectedText           bool     `json:"useSelectedText"`
	UseSelectedFile           bool     `json:"useSelectedFile"`
	ReplaceSelectedText       bool     `json:"replaceSelectedText"`
	PasteReplacementBundleIDs []string `json:"pasteReplacementBundleIds"`

	// TTS Settings
	TTSEnabled       bool    `json:"ttsEnabled"`
	TTSEngine        string  `json:"ttsEngine"`        // "os" or "supertonic3"
	TTSEndpoint      string  `json:"ttsEndpoint"`      // e.g., "http://localhost:7788"
	TTSVoice         string  `json:"ttsVoice"`         // e.g., "default" or "M1", "F1", etc.
	TTSOSVoice       string  `json:"ttsOsVoice"`       // OS-specific persistent voice identifier
	TTSUseAIResponse bool    `json:"ttsUseAiResponse"` // Speak on AI response
	TTSUseShortcut   bool    `json:"ttsUseShortcut"`   // Speak on shortcut hotkey
	TTSShortcut      string  `json:"ttsShortcut"`      // Hotkey for TTS reading
	TTSSpeed         float64 `json:"ttsSpeed"`         // Speech speed factor (0.7 - 2.0)
	TTSSteps         int     `json:"ttsSteps"`         // Number of denoising steps (5 - 12)
}

type ContextKind string

const (
	ContextNone         ContextKind = "none"
	ContextSelectedText ContextKind = "selected_text"
	ContextSelectedFile ContextKind = "selected_file"
)

type InvocationContext struct {
	Kind            ContextKind `json:"kind"`
	Text            string      `json:"text"`
	FilePath        string      `json:"filePath"`
	Label           string      `json:"label"`
	SourceProcessID int         `json:"sourceProcessId"`
	AppName         string      `json:"appName"`
	AppBundleID     string      `json:"appBundleId"`
	IsEditable      bool        `json:"isEditable"`
}

type AssistRequest struct {
	Instruction  string      `json:"instruction"`
	ContextKind  ContextKind `json:"contextKind"`
	ContextText  string      `json:"contextText"`
	FilePath     string      `json:"filePath"`
	AppName      string      `json:"appName"`
	AppBundleID  string      `json:"appBundleId"`
	CustomPrompt string      `json:"customPrompt"`
	CanReplace   bool        `json:"canReplace"`
}

type AssistResult struct {
	Intent        string `json:"intent"`
	SupportReport string `json:"supportReport"`
	Replacement   string `json:"replacement"`
}

func DefaultSettings() Settings {
	pasteReplacementBundleIDs := []string{}
	if runtime.GOOS == "darwin" {
		pasteReplacementBundleIDs = append(pasteReplacementBundleIDs, defaultMacPasteReplacementBundleIDs...)
	}
	return Settings{
		Enabled:                   false,
		Provider:                  ProviderOpenAI,
		Endpoint:                  DefaultEndpoint,
		Temperature:               DefaultTemperature,
		Hotkey:                    "",
		UseSelectedText:           true,
		UseSelectedFile:           false,
		ReplaceSelectedText:       true,
		PasteReplacementBundleIDs: pasteReplacementBundleIDs,

		// TTS Defaults
		TTSEnabled:       false,
		TTSEngine:        "os",
		TTSEndpoint:      "http://localhost:7788",
		TTSVoice:         "M1",
		TTSOSVoice:       "",
		TTSUseAIResponse: false,
		TTSUseShortcut:   false,
		TTSShortcut:      "",
		TTSSpeed:         1.05,
		TTSSteps:         8,
	}
}

func NormalizeSettings(settings Settings) Settings {
	defaults := DefaultSettings()

	settings.Provider = strings.TrimSpace(settings.Provider)
	if settings.Provider == "" {
		settings.Provider = defaults.Provider
	}
	if settings.Provider != ProviderOpenAI &&
		settings.Provider != ProviderLMStudio &&
		settings.Provider != ProviderAppleIntelligence {
		settings.Provider = ProviderOpenAI
	}
	if runtime.GOOS != "darwin" && settings.Provider == ProviderAppleIntelligence {
		settings.Provider = ProviderOpenAI
	}

	settings.Endpoint = strings.TrimSpace(settings.Endpoint)
	if settings.Endpoint == "" {
		settings.Endpoint = defaults.Endpoint
	}

	settings.Model = strings.TrimSpace(settings.Model)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.Hotkey = strings.TrimSpace(settings.Hotkey)

	if settings.Temperature < 0 {
		settings.Temperature = 0
	}
	if settings.Temperature > 2 {
		settings.Temperature = 2
	}

	if settings.PasteReplacementBundleIDs == nil {
		settings.PasteReplacementBundleIDs = append(
			[]string{},
			defaults.PasteReplacementBundleIDs...,
		)
	} else {
		seen := map[string]bool{}
		bundleIDs := make([]string, 0, len(settings.PasteReplacementBundleIDs))
		for _, bundleID := range settings.PasteReplacementBundleIDs {
			bundleID = strings.TrimSpace(bundleID)
			if bundleID == "" || seen[bundleID] {
				continue
			}
			seen[bundleID] = true
			bundleIDs = append(bundleIDs, bundleID)
		}
		settings.PasteReplacementBundleIDs = bundleIDs
	}

	settings.TTSEngine = strings.TrimSpace(settings.TTSEngine)
	if settings.TTSEngine == "" {
		settings.TTSEngine = "os"
	}
	if settings.TTSEngine != "os" && settings.TTSEngine != "supertonic3" {
		settings.TTSEngine = "os"
	}

	settings.TTSEndpoint = strings.TrimSpace(settings.TTSEndpoint)
	if settings.TTSEndpoint == "" {
		settings.TTSEndpoint = "http://localhost:7788"
	}

	settings.TTSVoice = strings.TrimSpace(settings.TTSVoice)
	if settings.TTSVoice == "" || settings.TTSVoice == "default" {
		settings.TTSVoice = "M1"
	}
	settings.TTSOSVoice = strings.TrimSpace(settings.TTSOSVoice)
	if runtime.GOOS == "windows" {
		switch strings.ToUpper(settings.TTSOSVoice) {
		case "TTS_MS_KO-KR_HEAMI_11.0":
			settings.TTSOSVoice = `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Speech_OneCore\Voices\Tokens\MSTTS_V110_koKR_HeamiM`
		case "TTS_MS_EN-US_DAVID_11.0":
			settings.TTSOSVoice = `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Speech_OneCore\Voices\Tokens\MSTTS_V110_enUS_DavidM`
		case "TTS_MS_EN-US_ZIRA_11.0":
			settings.TTSOSVoice = `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Speech_OneCore\Voices\Tokens\MSTTS_V110_enUS_ZiraM`
		}
	}

	settings.TTSShortcut = strings.TrimSpace(settings.TTSShortcut)

	if settings.TTSSpeed <= 0 {
		settings.TTSSpeed = 1.05
	}
	if settings.TTSSteps <= 0 {
		settings.TTSSteps = 8
	}

	return settings
}
