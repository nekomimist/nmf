package fileinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTextFilePortableCreatesFile(t *testing.T) {
	dir := t.TempDir()

	path, err := CreateTextFilePortable(dir, "note.txt", "hello\nworld")
	if err != nil {
		t.Fatalf("CreateTextFilePortable returned error: %v", err)
	}
	if path != filepath.Join(dir, "note.txt") {
		t.Fatalf("path = %q, want %q", path, filepath.Join(dir, "note.txt"))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "hello\nworld" {
		t.Fatalf("content = %q, want hello world", string(data))
	}
}

func TestCreateTextFilePortableRejectsExistingTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := CreateTextFilePortable(dir, "note.txt", "new"); err == nil {
		t.Fatal("CreateTextFilePortable should reject an existing target")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("content = %q, want old", string(data))
	}
}

func TestCreateTextFilePortableRejectsInvalidName(t *testing.T) {
	if _, err := CreateTextFilePortable(t.TempDir(), "../note.txt", "hello"); err == nil {
		t.Fatal("CreateTextFilePortable should reject path separators")
	}
}

func TestCreateTextFilePortableRejectsArchivePath(t *testing.T) {
	if _, err := CreateTextFilePortable(filepath.Join(t.TempDir(), "archive.zip")+"!/", "note.txt", "hello"); err == nil {
		t.Fatal("CreateTextFilePortable should reject archive paths")
	}
}
