package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dkst-text-flow/internal/ai"
	"dkst-text-flow/internal/flowengine"
	"dkst-text-flow/internal/hotkey"
	"dkst-text-flow/internal/loginitem"
	"dkst-text-flow/internal/platform"
	"dkst-text-flow/internal/storage"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/menu_icon.png
var menuIcon []byte

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
	ttsCmd                *exec.Cmd
	ttsMu                 sync.Mutex
	systray               *application.SystemTray
	supertonicEngine      *ai.SupertonicEngine
	supertonicEngineMu    sync.Mutex
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
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx

	store, err := storage.OpenDefault()
	if err != nil {
		println("failed to open storage:", err.Error())
		return err
	}
	a.store = store
	a.startExpansionSoundDispatcher(ctx)
	a.configureExpansionSoundEvent()
	flowengine.Start(a.store)
	a.configureAIHotkey()

	// Initialize the system tray
	appInst := application.Get()
	a.configureSystemTray(appInst)

	return nil
}

func (a *App) ServiceShutdown() error {
	appInst := application.Get()
	a.destroySystemTray()
	if appInst.GlobalShortcut != nil {
		_ = appInst.GlobalShortcut.UnregisterAll()
	}
	flowengine.SetExpansionHandler(nil)
	if a.expansionSoundStopper != nil {
		a.expansionSoundStopper()
		a.expansionSoundStopper = nil
	}
	flowengine.Stop()
	a.supertonicEngineMu.Lock()
	if a.supertonicEngine != nil {
		a.supertonicEngine.Destroy()
		a.supertonicEngine = nil
	}
	a.supertonicEngineMu.Unlock()

	if a.store == nil {
		return nil
	}
	if err := a.store.Close(); err != nil {
		println("failed to close storage:", err.Error())
	}
	return nil
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

	resultChan := make(chan bool, 1)
	appInst := application.Get()
	dialog := appInst.Dialog.Question().
		SetTitle("Delete Snippet").
		SetMessage(fmt.Sprintf("Delete snippet \"%s\"?\nThis action cannot be undone.", title))

	deleteBtn := dialog.AddButton("Delete")
	deleteBtn.OnClick(func() {
		resultChan <- true
	})

	cancelBtn := dialog.AddButton("Cancel")
	cancelBtn.OnClick(func() {
		resultChan <- false
	})

	dialog.SetDefaultButton(cancelBtn)
	dialog.SetCancelButton(cancelBtn)
	dialog.Show()

	return <-resultChan, nil
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

	resultChan := make(chan bool, 1)
	appInst := application.Get()
	dialog := appInst.Dialog.Question().
		SetTitle("Delete Label").
		SetMessage(fmt.Sprintf("Delete label \"%s\"?\nSnippets in this label will move back to All.", name))

	deleteBtn := dialog.AddButton("Delete")
	deleteBtn.OnClick(func() {
		resultChan <- true
	})

	cancelBtn := dialog.AddButton("Cancel")
	cancelBtn.OnClick(func() {
		resultChan <- false
	})

	dialog.SetDefaultButton(cancelBtn)
	dialog.SetCancelButton(cancelBtn)
	dialog.Show()

	return <-resultChan, nil
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
	appInst := application.Get()
	path, err := appInst.Dialog.OpenFile().
		SetTitle("Choose an application").
		SetDirectory("/Applications").
		PromptForSingleSelection()
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

func (a *App) GetTTSModelStatus() (ai.TTSModelStatus, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ai.TTSModelStatus{}, fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	supertonicDir := filepath.Join(configDir, "DKST Text Flow", "supertonic")
	return ai.CheckModelStatus(supertonicDir), nil
}

func (a *App) StartTTSModelDownload() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	supertonicDir := filepath.Join(configDir, "DKST Text Flow", "supertonic")
	return ai.StartTTSModelDownload(supertonicDir, func(status ai.TTSModelStatus) {
		appInst := application.Get()
		if appInst != nil {
			appInst.Event.Emit("tts:download-progress", status)
		}
	})
}

