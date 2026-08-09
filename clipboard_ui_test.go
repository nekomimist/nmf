package main

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestSetClipboardTextWritesApplicationClipboard(t *testing.T) {
	app := test.NewTempApp(t)
	defer app.Quit()

	fm := &FileManager{}
	if !fm.SetClipboardText("hello") {
		t.Fatal("SetClipboardText returned false")
	}
	if got := app.Clipboard().Content(); got != "hello" {
		t.Fatalf("clipboard content = %q, want hello", got)
	}
}

func TestCreateClipboardTextFileCreatesFile(t *testing.T) {
	app := test.NewTempApp(t)
	defer app.Quit()

	dir := t.TempDir()
	app.Clipboard().SetContent("from clipboard")
	fm := &FileManager{browser: newTestBrowser(testBrowserOptions{path: dir})}

	if !fm.CreateClipboardTextFile("clip.txt") {
		t.Fatal("CreateClipboardTextFile returned false")
	}
	path := filepath.Join(dir, "clip.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "from clipboard" {
		t.Fatalf("content = %q, want clipboard text", string(data))
	}
	originalFiles := fm.browserModel().OriginalFiles()
	if len(originalFiles) != 1 || originalFiles[0].Path != path {
		t.Fatalf("originalFiles = %#v, want created file", originalFiles)
	}
}
