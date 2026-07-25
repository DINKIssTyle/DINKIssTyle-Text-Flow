package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dkst-text-flow/internal/storage"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	contentBackupFormat  = "dkst-text-flow-content-backup"
	contentBackupVersion = 1
	contentBackupMaxSize = 50 << 20
)

type contentBackupFile struct {
	Format    string            `json:"format"`
	Version   int               `json:"version"`
	CreatedAt string            `json:"createdAt"`
	Labels    []backupLabel     `json:"labels"`
	Snippets  []backupSnippet   `json:"snippets"`
	AIPrompts *AIPromptSettings `json:"aiPrompts"`
}

type backupLabel struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type backupSnippet struct {
	ID            int64  `json:"id"`
	LabelID       int64  `json:"labelId"`
	Shortcut      string `json:"shortcut"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	ContentType   string `json:"contentType"`
	Enabled       bool   `json:"enabled"`
	CaseSensitive bool   `json:"caseSensitive"`
	UsePaste      bool   `json:"usePaste"`
	ExpandMode    string `json:"expandMode"`
	UsageCount    int64  `json:"usageCount"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type backupDialogStrings struct {
	fileDescription string
	exportTitle     string
	exportMessage   string
	exportButton    string
	importTitle     string
	importMessage   string
	importButton    string
	confirmTitle    string
	confirmMessage  string
	confirmButton   string
	cancelButton    string
}

func backupStrings(language string) backupDialogStrings {
	if strings.EqualFold(strings.TrimSpace(language), LanguageKorean) {
		return backupDialogStrings{
			fileDescription: "DKST Text Flow 백업",
			exportTitle:     "스니펫 및 AI 프롬프트 백업",
			exportMessage:   "백업 파일을 저장할 위치를 선택하세요.",
			exportButton:    "백업",
			importTitle:     "스니펫 및 AI 프롬프트 불러오기",
			importMessage:   "불러올 DKST Text Flow 백업 파일을 선택하세요.",
			importButton:    "불러오기",
			confirmTitle:    "백업 불러오기",
			confirmMessage:  "현재 스니펫과 AI 프롬프트를 선택한 백업으로 교체하시겠습니까?\n이 작업은 취소할 수 없습니다.",
			confirmButton:   "교체",
			cancelButton:    "취소",
		}
	}
	return backupDialogStrings{
		fileDescription: "DKST Text Flow Backup",
		exportTitle:     "Back Up Snippets and AI Prompts",
		exportMessage:   "Choose where to save the backup file.",
		exportButton:    "Back Up",
		importTitle:     "Import Snippets and AI Prompts",
		importMessage:   "Choose a DKST Text Flow backup file to import.",
		importButton:    "Import",
		confirmTitle:    "Import Backup",
		confirmMessage:  "Replace the current snippets and AI prompts with the selected backup?\nThis action cannot be undone.",
		confirmButton:   "Replace",
		cancelButton:    "Cancel",
	}
}

// BackupSnippetsAndAIPrompts writes a JSON backup with the proprietary .DTF
// extension. It returns false when the save dialog is cancelled.
func (a *App) BackupSnippetsAndAIPrompts(language string) (bool, error) {
	if a.store == nil {
		return false, errors.New("storage is not ready")
	}

	labels, err := a.store.ListLabels()
	if err != nil {
		return false, err
	}
	snippets, err := a.store.ListSnippets("")
	if err != nil {
		return false, err
	}
	promptSettings, err := a.GetAIPromptSettings()
	if err != nil {
		return false, err
	}
	promptSettings = NormalizeAIPromptSettings(promptSettings)

	backup := contentBackupFile{
		Format:    contentBackupFormat,
		Version:   contentBackupVersion,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Labels:    make([]backupLabel, 0, len(labels)),
		Snippets:  make([]backupSnippet, 0, len(snippets)),
		AIPrompts: &promptSettings,
	}
	for _, label := range labels {
		backup.Labels = append(backup.Labels, backupLabel{
			ID:          label.ID,
			Name:        label.Name,
			Description: label.Description,
			Color:       label.Color,
			CreatedAt:   label.CreatedAt,
			UpdatedAt:   label.UpdatedAt,
		})
	}
	for _, snippet := range snippets {
		backup.Snippets = append(backup.Snippets, backupSnippet{
			ID:            snippet.ID,
			LabelID:       snippet.LabelID,
			Shortcut:      snippet.Shortcut,
			Title:         snippet.Title,
			Content:       snippet.Content,
			ContentType:   snippet.ContentType,
			Enabled:       snippet.Enabled,
			CaseSensitive: snippet.CaseSensitive,
			UsePaste:      snippet.UsePaste,
			ExpandMode:    snippet.ExpandMode,
			UsageCount:    snippet.UsageCount,
			CreatedAt:     snippet.CreatedAt,
			UpdatedAt:     snippet.UpdatedAt,
		})
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode backup: %w", err)
	}
	data = append(data, '\n')

	text := backupStrings(language)
	defaultFilename := fmt.Sprintf("DKST-Text-Flow-Backup-%s.DTF", time.Now().Format("2006-01-02"))
	saveDialog := application.Get().Dialog.SaveFile()
	saveDialog.SetOptions(&application.SaveFileDialogOptions{
		CanCreateDirectories: true,
		AllowOtherFileTypes:  false,
		Title:                text.exportTitle,
		Message:              text.exportMessage,
		Filename:             defaultFilename,
		ButtonText:           text.exportButton,
		Filters: []application.FileFilter{{
			DisplayName: text.fileDescription + " (*.DTF)",
			Pattern:     "*.DTF",
		}},
	})
	path, err := saveDialog.PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return false, nil
	}
	if !strings.EqualFold(filepath.Ext(path), ".dtf") {
		path += ".DTF"
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return false, fmt.Errorf("write backup: %w", err)
	}
	return true, nil
}

