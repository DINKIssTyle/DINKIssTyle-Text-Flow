package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"dkst-text-flow/internal/ai"
	"dkst-text-flow/internal/flowengine"
	"dkst-text-flow/internal/hotkey"
	"dkst-text-flow/internal/loginitem"
	"dkst-text-flow/internal/platform"
	"dkst-text-flow/internal/statusitem"
	"dkst-text-flow/internal/storage"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	aiSettingsKey       = "ai.settings"
	aiPromptSettingsKey = "ai.prompt.settings"
	generalSettingsKey  = "general.settings"
)

const (
	ThemeAuto  = "auto"
	ThemeLight = "light"
	ThemeDark  = "dark"
)

const (
	LanguageEnglish = "en"
	LanguageKorean  = "ko"
)

type GeneralSettings struct {
	ThemeMode          string `json:"themeMode"`
	Language           string `json:"language"`
	TypingTrendEnabled bool   `json:"typingTrendEnabled"`
	StartAtLogin       bool   `json:"startAtLogin"`
	SoundName          string `json:"soundName"`
}

type AIPromptRule struct {
	UseSelectedText     bool   `json:"useSelectedText"`
	RunWithoutSelection bool   `json:"runWithoutSelection"`
	SelectedTextPrompt  string `json:"selectedTextPrompt"`
	NoSelectionPrompt   string `json:"noSelectionPrompt"`
}

type AIPromptProfile struct {
	ID          string `json:"id"`
	AppName     string `json:"appName"`
	AppBundleID string `json:"appBundleId"`
	AIPromptRule
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type AIPromptSettings struct {
	Common   AIPromptRule      `json:"common"`
	Profiles []AIPromptProfile `json:"profiles"`
}

type AIPromptProfileInput struct {
	AppName     string `json:"appName"`
	AppBundleID string `json:"appBundleId"`
	AIPromptRule
}

// App struct
type App struct {
	ctx                   context.Context
	store                 *storage.Store
	aiClient              *ai.Client
	expansionSoundEvents  chan struct{}
	expansionSoundStopper context.CancelFunc
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		aiClient:             ai.NewClient(),
		expansionSoundEvents: make(chan struct{}, 8),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	store, err := storage.OpenDefault()
	if err != nil {
		println("failed to open storage:", err.Error())
		return
	}
	a.store = store
	a.startExpansionSoundDispatcher(ctx)
	a.configureExpansionSoundEvent()
	flowengine.Start(a.store)
	a.configureAIHotkey()
}

func (a *App) domReady(ctx context.Context) {
	statusitem.Install(ctx)
	a.configureExpansionSoundEvent()
	a.configureAIHotkey()
}

func (a *App) shutdown(ctx context.Context) {
	hotkey.Unregister()
	flowengine.SetExpansionHandler(nil)
	if a.expansionSoundStopper != nil {
		a.expansionSoundStopper()
		a.expansionSoundStopper = nil
	}
	flowengine.Stop()
	if a.store == nil {
		return
	}
	if err := a.store.Close(); err != nil {
		println("failed to close storage:", err.Error())
	}
}

func (a *App) ListSnippets(query string) ([]storage.Snippet, error) {
	return a.store.ListSnippets(query)
}

func (a *App) ListSnippetsByLabel(query string, labelID int64) ([]storage.Snippet, error) {
	return a.store.ListSnippetsByLabel(query, labelID)
}

func (a *App) CreateSnippet(input storage.SnippetInput) (storage.Snippet, error) {
	return a.store.CreateSnippet(input)
}

func (a *App) UpdateSnippet(id int64, input storage.SnippetInput) (storage.Snippet, error) {
	return a.store.UpdateSnippet(id, input)
}

func (a *App) DeleteSnippet(id int64) error {
	return a.store.DeleteSnippet(id)
}

func (a *App) ConfirmSnippetDeletion(title string) (bool, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled snippet"
	}

	response, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Delete Snippet",
		Message:       fmt.Sprintf("Delete snippet \"%s\"?\nThis action cannot be undone.", title),
		Buttons:       []string{"Delete", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return false, err
	}
	return response == "Delete", nil
}

