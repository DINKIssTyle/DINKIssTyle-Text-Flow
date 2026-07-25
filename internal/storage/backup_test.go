package storage

import (
	"testing"
)

func TestRestoreContentBackupReplacesLibraryAndSettings(t *testing.T) {
	store, err := Open(t.TempDir() + "/restore.sqlite3")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	labels := []Label{{
		ID:          10,
		Name:        "Work",
		Description: "Work snippets",
		Color:       "#123456",
		CreatedAt:   "2026-01-01T00:00:00Z",
		UpdatedAt:   "2026-01-02T00:00:00Z",
	}}
	snippets := []Snippet{{
		ID:            20,
		LabelID:       10,
		Shortcut:      ";hello",
		Title:         "Hello",
		Content:       "Hello, world!",
		ContentType:   "plain",
		Enabled:       true,
		CaseSensitive: true,
		UsePaste:      true,
		ExpandMode:    "instant",
		UsageCount:    7,
		CreatedAt:     "2026-01-03T00:00:00Z",
		UpdatedAt:     "2026-01-04T00:00:00Z",
	}}

	if err := store.RestoreContentBackup(labels, snippets, map[string]string{
		"ai.prompt.settings": `{"common":{"useSelectedText":true},"profiles":[]}`,
	}); err != nil {
		t.Fatal(err)
	}

	gotLabels, err := store.ListLabels()
	if err != nil {
		t.Fatal(err)
	}
	if len(gotLabels) != 1 || gotLabels[0].ID != 10 || gotLabels[0].Name != "Work" {
		t.Fatalf("unexpected labels: %#v", gotLabels)
	}

	gotSnippets, err := store.ListSnippets("")
	if err != nil {
		t.Fatal(err)
	}
	if len(gotSnippets) != 1 {
		t.Fatalf("unexpected snippets: %#v", gotSnippets)
	}
	got := gotSnippets[0]
	if got.ID != 20 || got.LabelID != 10 || got.Shortcut != ";hello" || got.UsageCount != 7 || !got.UsePaste {
		t.Fatalf("unexpected snippet: %#v", got)
	}

	value, found, err := store.GetSetting("ai.prompt.settings")
	if err != nil {
		t.Fatal(err)
	}
	if !found || value != `{"common":{"useSelectedText":true},"profiles":[]}` {
		t.Fatalf("unexpected setting: found=%v value=%q", found, value)
	}
}

func TestRestoreContentBackupRejectsInvalidDataWithoutChangingLibrary(t *testing.T) {
	store, err := Open(t.TempDir() + "/rollback.sqlite3")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	before, err := store.ListSnippets("")
	if err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 {
		t.Fatal("expected seeded snippets")
	}

	err = store.RestoreContentBackup([]Label{
		{ID: 10, Name: "Duplicate", Color: "#123456"},
		{ID: 11, Name: "Duplicate", Color: "#654321"},
	}, nil, map[string]string{"new.setting": "should-roll-back"})
	if err == nil {
		t.Fatal("expected duplicate label name error")
	}

	after, err := store.ListSnippets("")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("library changed after failed restore: before=%d after=%d", len(before), len(after))
	}
	if _, found, err := store.GetSetting("new.setting"); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("setting changed after failed restore")
	}
}
