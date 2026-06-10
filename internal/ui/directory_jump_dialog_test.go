package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"nmf/internal/config"
	"nmf/internal/keymanager"
)

func TestNormalizeDirectoryJumpShortcut(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "lower", in: "p", want: "p"},
		{name: "upper", in: "P", want: "p"},
		{name: "trim", in: " P ", want: "p"},
		{name: "multiple", in: "Proj", want: "proj"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeDirectoryJumpShortcut(tt.in); got != tt.want {
				t.Fatalf("NormalizeDirectoryJumpShortcut(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSortDirectoryJumpEntriesOrdersShortcutsBeforeEmpty(t *testing.T) {
	entries := []config.DirectoryJumpEntry{
		{Shortcut: "", Directory: "/tmp"},
		{Shortcut: "proj", Directory: "/projects"},
		{Shortcut: "d", Directory: "/downloads"},
		{Shortcut: "pa", Directory: "/archive"},
		{Shortcut: "", Directory: "/var/tmp"},
		{Shortcut: "b", Directory: "/backup"},
	}

	sorted := sortDirectoryJumpEntries(copyDirectoryJumpEntries(entries))
	got := make([]string, len(sorted))
	for i, entry := range sorted {
		got[i] = entry.Shortcut + ":" + entry.Directory
	}
	want := []string{
		"b:/backup",
		"d:/downloads",
		"pa:/archive",
		"proj:/projects",
		":/tmp",
		":/var/tmp",
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted entries = %v, want %v", got, want)
		}
	}
}

func TestFilterDirectoryJumpEntriesMatchesShortcutPrefixOnly(t *testing.T) {
	entries := []config.DirectoryJumpEntry{
		{Shortcut: "proj", Directory: "/work/projects"},
		{Shortcut: "pa", Directory: "/work/archive"},
		{Shortcut: "", Directory: "/plain"},
		{Shortcut: "x", Directory: "/tmp"},
	}

	filtered := filterDirectoryJumpEntries(entries, "p")
	if len(filtered) != 2 || filtered[0].Directory != "/work/projects" || filtered[1].Directory != "/work/archive" {
		t.Fatalf("shortcut prefix query result = %+v, want projects and archive", filtered)
	}

	filtered = filterDirectoryJumpEntries(entries, "work")
	if len(filtered) != 0 {
		t.Fatalf("directory-only query result = %+v, want empty", filtered)
	}

	filtered = filterDirectoryJumpEntries(entries, "pr")
	if len(filtered) != 1 || filtered[0].Directory != "/work/projects" {
		t.Fatalf("unique shortcut query result = %+v, want only /work/projects", filtered)
	}

	filtered = filterDirectoryJumpEntries(entries, "")
	if len(filtered) != len(entries) {
		t.Fatalf("empty query result length = %d, want %d", len(filtered), len(entries))
	}
}

func TestDirectoryJumpUniqueShortcutDefersAcceptUntilKeyUp(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	dialog := NewDirectoryJumpDialog([]config.DirectoryJumpEntry{
		{Shortcut: "e", Directory: "/target"},
	}, km, func(string, ...interface{}) {})
	var acceptedPath string
	dialog.callback = func(path string) {
		acceptedPath = path
	}
	dialog.kmToken = km.PushHandler(keymanager.NewDirectoryJumpDialogKeyHandler(dialog, func(string, ...interface{}) {}))

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyE})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyE})
	km.HandleTypedRune('e')

	if acceptedPath != "" {
		t.Fatalf("accepted path before key up = %q, want empty", acceptedPath)
	}
	if dialog.closed {
		t.Fatal("dialog should stay open until key up")
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyE})

	if acceptedPath != "/target" {
		t.Fatalf("accepted path after key up = %q, want /target", acceptedPath)
	}
	if !dialog.closed {
		t.Fatal("dialog should be closed after deferred accept")
	}
	if got := km.GetStackSize(); got != 0 {
		t.Fatalf("key manager stack size = %d, want 0", got)
	}
}

func TestDirectoryJumpManualAcceptDefersCloseUntilKeyUp(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	dialog := NewDirectoryJumpDialog([]config.DirectoryJumpEntry{
		{Shortcut: "e", Directory: "/target"},
	}, km, func(string, ...interface{}) {})
	dialog.selectedPath = "/target"
	var acceptedPath string
	dialog.callback = func(path string) {
		acceptedPath = path
	}
	dialog.kmToken = km.PushHandler(keymanager.NewDirectoryJumpDialogKeyHandler(dialog, func(string, ...interface{}) {}))

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	dialog.AcceptSelection()

	if acceptedPath != "" {
		t.Fatalf("accepted path before key up = %q, want empty", acceptedPath)
	}
	if got := km.GetStackSize(); got != 1 {
		t.Fatalf("key manager stack size before key up = %d, want 1", got)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if acceptedPath != "/target" {
		t.Fatalf("accepted path after key up = %q, want /target", acceptedPath)
	}
	if got := km.GetStackSize(); got != 0 {
		t.Fatalf("key manager stack size after key up = %d, want 0", got)
	}
}
