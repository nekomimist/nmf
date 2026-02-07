package fileinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathDisplay_LocalDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	resolved, parsed, err := ResolvePathDisplay(tmpDir)
	if err != nil {
		t.Fatalf("expected local path to resolve: %v", err)
	}
	if parsed.Scheme != SchemeFile {
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

	if _, _, err := ResolveDirectoryPath(filePath); err == nil {
		t.Fatalf("expected non-directory path to fail")
	}
}

func TestResolveDirectoryPath_SMBCanonicalDisplay(t *testing.T) {
	input := "smb://example.local/share/path/to/dir"

	resolved, parsed, err := ResolveDirectoryPath(input)
	if err != nil {
		t.Fatalf("expected SMB path parse to succeed: %v", err)
	}
	if parsed.Scheme != SchemeSMB {
		t.Fatalf("expected smb scheme, got %q", parsed.Scheme)
	}
	if !strings.HasPrefix(strings.ToLower(resolved), "smb://example.local/share") {
		t.Fatalf("unexpected SMB resolved path: %q", resolved)
	}
}

func TestResolveAccessibleDirectoryPath_LocalDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	resolved, parsed, err := ResolveAccessibleDirectoryPath(tmpDir)
	if err != nil {
		t.Fatalf("expected local path to resolve: %v", err)
	}
	if parsed.Scheme != SchemeFile {
		t.Fatalf("expected file scheme, got %q", parsed.Scheme)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("expected absolute local path, got %q", resolved)
	}
}

func TestResolveAccessibleDirectoryPath_EmptyRejected(t *testing.T) {
	if _, _, err := ResolveAccessibleDirectoryPath("   "); err == nil {
		t.Fatalf("expected empty path to fail")
	}
}
