package main

import (
	"path/filepath"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"nmf/internal/config"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

func TestCreateDialogsRecordOnlySuccessfulNames(t *testing.T) {
	app := test.NewTempApp(t)
	app.Clipboard().SetContent("clipboard text")
	w := app.NewWindow("Create")
	defer w.Close()
	state := &config.State{}
	dir := t.TempDir()
	sm := config.NewStateManagerWithPath(filepath.Join(dir, "state.json"), nil)
	t.Cleanup(func() {
		if err := sm.Close(); err != nil {
			t.Error(err)
		}
	})
	fm := &FileManager{
		window:       w,
		browser:      newTestBrowser(testBrowserOptions{path: dir}),
		config:       config.Default(),
		state:        state,
		stateManager: sm,
		keyManager:   keymanager.NewKeyManager(nil),
	}
	enter := func(show func(), name string, key fyne.KeyName) {
		t.Helper()
		show()
		entry, ok := w.Canvas().Focused().(*ui.LineEditEntry)
		if !ok {
			t.Fatalf("focused object = %T, want a line edit", w.Canvas().Focused())
		}
		test.Type(entry, name)
		ev := &fyne.KeyEvent{Name: key}
		entry.KeyDown(ev)
		entry.TypedKey(ev)
		entry.KeyUp(ev)
	}
	enter(fm.ShowCreateDirectoryDialog, "directory-draft", fyne.KeyEscape)
	enter(fm.ShowClipboardTextFileDialog, "file-draft.txt", fyne.KeyEscape)
	enter(fm.ShowCreateDirectoryDialog, " notes ", fyne.KeyReturn)
	enter(fm.ShowClipboardTextFileDialog, " todo.txt ", fyne.KeyReturn)
	enter(fm.ShowClipboardTextFileDialog, "invalid/name", fyne.KeyReturn)

	if err := sm.Flush(); err != nil {
		t.Fatal(err)
	}
	loaded, err := sm.Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(loaded.CreateDirectoryHistory, []string{"notes"}) {
		t.Fatalf("saved directory names = %q, want [notes]", loaded.CreateDirectoryHistory)
	}
	if !slices.Equal(loaded.CreateTextFileHistory, []string{"todo.txt"}) {
		t.Fatalf("saved text file names = %q, want [todo.txt]", loaded.CreateTextFileHistory)
	}
}
