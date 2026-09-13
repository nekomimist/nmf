//go:build windows

package fileinfo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nziu/lnk"
)

func TestResolveShortcutNavigationDirToDirectory(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	shortcut := filepath.Join(tmp, "target.lnk")
	makeTestShortcut(t, shortcut, target)

	got, ok, err := ResolveShortcutNavigationDir(shortcut)
	if err != nil {
		t.Fatalf("ResolveShortcutNavigationDir returned error: %v", err)
	}
	if !ok {
		t.Fatal("ResolveShortcutNavigationDir ok = false, want true")
	}
	assertShortcutNavigationDir(t, got, target)
}

func TestResolveShortcutNavigationDirToFileParent(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, "file.txt")
	if err := os.WriteFile(target, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	shortcut := filepath.Join(tmp, "file.LNK")
	makeTestShortcut(t, shortcut, target)

	got, ok, err := ResolveShortcutNavigationDir(shortcut)
	if err != nil {
		t.Fatalf("ResolveShortcutNavigationDir returned error: %v", err)
	}
	if !ok {
		t.Fatal("ResolveShortcutNavigationDir ok = false, want true")
	}
	assertShortcutNavigationDir(t, got, targetDir)
}

func TestResolveShortcutNavigationDirMissingTargetReportsTargetError(t *testing.T) {
	tmp := t.TempDir()
	shortcut := filepath.Join(tmp, "missing.lnk")
	makeTestShortcut(t, shortcut, filepath.Join(tmp, "missing.txt"))

	_, ok, err := ResolveShortcutNavigationDir(shortcut)
	if err == nil {
		t.Fatal("ResolveShortcutNavigationDir error = nil, want missing target error")
	}
	if ok {
		t.Fatal("ResolveShortcutNavigationDir ok = true, want false")
	}
	var shortcutErr *ShortcutNavigationError
	if !errors.As(err, &shortcutErr) || shortcutErr.Stage != ShortcutNavigationTarget {
		t.Fatalf("ResolveShortcutNavigationDir error = %#v, want target-stage error", err)
	}
}

func TestResolveShortcutNavigationDirNonShortcut(t *testing.T) {
	got, ok, err := ResolveShortcutNavigationDir(filepath.Join(t.TempDir(), "file.txt"))
	if err != nil {
		t.Fatalf("ResolveShortcutNavigationDir returned error: %v", err)
	}
	if ok || got != "" {
		t.Fatalf("ResolveShortcutNavigationDir = %q, %t, want empty, false", got, ok)
	}
}

func TestResolveShortcutNavigationDirContextCanceledBeforeCOM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, ok, err := ResolveShortcutNavigationDirContext(ctx, filepath.Join(t.TempDir(), "target.lnk"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveShortcutNavigationDirContext error = %v, want context.Canceled", err)
	}
	if ok {
		t.Fatal("ResolveShortcutNavigationDirContext ok = true, want false")
	}
}

func assertShortcutNavigationDir(t *testing.T, got, want string) {
	t.Helper()
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat resolved directory %q: %v", got, err)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		t.Fatalf("stat expected directory %q: %v", want, err)
	}
	// Windows may expand an 8.3 path from TEMP when resolving a shortcut.
	// Verify the destination's identity instead of its path spelling.
	if !gotInfo.IsDir() || !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("ResolveShortcutNavigationDir = %q, want directory %q", got, want)
	}
}

func makeTestShortcut(t *testing.T, path string, target string) {
	t.Helper()
	if err := lnk.Make(path, lnk.Shortcut{TargetPath: target}); err != nil {
		if strings.Contains(err.Error(), "failed to initialize shell") || strings.Contains(err.Error(), "failed to create WScript.Shell") {
			t.Skipf("Windows shortcut COM unavailable: %v", err)
		}
		t.Fatalf("failed to create shortcut: %v", err)
	}
}
