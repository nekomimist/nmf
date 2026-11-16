package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

func TestBindingHelpMarkdownOrderingAndEscaping(t *testing.T) {
	modes := map[string][]keymanager.BindingDescription{
		"main": {{Key: "F6", Command: "user.__key.1", Description: "function|name_*<>&\nnext", Custom: true}},
	}
	for _, origin := range []string{"main", "lineEdit", "fileViewer"} {
		text := BindingHelpMarkdown(origin, modes)
		first := strings.Index(text, "## ")
		if !strings.HasPrefix(text[first:], "## "+origin+"\n") {
			t.Fatalf("first section for %s is wrong: %s", origin, text[first:])
		}
		for _, target := range []string{"main", "lineEdit", "fileViewer"} {
			if strings.Count(text, "## "+target+"\n") != 1 {
				t.Fatalf("expected one %s section", target)
			}
		}
		// Verify actual rendered cells, including user text that contains Markdown.
		rendered := markdownViewerText(&fileinfo.PreviewFile{Text: text})
		for _, want := range []string{"user.__key.1", "function|name_*<>&", "next", "Custom"} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("rendered help missing %q: %s", want, rendered)
			}
		}
	}
}

func TestEmbeddedLineEditInputsCanRequestHelp(t *testing.T) {
	for _, kind := range []string{"conflict", "smb", "archive"} {
		t.Run(kind, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			w := test.NewWindow(nil)
			defer w.Close()
			km := keymanager.NewKeyManager(func(string, ...interface{}) {})
			target := ""
			km.SetBindingHelpAction(func(mode string) { target = mode })
			var entry fyne.Focusable
			switch kind {
			case "conflict":
				e := newConflictNameEntry(km, nil)
				w.SetContent(e)
				w.Canvas().Focus(e)
				e.KeyDown(&fyne.KeyEvent{Name: fyne.KeyF1})
				e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
				entry = e
			case "smb":
				d := newSMBLoginDialog("server", "share", w, km, nil, nil)
				d.show()
				defer d.CancelDialog()
				typeEntryKey(d.username, fyne.KeyF1)
				entry = d.username
			case "archive":
				d := newArchivePasswordDialog(fileinfo.ArchivePasswordRequest{}, w, km, nil, nil)
				d.show()
				defer d.CancelDialog()
				typeEntryKey(d.entry, fyne.KeyF1)
				entry = d.entry
			}
			if target != keymanager.KeyBindingTargetLineEdit || w.Canvas().Focused() != entry {
				t.Fatalf("help target=%q focus=%T, want lineEdit from %s", target, w.Canvas().Focused(), kind)
			}
		})
	}
}

func TestViewerInputHelpPreservesSearchAndPreviewState(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(nil)
	defer w.Close()
	w.Resize(fyne.NewSize(1000, 700))
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewFileViewerDialog(&fileinfo.PreviewFile{Path: "file.txt", Text: strings.Repeat("some text\n", 100)}, km)
	d.ShowDialog(w)
	defer d.CancelDialog()
	d.ViewerPageDown()
	line := d.textGrid.CurrentLine()
	d.search.SetText("selected")
	d.search.SelectAll()
	w.Canvas().Focus(d.search)
	calls := 0
	km.SetBindingHelpAction(func(target string) {
		calls++
		if target != "fileViewer" {
			t.Fatalf("target = %q, want fileViewer", target)
		}
		focus := w.Canvas().Focused()
		help := NewFileViewerDialog(&fileinfo.PreviewFile{Path: "help.md", Text: "# Help", Markdown: true}, km)
		help.SetAfterClosed(func() { w.Canvas().Focus(focus) })
		help.ShowDialog(w)
		help.CloseViewer()
	})
	d.search.KeyDown(&fyne.KeyEvent{Name: fyne.KeyF1})
	d.search.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
	if calls != 1 || w.Canvas().Focused() != d.search || d.search.SelectedText() != "selected" || d.textGrid.CurrentLine() != line {
		t.Fatal("help did not preserve search selection, focus and preview scroll")
	}
	d.search.TypedRune('z')
	if d.search.Text != "z" {
		t.Fatalf("continued search input = %q, want z", d.search.Text)
	}
	d.UpdatePreview(&fileinfo.PreviewFile{Path: "next.txt", Text: "next"})
	w.Canvas().Focus(d.jump)
	d.jump.KeyDown(&fyne.KeyEvent{Name: fyne.KeyF1})
	d.jump.TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
	if calls != 2 {
		t.Fatal("replacement preview inputs lost the help binding")
	}
}