func (a *App) ToggleSnippet(id int64, enabled bool) (storage.Snippet, error) {
	return a.store.ToggleSnippet(id, enabled)
}

func (a *App) ListLabels() ([]storage.Label, error) {
	return a.store.ListLabels()
}

func (a *App) CreateLabel(input storage.LabelInput) (storage.Label, error) {
	return a.store.CreateLabel(input)
}

func (a *App) UpdateLabel(id int64, input storage.LabelInput) (storage.Label, error) {
	return a.store.UpdateLabel(id, input)
}

func (a *App) DeleteLabel(id int64) error {
	return a.store.DeleteLabel(id)
}

func (a *App) ConfirmLabelDeletion(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Untitled label"
	}

	response, err := wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Delete Label",
		Message:       fmt.Sprintf("Delete label \"%s\"?\nSnippets in this label will move back to All.", name),
		Buttons:       []string{"Delete", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return false, err
	}
	return response == "Delete", nil
}

func (a *App) AssignSnippetLabel(snippetID int64, labelID int64) (storage.Snippet, error) {
	return a.store.AssignSnippetLabel(snippetID, labelID)
}

func (a *App) SetLabelSnippetsEnabled(labelID int64, enabled bool) error {
	return a.store.SetLabelSnippetsEnabled(labelID, enabled)
}

func (a *App) GetDashboard() (storage.DashboardStats, error) {
	return a.store.Dashboard()
}

func (a *App) LogExpansion(snippetID int64, appBundleID string) error {
	return a.store.LogExpansion(snippetID, appBundleID)
}

func (a *App) GetPlatformStatus() platform.Status {
	status := platform.CurrentStatus()
	if a.store != nil && status.AccessibilityTrusted && !flowengine.Running() {
		flowengine.Start(a.store)
	}
	status.FlowEngineRunning = flowengine.Running()
	return status
}

func (a *App) RequestAccessibilityPermission() platform.Status {
	platform.RequestAccessibilityPermission()
	return a.GetPlatformStatus()
}

func (a *App) GetGeneralSettings() (GeneralSettings, error) {
	settings := DefaultGeneralSettings()
	if a.store == nil {
		return settings, nil
	}

	found, err := a.store.GetJSONSetting(generalSettingsKey, &settings)
	if err != nil {
		return GeneralSettings{}, err
	}
	if !found {
		settings.StartAtLogin = loginitem.Enabled()
		return settings, nil
	}
	normalized := NormalizeGeneralSettings(settings)
	normalized.StartAtLogin = loginitem.Enabled()
	return normalized, nil
}

func (a *App) SaveGeneralSettings(settings GeneralSettings) (GeneralSettings, error) {
	if a.store == nil {
		return GeneralSettings{}, errors.New("storage is not ready")
	}

	normalized := NormalizeGeneralSettings(settings)
	if err := loginitem.SetEnabled(normalized.StartAtLogin); err != nil {
		return GeneralSettings{}, err
	}
	if err := a.store.SetJSONSetting(generalSettingsKey, normalized); err != nil {
		return GeneralSettings{}, err
	}
	return normalized, nil
}

func (a *App) GetAIPromptSettings() (AIPromptSettings, error) {
	settings := DefaultAIPromptSettings()
	if a.store == nil {
		return settings, nil
	}

	found, err := a.store.GetJSONSetting(aiPromptSettingsKey, &settings)
	if err != nil {
		return AIPromptSettings{}, err
	}
	if !found {
		return settings, nil
	}
	return NormalizeAIPromptSettings(settings), nil
}

func (a *App) SaveCommonAIPromptRule(rule AIPromptRule) (AIPromptSettings, error) {
	settings, err := a.GetAIPromptSettings()
	if err != nil {
		return AIPromptSettings{}, err
	}
	settings.Common = NormalizeAIPromptRule(rule)
	return a.saveAIPromptSettings(settings)
}

