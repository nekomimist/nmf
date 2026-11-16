package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

func TestBindingHelpReturnsToEachModeAndDoesNotStackOrChangeFiles(t *testing.T) {
	for _, mode := range []string{"main", "lineEdit", "fileViewer"} {
		t.Run(mode, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			fm := newFileViewerSessionTestManager([]fileinfo.FileInfo{{Name: "note.txt", Path: "/tmp/note.txt"}})
			fm.SetCursorByIndex(0)
			fm.config = config.Default()
			fm.config.UI.KeyBindings = []config.KeyBindingEntry{
				{Target: "fileViewer", Key: "Escape", Command: keymanager.CommandNoop},
			}
			fm.keyManager = keymanager.NewKeyManager(func(string, ...interface{}) {})
			fm.fileListView = ui.NewKeySink(fm.fileList, fm.keyManager)
			fm.window = test.NewWindow(fm.fileListView)
			defer fm.window.Close()
			fm.window.Resize(fyne.NewSize(1000, 700))
			fm.mainKeyHandler = keymanager.NewMainScreenKeyHandler(newMainScreenDependencies(fm), func(string, ...interface{}) {})
			fm.mainKeyHandler.SetTransitionGate(fm.keyManager.BeginOwnerTransition)
			fm.mainKeyHandler.SetActions(keymanager.DialogActions{ShowBindingHelp: func() { fm.showBindingHelp("main") }})
			fm.keyManager.SetBindingHelpAction(fm.showBindingHelp)
			fm.keyManager.PushHandler(fm.mainKeyHandler)
			fm.window.Canvas().Focus(fm.fileListView)
			var entry *ui.LineEditEntry
			switch mode {
			case "lineEdit":
				d := ui.NewLineEditDialog(ui.LineEditDialogOptions{InitialText: "abcd", InitialSelection: &ui.LineEditSelection{Start: 1, End: 3}}, fm.keyManager)
				d.ShowDialog(fm.window, nil)
				defer d.CancelDialog()
				entry = fm.window.Canvas().Focused().(*ui.LineEditEntry)
			case "fileViewer":
				d := ui.NewFileViewerDialog(&fileinfo.PreviewFile{Path: "note.txt", Text: "first\nsecond"}, fm.keyManager)
				d.SetKeyBindings(fm.config.UI.KeyBindings)
				d.ShowDialog(fm.window)
				defer d.CancelDialog()
			}
			owner := fm.keyManager.GetCurrentHandler()
			focus := fm.window.Canvas().Focused()
			overlays := len(fm.window.Canvas().Overlays().List())
			press := func(key fyne.KeyName) {
				ev := &fyne.KeyEvent{Name: key}
				fm.keyManager.HandleKeyDown(ev)
				fm.keyManager.HandleTypedKey(ev)
				fm.keyManager.HandleKeyUp(ev)
			}
			press(fyne.KeyF1)
			help := fm.bindingHelp
			if help == nil || len(fm.window.Canvas().Overlays().List()) != overlays+1 {
				t.Fatal("F1 should open one help overlay")
			}
			press(fyne.KeyF1)
			fm.showBindingHelp(mode) // Also guards a delayed duplicate request.
			if fm.bindingHelp != help || len(fm.window.Canvas().Overlays().List()) != overlays+1 {
				t.Fatal("repeated help request stacked another viewer")
			}
			help.ViewerFileNext()
			help.ViewerFilePrevious()
			help.ViewerMarkToggle()
			if fm.GetCurrentCursorIndex() != 0 || len(fm.GetSelectedFiles()) != 0 {
				t.Fatal("help navigation changed the underlying file list")
			}
			press(fyne.KeyEscape)
			if fm.bindingHelp != nil || fm.keyManager.GetCurrentHandler() != owner || fm.window.Canvas().Focused() != focus {
				t.Fatalf("help did not restore %s input owner and focus", mode)
			}
			if len(fm.window.Canvas().Overlays().List()) != overlays {
				t.Fatal("help overlay was not removed")
			}
			if entry != nil {
				if entry.SelectedText() != "bc" {
					t.Fatalf("selection after help = %q, want bc", entry.SelectedText())
				}
				entry.TypedRune('z')
				if entry.Text != "azd" {
					t.Fatalf("continued input = %q, want azd", entry.Text)
				}
			}
		})
	}
}