func (a *App) CancelTTSModelDownload() {
	ai.CancelTTSModelDownload()
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

func (a *App) showMainWindow() {
	appInst := application.Get()
	if mainWin, ok := appInst.Window.GetByName("main"); ok {
		application.InvokeSync(func() {
			mainWin.SetAlwaysOnTop(false)
			mainWin.UnMinimise()
			mainWin.SetMinSize(900, 560)
			mainWin.SetSize(900, 560)
			mainWin.Center()
			mainWin.Show()
			mainWin.Focus()
			appInst.Show()
			appInst.Event.Emit("app:show-main")
		})
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
				application.Get().Event.Emit("snippet:expanded")
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
	appInst := application.Get()
	if appInst.GlobalShortcut != nil {
		_ = appInst.GlobalShortcut.UnregisterAll()
	}

	if settings.Enabled && settings.Hotkey != "" {
		err := appInst.GlobalShortcut.Register(settings.Hotkey, func() {
			a.handleAIHotkey(platform.GetFrontmostPID())
		})
		if err != nil {
			println("failed to register AI hotkey:", err.Error())
		}
	}

	if settings.TTSEnabled && settings.TTSUseShortcut && settings.TTSShortcut != "" {
		err := appInst.GlobalShortcut.Register(settings.TTSShortcut, func() {
			a.handleTTSHotkey(platform.GetFrontmostPID())
		})
		if err != nil {
			println("failed to register TTS hotkey:", err.Error())
		}
	}
}

func (a *App) handleAIHotkey(sourceProcessID int) {
	a.showAIPrompt(sourceProcessID, true)
}

func (a *App) showAIPrompt(sourceProcessID int, requireEnabled bool) {
	settings, err := a.GetAISettings()
	if err != nil || (requireEnabled && !settings.Enabled) {
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
	if requireEnabled && invocation.Kind == ai.ContextNone && !rule.RunWithoutSelection {
		return
	}

	appInst := application.Get()
	if aiWin, ok := appInst.Window.GetByName("ai"); ok {
		application.InvokeSync(func() {
			aiWin.SetMinSize(460, 112)
			aiWin.SetSize(460, 112)
			aiWin.Center()
			aiWin.SetAlwaysOnTop(true)
			aiWin.UnMinimise()
			aiWin.Show()
			aiWin.Focus()
		})
		appInst.Event.Emit("ai:invoke", invocation)
	}
}

func (a *App) normalizeAISettings(settings ai.Settings) ai.Settings {
	normalized := ai.NormalizeSettings(settings)
	if parsed, err := hotkey.Parse(normalized.Hotkey); err == nil {
		normalized.Hotkey = parsed.Canonical
	} else {
		normalized.Hotkey = ai.DefaultHotkey
	}
	if parsed, err := hotkey.Parse(normalized.TTSShortcut); err == nil {
		normalized.TTSShortcut = parsed.Canonical
	} else {
		normalized.TTSShortcut = "Cmd+Shift+T"
	}
	return normalized
}

func (a *App) Speak(text string) error {
	settings, err := a.GetAISettings()
	if err != nil {
		return err
	}
	return a.speakWithSettings(text, settings, true)
}

func (a *App) TestSpeak(text string, settings ai.Settings) error {
	settings = a.normalizeAISettings(settings)
	settings.TTSEnabled = true
	return a.speakWithSettings(text, settings, false)
}

func (a *App) speakWithSettings(text string, settings ai.Settings, requireEnabled bool) error {
	a.ttsMu.Lock()
	defer a.ttsMu.Unlock()

	// Stop any existing playback
	if a.ttsCmd != nil && a.ttsCmd.Process != nil {
		_ = a.ttsCmd.Process.Kill()
		_ = a.ttsCmd.Wait()
		a.ttsCmd = nil
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	if requireEnabled && !settings.TTSEnabled {
		return errors.New("TTS is disabled in settings")
	}

	switch settings.TTSEngine {
	case "os":
		a.ttsCmd = exec.Command("say", text)
		err := a.ttsCmd.Start()
		if err != nil {
			return fmt.Errorf("failed to start say command: %w", err)
		}
		go func(cmd *exec.Cmd) {
			_ = cmd.Wait()
			a.ttsMu.Lock()
			if a.ttsCmd == cmd {
				a.ttsCmd = nil
			}
			a.ttsMu.Unlock()
		}(a.ttsCmd)

	case "supertonic3":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("failed to resolve user config dir: %w", err)
		}
		supertonicDir := filepath.Join(configDir, "DKST Text Flow", "supertonic")

		status := ai.CheckModelStatus(supertonicDir)
		if !status.IsDownloaded {
			return errors.New("TTS model and runtime are not downloaded; please configure them in settings")
		}

		// Initialize local engine if not loaded yet
		a.supertonicEngineMu.Lock()
		if a.supertonicEngine == nil {
			engine, err := ai.LoadSupertonicEngine(supertonicDir)
			if err != nil {
				a.supertonicEngineMu.Unlock()
				return fmt.Errorf("failed to load local Supertonic engine: %w", err)
			}
			a.supertonicEngine = engine
		}
		engine := a.supertonicEngine
		a.supertonicEngineMu.Unlock()

		// Load selected voice style JSON
		voiceStylePath := filepath.Join(supertonicDir, "voice_styles", settings.TTSVoice+".json")
		style, err := ai.LoadVoiceStyle(voiceStylePath)
		if err != nil {
			return fmt.Errorf("failed to load voice style %s: %w", settings.TTSVoice, err)
		}
		defer style.Destroy()

		// Determine language: Check if Korean Hangul is present, otherwise language-agnostic "na"
		lang := "na"
		for _, r := range text {
			if r >= 0xAC00 && r <= 0xD7A3 {
				lang = "ko"
				break
			}
		}

		// Synthesize waveform
		wavData, err := engine.Synthesize(text, lang, style, settings.TTSSteps, float32(settings.TTSSpeed))
		if err != nil {
			return fmt.Errorf("failed to synthesize speech: %w", err)
		}

		// Write to a temporary WAV file
		tempFile, err := os.CreateTemp("", "dkst-tts-*.wav")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		tempFilePath := tempFile.Name()
		_ = tempFile.Close() // close immediately so we can write to it using WriteWav

		err = ai.WriteWav(tempFilePath, wavData, engine.SampleRate)
		if err != nil {
			_ = os.Remove(tempFilePath)
			return fmt.Errorf("failed to write WAV data: %w", err)
		}

		a.ttsCmd = exec.Command("afplay", tempFilePath)
		err = a.ttsCmd.Start()
		if err != nil {
			_ = os.Remove(tempFilePath)
			return fmt.Errorf("failed to start afplay command: %w", err)
		}

		go func(cmd *exec.Cmd, filePath string) {
			_ = cmd.Wait()
			_ = os.Remove(filePath)
			a.ttsMu.Lock()
			if a.ttsCmd == cmd {
				a.ttsCmd = nil
			}
			a.ttsMu.Unlock()
		}(a.ttsCmd, tempFilePath)

	default:
		return fmt.Errorf("unknown TTS engine: %s", settings.TTSEngine)
	}

	return nil
}

func (a *App) StopSpeaking() {
	a.ttsMu.Lock()
	defer a.ttsMu.Unlock()

	if a.ttsCmd != nil && a.ttsCmd.Process != nil {
		_ = a.ttsCmd.Process.Kill()
		_ = a.ttsCmd.Wait()
		a.ttsCmd = nil
	}
}

func (a *App) handleTTSHotkey(sourceProcessID int) {
	settings, err := a.GetAISettings()
	if err != nil || !settings.TTSEnabled || !settings.TTSUseShortcut {
		return
	}

	a.ttsMu.Lock()
	isSpeaking := a.ttsCmd != nil
	a.ttsMu.Unlock()

	if isSpeaking {
		a.StopSpeaking()
		return
	}

	selected, err := platform.SelectedTextFromProcess(sourceProcessID)
	if err != nil || strings.TrimSpace(selected) == "" {
		return
	}

	_ = a.Speak(selected)
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
