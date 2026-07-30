package app

import (
	"testing"

	"dkst-text-flow/internal/ai"
	"dkst-text-flow/internal/hotkey"
	"dkst-text-flow/internal/ocr"
	"dkst-text-flow/internal/storage"
)

func TestNormalizeGeneralSettingsCanonicalizesFlowToggleHotkey(t *testing.T) {
	shortcut, err := hotkey.Parse("ctrl+alt+p")
	if err != nil {
		t.Fatal(err)
	}

	settings := NormalizeGeneralSettings(GeneralSettings{FlowToggleHotkey: "ctrl+alt+p"})
	if settings.FlowToggleHotkey != shortcut.Canonical {
		t.Fatalf("FlowToggleHotkey = %q, want %q", settings.FlowToggleHotkey, shortcut.Canonical)
	}
}

func TestNormalizeGeneralSettingsClearsInvalidFlowToggleHotkey(t *testing.T) {
	settings := NormalizeGeneralSettings(GeneralSettings{FlowToggleHotkey: "F"})
	if settings.FlowToggleHotkey != "" {
		t.Fatalf("FlowToggleHotkey = %q, want empty", settings.FlowToggleHotkey)
	}
}

func TestDefaultGeneralSettingsLeavesFlowToggleHotkeyEmpty(t *testing.T) {
	if value := DefaultGeneralSettings().FlowToggleHotkey; value != "" {
		t.Fatalf("FlowToggleHotkey = %q, want empty", value)
	}
}

func TestNormalizeGeneralSettingsCanonicalizesOCRHotkey(t *testing.T) {
	shortcut, err := hotkey.Parse("cmd+shift+o")
	if err != nil {
		t.Fatal(err)
	}

	settings := NormalizeGeneralSettings(GeneralSettings{OCRHotkey: "cmd+shift+o"})
	if settings.OCRHotkey != shortcut.Canonical {
		t.Fatalf("OCRHotkey = %q, want %q", settings.OCRHotkey, shortcut.Canonical)
	}
}

func TestNormalizeGeneralSettingsClearsInvalidOCRHotkey(t *testing.T) {
	settings := NormalizeGeneralSettings(GeneralSettings{OCRHotkey: "O"})
	if settings.OCRHotkey != "" {
		t.Fatalf("OCRHotkey = %q, want empty", settings.OCRHotkey)
	}
}

func TestDefaultGeneralSettingsUsesAutomaticOCRAndShowsResult(t *testing.T) {
	settings := DefaultGeneralSettings()
	if settings.AppleVisionOCREnabled {
		t.Fatal("AppleVisionOCREnabled = true, want false")
	}
	if settings.OCRHotkey != "" {
		t.Fatalf("OCRHotkey = %q, want empty", settings.OCRHotkey)
	}
	if settings.OCRRecognitionLanguage != ocr.LanguageAutomatic {
		t.Fatalf("OCRRecognitionLanguage = %q, want %q", settings.OCRRecognitionLanguage, ocr.LanguageAutomatic)
	}
	if settings.OCRResultAction != ocr.ResultActionShow {
		t.Fatalf("OCRResultAction = %q, want %q", settings.OCRResultAction, ocr.ResultActionShow)
	}
}

func TestNormalizeGeneralSettingsRepairsInvalidOCRValues(t *testing.T) {
	settings := NormalizeGeneralSettings(GeneralSettings{
		OCRRecognitionLanguage: " ",
		OCRResultAction:        "invalid",
	})
	if settings.OCRRecognitionLanguage != ocr.LanguageAutomatic {
		t.Fatalf("OCRRecognitionLanguage = %q, want %q", settings.OCRRecognitionLanguage, ocr.LanguageAutomatic)
	}
	if settings.OCRResultAction != ocr.ResultActionShow {
		t.Fatalf("OCRResultAction = %q, want %q", settings.OCRResultAction, ocr.ResultActionShow)
	}
}

func TestNormalizeGeneralSettingsPreservesSupportedOCRValues(t *testing.T) {
	settings := NormalizeGeneralSettings(GeneralSettings{
		OCRRecognitionLanguage: "ko-KR",
		OCRResultAction:        ocr.ResultActionClipboard,
	})
	if settings.OCRRecognitionLanguage != "ko-KR" {
		t.Fatalf("OCRRecognitionLanguage = %q, want ko-KR", settings.OCRRecognitionLanguage)
	}
	if settings.OCRResultAction != ocr.ResultActionClipboard {
		t.Fatalf("OCRResultAction = %q, want %q", settings.OCRResultAction, ocr.ResultActionClipboard)
	}
}

func TestUpdateStartAtLoginSkipsUnchangedStoredValue(t *testing.T) {
	store, err := storage.Open(t.TempDir() + "/settings.sqlite3")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	settings := DefaultGeneralSettings()
	settings.StartAtLogin = true
	if err := store.SetJSONSetting(generalSettingsKey, settings); err != nil {
		t.Fatal(err)
	}

	application := &App{store: store}
	if err := application.updateStartAtLoginIfChanged(true); err != nil {
		t.Fatalf("unchanged start-at-login setting returned an error: %v", err)
	}
}

func TestValidateUniqueHotkeysRejectsDuplicateAssignments(t *testing.T) {
	general := GeneralSettings{FlowToggleHotkey: "Cmd+Shift+F"}
	settings := ai.Settings{Hotkey: "Cmd+Shift+Space", TTSShortcut: "Cmd+Shift+F"}
	if err := validateUniqueHotkeys(general, settings); err == nil {
		t.Fatal("expected duplicate hotkey error")
	}
}

func TestValidateUniqueHotkeysRejectsDuplicateOCRAssignment(t *testing.T) {
	general := GeneralSettings{
		FlowToggleHotkey: "Cmd+Shift+F",
		OCRHotkey:        "Cmd+Shift+O",
	}
	settings := ai.Settings{Hotkey: "Cmd+Shift+O", TTSShortcut: "Cmd+Shift+T"}
	if err := validateUniqueHotkeys(general, settings); err == nil {
		t.Fatal("expected duplicate OCR hotkey error")
	}
}

func TestValidateUniqueHotkeysAllowsEmptyAndDistinctAssignments(t *testing.T) {
	general := GeneralSettings{}
	settings := ai.Settings{Hotkey: "Cmd+Shift+Space", TTSShortcut: "Cmd+Shift+T"}
	if err := validateUniqueHotkeys(general, settings); err != nil {
		t.Fatal(err)
	}
}

func TestInsertOCRTextAtCursorValidatesInput(t *testing.T) {
	application := &App{}
	if err := application.InsertOCRTextAtCursor(0, "recognized text"); err == nil {
		t.Fatal("expected missing OCR insertion target error")
	}
	if err := application.InsertOCRTextAtCursor(123, " "); err == nil {
		t.Fatal("expected empty OCR text error")
	}
}