// ImportSnippetsAndAIPrompts validates and then atomically restores a .DTF
// backup. It returns false when either dialog is cancelled.
func (a *App) ImportSnippetsAndAIPrompts(language string) (bool, error) {
	if a.store == nil {
		return false, errors.New("storage is not ready")
	}

	text := backupStrings(language)
	path, err := application.Get().Dialog.OpenFile().
		SetTitle(text.importTitle).
		SetMessage(text.importMessage).
		SetButtonText(text.importButton).
		AddFilter(text.fileDescription+" (*.DTF)", "*.DTF;*.dtf").
		AllowsOtherFileTypes(false).
		PromptForSingleSelection()
	if err != nil {
		return false, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("inspect backup: %w", err)
	}
	if info.Size() > contentBackupMaxSize {
		return false, errors.New("backup file is too large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read backup: %w", err)
	}
	backup, err := decodeContentBackup(data)
	if err != nil {
		return false, err
	}

	confirmed, err := confirmContentBackupImport(text)
	if err != nil || !confirmed {
		return false, err
	}

	labels := make([]storage.Label, 0, len(backup.Labels))
	for _, label := range backup.Labels {
		labels = append(labels, storage.Label{
			ID:          label.ID,
			Name:        label.Name,
			Description: label.Description,
			Color:       label.Color,
			CreatedAt:   label.CreatedAt,
			UpdatedAt:   label.UpdatedAt,
		})
	}
	snippets := make([]storage.Snippet, 0, len(backup.Snippets))
	for _, snippet := range backup.Snippets {
		snippets = append(snippets, storage.Snippet{
			ID:            snippet.ID,
			LabelID:       snippet.LabelID,
			Shortcut:      snippet.Shortcut,
			Title:         snippet.Title,
			Content:       snippet.Content,
			ContentType:   snippet.ContentType,
			Enabled:       snippet.Enabled,
			CaseSensitive: snippet.CaseSensitive,
			UsePaste:      snippet.UsePaste,
			ExpandMode:    snippet.ExpandMode,
			UsageCount:    snippet.UsageCount,
			CreatedAt:     snippet.CreatedAt,
			UpdatedAt:     snippet.UpdatedAt,
		})
	}

	promptSettings := NormalizeAIPromptSettings(*backup.AIPrompts)
	promptData, err := json.Marshal(promptSettings)
	if err != nil {
		return false, fmt.Errorf("encode AI prompt settings: %w", err)
	}
	if err := a.store.RestoreContentBackup(labels, snippets, map[string]string{
		aiPromptSettingsKey: string(promptData),
	}); err != nil {
		return false, err
	}
	return true, nil
}

func decodeContentBackup(data []byte) (contentBackupFile, error) {
	var backup contentBackupFile
	if err := json.Unmarshal(data, &backup); err != nil {
		return contentBackupFile{}, fmt.Errorf("invalid backup JSON: %w", err)
	}
	if backup.Format != contentBackupFormat {
		return contentBackupFile{}, errors.New("this is not a DKST Text Flow backup")
	}
	if backup.Version != contentBackupVersion {
		return contentBackupFile{}, fmt.Errorf("unsupported backup version %d", backup.Version)
	}
	if backup.AIPrompts == nil {
		return contentBackupFile{}, errors.New("backup does not contain AI prompt settings")
	}
	if backup.Labels == nil {
		backup.Labels = []backupLabel{}
	}
	if backup.Snippets == nil {
		backup.Snippets = []backupSnippet{}
	}
	if len(backup.Labels) > 10_000 || len(backup.Snippets) > 100_000 || len(backup.AIPrompts.Profiles) > 10_000 {
		return contentBackupFile{}, errors.New("backup contains too many items")
	}
	normalizedPrompts := NormalizeAIPromptSettings(*backup.AIPrompts)
	profileIDs := make(map[string]struct{}, len(normalizedPrompts.Profiles))
	for _, profile := range normalizedPrompts.Profiles {
		if _, exists := profileIDs[profile.ID]; exists {
			return contentBackupFile{}, fmt.Errorf("backup contains duplicate AI prompt profile ID %q", profile.ID)
		}
		profileIDs[profile.ID] = struct{}{}
	}
	backup.AIPrompts = &normalizedPrompts
	return backup, nil
}

func confirmContentBackupImport(text backupDialogStrings) (bool, error) {
	resultChan := make(chan bool, 1)
	dialog := application.Get().Dialog.Question().
		SetTitle(text.confirmTitle).
		SetMessage(text.confirmMessage)

	replaceButton := dialog.AddButton(text.confirmButton)
	replaceButton.OnClick(func() {
		resultChan <- true
	})

	cancelButton := dialog.AddButton(text.cancelButton)
	cancelButton.OnClick(func() {
		resultChan <- false
	})
	dialog.SetDefaultButton(cancelButton)
	dialog.SetCancelButton(cancelButton)
	dialog.Show()

	return <-resultChan, nil
}
