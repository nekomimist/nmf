package fileinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRenameName(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{name: "new.txt", ok: true},
		{name: " new.txt ", ok: true},
		{name: "", ok: false},
		{name: ".", ok: false},
		{name: "..", ok: false},
		{name: "dir/file", ok: false},
		{name: `dir\file`, ok: false},
		{name: "bad\x00name", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateRenameName(tt.name)
			if tt.ok && err != nil {
				t.Fatalf("ValidateRenameName returned error: %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("ValidateRenameName returned nil error")
			}
			if tt.ok && got == "" {
				t.Fatal("ValidateRenameName returned empty name")
			}
		})
	}
}

func TestRenamePortableFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	newPath, err := RenamePortable(oldPath, "new.txt")
	if err != nil {
		t.Fatalf("RenamePortable returned error: %v", err)
	}
	if newPath != filepath.Join(dir, "new.txt") {
		t.Fatalf("newPath = %q, want %q", newPath, filepath.Join(dir, "new.txt"))
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old file still exists or unexpected error: %v", err)
	}
}

func TestRenamePortableDirectory(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old-dir")
	if err := os.Mkdir(oldPath, 0755); err != nil {
		t.Fatal(err)
	}

	newPath, err := RenamePortable(oldPath, "new-dir")
	if err != nil {
		t.Fatalf("RenamePortable returned error: %v", err)
	}
	if info, err := os.Stat(newPath); err != nil || !info.IsDir() {
		t.Fatalf("renamed directory missing or not directory: info=%v err=%v", info, err)
	}
}

func TestRenamePortableRejectsExistingTarget(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	existingPath := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := RenamePortable(oldPath, "existing.txt"); err == nil {
		t.Fatal("RenamePortable returned nil error for existing target")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("old file should remain: %v", err)
	}
}

func TestRenamePortableSameNameNoop(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "same.txt")
	if err := os.WriteFile(oldPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	newPath, err := RenamePortable(oldPath, "same.txt")
	if err != nil {
		t.Fatalf("RenamePortable returned error: %v", err)
	}
	if newPath != oldPath {
		t.Fatalf("newPath = %q, want %q", newPath, oldPath)
	}
}
