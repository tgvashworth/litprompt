package lockfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashContent(t *testing.T) {
	// SHA-256 of "hello\n" is known
	got := HashContent("hello\n")
	want := "sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	if got != want {
		t.Errorf("HashContent = %q, want %q", got, want)
	}
}

func TestSave_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.lock")

	original := &Lockfile{
		Imports: map[string]Entry{
			"https://github.com/org/repo/blob/main/file.md": {
				Hash: "sha256:abc123",
			},
			"https://gitlab.com/org/repo/-/blob/v1/other.md": {
				Hash: "sha256:def456",
			},
		},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.Imports) != len(original.Imports) {
		t.Fatalf("loaded %d imports, want %d", len(loaded.Imports), len(original.Imports))
	}

	for url, origEntry := range original.Imports {
		loadedEntry, ok := loaded.Imports[url]
		if !ok {
			t.Errorf("missing import %q after roundtrip", url)
			continue
		}
		if loadedEntry.Hash != origEntry.Hash {
			t.Errorf("hash for %q = %q, want %q", url, loadedEntry.Hash, origEntry.Hash)
		}
	}
}

func TestSave_creates_file(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.lock")

	lf := &Lockfile{Imports: map[string]Entry{
		"https://github.com/org/repo/blob/main/file.md": {Hash: "sha256:abc"},
	}}

	if err := Save(path, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Error("saved file is empty")
	}
}
