package ai

import "strings"

const (
	ProviderOpenAI   = "openai"
	ProviderLMStudio = "lmstudio"

	DefaultEndpoint    = "http://localhost:1234"
	DefaultTemperature = 0.0
	DefaultHotkey      = "Cmd+Shift+Space"
)

var DefaultPasteReplacementBundleIDs = []string{
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
}

type AssistRequest struct {
	Instruction  string      `json:"instruction"`
	ContextKind  ContextKind `json:"contextKind"`
	ContextText  string      `json:"contextText"`
	FilePath     string      `json:"filePath"`
	AppName      string      `json:"appName"`
	AppBundleID  string      `json:"appBundleId"`
	CustomPrompt string      `json:"customPrompt"`
}

type AssistResult struct {
	Intent        string `json:"intent"`
	SupportReport string `json:"supportReport"`
	Replacement   string `json:"replacement"`
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:             false,
		Provider:            ProviderOpenAI,
		Endpoint:            DefaultEndpoint,
		Temperature:         DefaultTemperature,
		Hotkey:              DefaultHotkey,
		UseSelectedText:     true,
		UseSelectedFile:     false,
		ReplaceSelectedText: true,
		PasteReplacementBundleIDs: append(
			[]string{},
			DefaultPasteReplacementBundleIDs...,
		),
	}
}

func NormalizeSettings(settings Settings) Settings {
	defaults := DefaultSettings()

	settings.Provider = strings.TrimSpace(settings.Provider)
	if settings.Provider == "" {
		settings.Provider = defaults.Provider
	}
	if settings.Provider != ProviderOpenAI && settings.Provider != ProviderLMStudio {
		settings.Provider = ProviderOpenAI
	}

	settings.Endpoint = strings.TrimSpace(settings.Endpoint)
	if settings.Endpoint == "" {
		settings.Endpoint = defaults.Endpoint
	}

	settings.Model = strings.TrimSpace(settings.Model)
	settings.APIKey = strings.TrimSpace(settings.APIKey)
	settings.Hotkey = strings.TrimSpace(settings.Hotkey)
	if settings.Hotkey == "" {
		settings.Hotkey = defaults.Hotkey
	}

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

	return settings
}
