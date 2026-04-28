package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestResolveDirectoryPath_LocalDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	resolved, parsed, err := resolveDirectoryPath(tmpDir)
	if err != nil {
		t.Fatalf("expected directory to resolve: %v", err)
	}
	if parsed.Scheme != fileinfo.SchemeFile {
		t.Fatalf("expected file scheme, got %q", parsed.Scheme)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("expected absolute local path, got %q", resolved)
	}
}

func TestResolveDirectoryPath_LocalFileRejected(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if _, _, err := resolveDirectoryPath(filePath); err == nil {
		t.Fatalf("expected non-directory path to fail")
	}
}

func TestResolveDirectoryPath_SMBCanonicalDisplay(t *testing.T) {
	input := "smb://example.local/share/path/to/dir"

	resolved, parsed, err := resolveDirectoryPath(input)
	if err != nil {
		t.Fatalf("expected SMB path parse to succeed: %v", err)
	}
	if parsed.Scheme != fileinfo.SchemeSMB {
		t.Fatalf("expected smb scheme, got %q", parsed.Scheme)
	}
	if !strings.HasPrefix(strings.ToLower(resolved), "smb://example.local/share") {
		t.Fatalf("unexpected SMB resolved path: %q", resolved)
	}
}

func TestResolveDirectoryPath_EmptyRejected(t *testing.T) {
	if _, _, err := resolveDirectoryPath("   "); err == nil {
		t.Fatalf("expected empty path to fail")
	}
}

func TestSameDirectoryPath_LocalCleanedPath(t *testing.T) {
	tmpDir := t.TempDir()

	if !sameDirectoryPath(filepath.Join(tmpDir, "."), tmpDir) {
		t.Fatalf("expected cleaned local paths to match")
	}
}

func TestSameDirectoryPath_SMBNormalizedPath(t *testing.T) {
	if !sameDirectoryPath("smb://Example.Local/share/path/", "smb://example.local/share/path") {
		t.Fatalf("expected normalized SMB paths to match")
	}
}

func TestSameDirectoryPath_EmptyDoesNotMatch(t *testing.T) {
	if sameDirectoryPath("", "") {
		t.Fatalf("expected empty paths not to match")
	}
}
