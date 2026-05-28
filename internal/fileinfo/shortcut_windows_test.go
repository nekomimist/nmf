//go:build windows

package fileinfo

import (
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
	if got != target {
		t.Fatalf("ResolveShortcutNavigationDir = %q, want %q", got, target)
	}
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
	if got != targetDir {
		t.Fatalf("ResolveShortcutNavigationDir = %q, want %q", got, targetDir)
	}
}

func TestResolveShortcutNavigationDirMissingTargetFallsBack(t *testing.T) {
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

func makeTestShortcut(t *testing.T, path string, target string) {
	t.Helper()
	if err := lnk.Make(path, lnk.Shortcut{TargetPath: target}); err != nil {
		if strings.Contains(err.Error(), "failed to initialize shell") || strings.Contains(err.Error(), "failed to create WScript.Shell") {
			t.Skipf("Windows shortcut COM unavailable: %v", err)
		}
		t.Fatalf("failed to create shortcut: %v", err)
	}
}
