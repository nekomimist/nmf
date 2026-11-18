package fileinfo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestResolveAccessibleDirectoryPathContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := ResolveAccessibleDirectoryPathContext(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestResolveDirectoryPathContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := ResolveDirectoryPathContext(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestSMBDisplayPreservesSelectedWhitespace(t *testing.T) {
	input := "smb://host/share/report\u2003"
	display, _, err := CanonicalDisplayPath(input)
	if err != nil || display != input {
		t.Fatalf("canonical path=%q err=%v", display, err)
	}
	if got := smbURLToUNC(input); got != "\\\\host\\share\\report\u2003" {
		t.Fatalf("UNC path = %q", got)
	}
}
