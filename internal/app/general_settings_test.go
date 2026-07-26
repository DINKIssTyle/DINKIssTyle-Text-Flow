package app

import (
	"testing"

	"dkst-text-flow/internal/ai"
	"dkst-text-flow/internal/hotkey"
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

func TestValidateUniqueHotkeysRejectsDuplicateAssignments(t *testing.T) {
	general := GeneralSettings{FlowToggleHotkey: "Cmd+Shift+F"}
	settings := ai.Settings{Hotkey: "Cmd+Shift+Space", TTSShortcut: "Cmd+Shift+F"}
	if err := validateUniqueHotkeys(general, settings); err == nil {
		t.Fatal("expected duplicate hotkey error")
	}
}

func TestValidateUniqueHotkeysAllowsEmptyAndDistinctAssignments(t *testing.T) {
	general := GeneralSettings{}
	settings := ai.Settings{Hotkey: "Cmd+Shift+Space", TTSShortcut: "Cmd+Shift+T"}
	if err := validateUniqueHotkeys(general, settings); err != nil {
		t.Fatal(err)
	}
}