func (a *App) CreateAIPromptProfile(input AIPromptProfileInput) (AIPromptSettings, error) {
	settings, err := a.GetAIPromptSettings()
	if err != nil {
		return AIPromptSettings{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	profile := AIPromptProfile{
		ID:           fmt.Sprintf("profile-%d", time.Now().UnixNano()),
		AppName:      strings.TrimSpace(input.AppName),
		AppBundleID:  strings.TrimSpace(input.AppBundleID),
		AIPromptRule: NormalizeAIPromptRule(input.AIPromptRule),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if profile.AppName == "" {
		profile.AppName = "New App"
	}
	settings.Profiles = append(settings.Profiles, profile)
	return a.saveAIPromptSettings(settings)
}

func (a *App) UpdateAIPromptProfile(id string, input AIPromptProfileInput) (AIPromptSettings, error) {
	settings, err := a.GetAIPromptSettings()
	if err != nil {
		return AIPromptSettings{}, err
	}
	id = strings.TrimSpace(id)
	for index := range settings.Profiles {
		if settings.Profiles[index].ID != id {
			continue
		}
		settings.Profiles[index].AppName = strings.TrimSpace(input.AppName)
		settings.Profiles[index].AppBundleID = strings.TrimSpace(input.AppBundleID)
		settings.Profiles[index].AIPromptRule = NormalizeAIPromptRule(input.AIPromptRule)
		settings.Profiles[index].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if settings.Profiles[index].AppName == "" {
			settings.Profiles[index].AppName = "New App"
		}
		return a.saveAIPromptSettings(settings)
	}
	return AIPromptSettings{}, errors.New("AI prompt profile was not found")
}

func (a *App) DeleteAIPromptProfile(id string) (AIPromptSettings, error) {
	settings, err := a.GetAIPromptSettings()
	if err != nil {
		return AIPromptSettings{}, err
	}
	id = strings.TrimSpace(id)
	nextProfiles := make([]AIPromptProfile, 0, len(settings.Profiles))
	found := false
	for _, profile := range settings.Profiles {
		if profile.ID == id {
			found = true
			continue
		}
		nextProfiles = append(nextProfiles, profile)
	}
	if !found {
		return AIPromptSettings{}, errors.New("AI prompt profile was not found")
	}
	settings.Profiles = nextProfiles
	return a.saveAIPromptSettings(settings)
}

func (a *App) BrowseAIPromptApp() (platform.AppInfo, error) {
	if a.ctx == nil {
		return platform.AppInfo{}, errors.New("application context is not ready")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                      "Choose an application",
		DefaultDirectory:           "/Applications",
		TreatPackagesAsDirectories: false,
	})
	if err != nil {
		return platform.AppInfo{}, err
	}
	path = enclosingAppBundlePath(path)
	if path == "" {
		return platform.AppInfo{}, nil
	}
	return platform.AppInfoFromBundlePath(path), nil
}

func enclosingAppBundlePath(path string) string {
	path = strings.TrimSpace(path)
	for path != "" && path != "." && path != string(filepath.Separator) {
		if strings.EqualFold(filepath.Ext(path), ".app") {
			return path
		}
		next := filepath.Dir(path)
		if next == path {
			break
		}
		path = next
	}
	return ""
}

func (a *App) GetAISettings() (ai.Settings, error) {
	settings := ai.DefaultSettings()
	if a.store == nil {
		return settings, nil
	}

	found, err := a.store.GetJSONSetting(aiSettingsKey, &settings)
	if err != nil {
		return ai.Settings{}, err
	}
	if !found {
		return settings, nil
	}
	return a.normalizeAISettings(settings), nil
}

func (a *App) SaveAISettings(settings ai.Settings) (ai.Settings, error) {
	if a.store == nil {
		return ai.Settings{}, errors.New("storage is not ready")
	}

	normalized := a.normalizeAISettings(settings)
	if err := a.store.SetJSONSetting(aiSettingsKey, normalized); err != nil {
		return ai.Settings{}, err
	}
	a.configureAIHotkeyWithSettings(normalized)
	return normalized, nil
}

func (a *App) MakeAIRequest(endpoint string, headers map[string]string, body string) (string, error) {
	if a.aiClient == nil {
		a.aiClient = ai.NewClient()
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", errors.New("AI endpoint is required")
	}
	return a.aiClient.MakeRequest(endpoint, headers, body)
}

func (a *App) RunAIAssist(input ai.AssistRequest) (ai.AssistResult, error) {
	settings, err := a.GetAISettings()
	if err != nil {
		return ai.AssistResult{}, err
	}
	if a.aiClient == nil {
		a.aiClient = ai.NewClient()
	}
	input.CustomPrompt = a.customPromptForRequest(input)
	return ai.RunAssist(a.aiClient, settings, input)
}

func (a *App) ReplaceSelectedText(processID int, replacement string) error {
	if processID <= 0 {
		return errors.New("source process is missing")
	}
	if strings.TrimSpace(replacement) == "" {
		return errors.New("replacement text is empty")
	}
	preferPaste := false
	if settings, err := a.GetAISettings(); err == nil {
		bundleID := strings.TrimSpace(platform.AppInfoFromProcess(processID).BundleID)
		for _, pasteBundleID := range settings.PasteReplacementBundleIDs {
			if bundleID != "" && bundleID == strings.TrimSpace(pasteBundleID) {
				preferPaste = true
				break
			}
		}
	}
	return platform.ReplaceSelectedTextInProcess(processID, replacement, preferPaste)
}

func (a *App) ActivateProcess(processID int) error {
	return platform.ActivateProcess(processID)
}

func (a *App) CancelAIRequest() {
	if a.aiClient != nil {
		a.aiClient.Cancel()
	}
}

func (a *App) startExpansionSoundDispatcher(ctx context.Context) {
	if a.expansionSoundStopper != nil {
		return
	}
	dispatchCtx, stop := context.WithCancel(ctx)
	a.expansionSoundStopper = stop
	go func() {
		for {
			select {
			case <-dispatchCtx.Done():
				return
			case <-a.expansionSoundEvents:
				if a.ctx != nil {
					wailsruntime.EventsEmit(a.ctx, "snippet:expanded")
				}
			}
		}
	}()
}

func (a *App) configureExpansionSoundEvent() {
	flowengine.SetExpansionHandler(func(snippet storage.Snippet) {
		if a.expansionSoundEvents == nil {
			return
		}
		select {
		case a.expansionSoundEvents <- struct{}{}:
		default:
		}
	})
}

func (a *App) configureAIHotkey() {
	settings, err := a.GetAISettings()
	if err != nil {
		println("failed to load AI hotkey settings:", err.Error())
		return
	}
	a.configureAIHotkeyWithSettings(settings)
}

func (a *App) configureAIHotkeyWithSettings(settings ai.Settings) {
	if !settings.Enabled {
		hotkey.Unregister()
		return
	}
	if err := hotkey.Register(settings.Hotkey, a.handleAIHotkey); err != nil {
		println("failed to register AI hotkey:", err.Error())
	}
}

func (a *App) handleAIHotkey(sourceProcessID int) {
	if a.ctx == nil {
		return
	}

	settings, err := a.GetAISettings()
	if err != nil || !settings.Enabled {
		return
	}

	invocation := ai.InvocationContext{
		Kind:            ai.ContextNone,
		Label:           "No Context",
		SourceProcessID: sourceProcessID,
	}
	appInfo := platform.AppInfoFromProcess(sourceProcessID)
	invocation.AppName = appInfo.Name
	invocation.AppBundleID = appInfo.BundleID
	rule := a.aiPromptRuleForBundleID(appInfo.BundleID)
	if settings.UseSelectedText && rule.UseSelectedText {
		if selected, err := platform.SelectedTextFromProcess(sourceProcessID); err == nil && strings.TrimSpace(selected) != "" {
			invocation.Kind = ai.ContextSelectedText
			invocation.Text = selected
			invocation.Label = "Selected Text"
		}
	}
	if invocation.Kind == ai.ContextNone && !rule.RunWithoutSelection {
		return
	}

	wailsruntime.WindowSetMinSize(a.ctx, 460, 152)
	wailsruntime.WindowSetSize(a.ctx, 460, 152)
	wailsruntime.WindowCenter(a.ctx)
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.EventsEmit(a.ctx, "ai:invoke", invocation)
}

func (a *App) normalizeAISettings(settings ai.Settings) ai.Settings {
	normalized := ai.NormalizeSettings(settings)
	if parsed, err := hotkey.Parse(normalized.Hotkey); err == nil {
		normalized.Hotkey = parsed.Canonical
	} else {
		normalized.Hotkey = ai.DefaultHotkey
	}
	return normalized
}

func (a *App) saveAIPromptSettings(settings AIPromptSettings) (AIPromptSettings, error) {
	if a.store == nil {
		return AIPromptSettings{}, errors.New("storage is not ready")
	}
	normalized := NormalizeAIPromptSettings(settings)
	if err := a.store.SetJSONSetting(aiPromptSettingsKey, normalized); err != nil {
		return AIPromptSettings{}, err
	}
	return normalized, nil
}

func (a *App) customPromptForRequest(request ai.AssistRequest) string {
	rule := a.aiPromptRuleForBundleID(request.AppBundleID)
	if request.ContextKind == ai.ContextSelectedText && strings.TrimSpace(request.ContextText) != "" {
		if !rule.UseSelectedText {
			return ""
		}
		return rule.SelectedTextPrompt
	}
	if !rule.RunWithoutSelection {
		return ""
	}
	return rule.NoSelectionPrompt
}

func (a *App) aiPromptRuleForBundleID(bundleID string) AIPromptRule {
	settings, err := a.GetAIPromptSettings()
	if err != nil {
		return DefaultAIPromptSettings().Common
	}
	bundleID = strings.TrimSpace(bundleID)
	for _, profile := range settings.Profiles {
		if strings.TrimSpace(profile.AppBundleID) != "" && profile.AppBundleID == bundleID {
			return profile.AIPromptRule
		}
	}
	return settings.Common
}

func DefaultGeneralSettings() GeneralSettings {
	return GeneralSettings{
		ThemeMode:          ThemeAuto,
		Language:           LanguageEnglish,
		TypingTrendEnabled: true,
		SoundName:          "None",
	}
}

func DefaultAIPromptSettings() AIPromptSettings {
	return AIPromptSettings{
		Common: AIPromptRule{
			UseSelectedText:     true,
			RunWithoutSelection: true,
		},
		Profiles: []AIPromptProfile{},
	}
}

func NormalizeAIPromptSettings(settings AIPromptSettings) AIPromptSettings {
	settings.Common = NormalizeAIPromptRule(settings.Common)
	if settings.Profiles == nil {
		settings.Profiles = []AIPromptProfile{}
	}
	for index := range settings.Profiles {
		settings.Profiles[index].ID = strings.TrimSpace(settings.Profiles[index].ID)
		if settings.Profiles[index].ID == "" {
			settings.Profiles[index].ID = fmt.Sprintf("profile-%d", index+1)
		}
		settings.Profiles[index].AppName = strings.TrimSpace(settings.Profiles[index].AppName)
		if settings.Profiles[index].AppName == "" {
			settings.Profiles[index].AppName = "New App"
		}
		settings.Profiles[index].AppBundleID = strings.TrimSpace(settings.Profiles[index].AppBundleID)
		settings.Profiles[index].AIPromptRule = NormalizeAIPromptRule(settings.Profiles[index].AIPromptRule)
	}
	return settings
}

func NormalizeAIPromptRule(rule AIPromptRule) AIPromptRule {
	rule.SelectedTextPrompt = strings.TrimSpace(rule.SelectedTextPrompt)
	rule.NoSelectionPrompt = strings.TrimSpace(rule.NoSelectionPrompt)
	return rule
}

func NormalizeGeneralSettings(settings GeneralSettings) GeneralSettings {
	settings.ThemeMode = strings.TrimSpace(settings.ThemeMode)
	switch settings.ThemeMode {
	case ThemeAuto, ThemeLight, ThemeDark:
	default:
		settings.ThemeMode = ThemeAuto
	}

	settings.Language = strings.TrimSpace(settings.Language)
	switch settings.Language {
	case LanguageEnglish, LanguageKorean:
	default:
		settings.Language = LanguageEnglish
	}

	settings.SoundName = strings.TrimSpace(settings.SoundName)
	if settings.SoundName == "" {
		settings.SoundName = "None"
	}

	return settings
}
