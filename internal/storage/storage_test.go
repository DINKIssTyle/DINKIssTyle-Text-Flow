package storage

import (
	"path/filepath"
	"testing"
)

func TestDeletedSnippetsStayDeletedAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deleted-snippets.sqlite3")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	snippets, err := store.ListSnippets("")
	if err != nil {
		t.Fatal(err)
	}
	if len(snippets) == 0 {
		t.Fatal("expected a new store to contain example snippets")
	}
	for _, snippet := range snippets {
		if err := store.DeleteSnippet(snippet.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	snippets, err = reopened.ListSnippets("")
	if err != nil {
		t.Fatal(err)
	}
	if len(snippets) != 0 {
		t.Fatalf("deleted snippets were recreated after reopen: %#v", snippets)
	}
}
