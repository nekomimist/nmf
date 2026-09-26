package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/configscript"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

func newCommandErrorTestFileManager(t *testing.T) *FileManager {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	sink := ui.NewKeySink(widget.NewLabel("Main file list"), km)
	w := test.NewWindow(sink)
	w.Resize(fyne.NewSize(900, 600))
	w.Canvas().Focus(sink)
	fm := &FileManager{window: w, keyManager: km, fileListView: sink, windowActive: true}
	t.Cleanup(func() {
		if !fm.isWindowClosed() {
			w.Close()
		}
	})
	return fm
}

func installCommandErrorTestScript(t *testing.T, fm *FileManager, source string) (*keymanager.MainScreenKeyHandler, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "init.star")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	rt, err := configscript.Load(path, cfg, configscript.Options{})
	if err != nil {
		t.Fatal(err)
	}
	fm.config = cfg
	main := keymanager.NewMainScreenKeyHandlerWithCommands(newMainScreenDependencies(fm), debugPrint, cfg.UI.KeyBindings, rt.Commands)
	main.SetActions(keymanager.DialogActions{
		ShowCommandError: fm.ShowCommandError,
		ShowCommandMenu:  fm.ShowCommandMenu,
	})
	main.SetTransitionGate(fm.keyManager.BeginOwnerTransition)
	fm.keyManager.PushHandler(main)
	return main, path
}

func TestScriptErrorDialogThroughMainScreenPreservesRunResult(t *testing.T) {
	fm := newCommandErrorTestFileManager(t)
	main, path := installCommandErrorTestScript(t, fm, `def broken(ctx):
    nmf.window(width = 800)
def outer(ctx):
    if nmf.run("user.broken"):
        nmf.set_clipboard("dispatch succeeded")
nmf.command("user.broken", broken)
nmf.command("user.outer", outer)
nmf.key("F12", "user.outer")
`)
	ev := &fyne.KeyEvent{Name: fyne.KeyF12}
	fm.fileListView.KeyDown(ev)
	fm.fileListView.TypedKey(ev)
	if fm.commandErrorDialog == nil {
		t.Fatal("script error did not open a dialog through the main-screen context")
	}
	if got := fyne.CurrentApp().Clipboard().Content(); got != "dispatch succeeded" {
		t.Fatalf("nmf.run dispatch semantics changed: clipboard=%q", got)
	}
	fm.commandErrorDialog.CopyDetails()
	for _, want := range []string{"Command user.broken", "cannot be used", path + ":2:"} {
		if !strings.Contains(fyne.CurrentApp().Clipboard().Content(), want) {
			t.Errorf("error details missing %q", want)
		}
	}
	fm.commandErrorDialog.Close()
	if fm.commandErrorDialog != nil || fm.keyManager.GetCurrentHandler() != main || fm.window.Canvas().Focused() != fm.fileListView {
		t.Fatal("closing the error did not restore the main file list")
	}
}

func TestScriptMenuErrorDialogRestoresKeyboardInput(t *testing.T) {
	for _, activation := range []string{"Return", "accelerator"} {
		t.Run(activation, func(t *testing.T) {
			fm := newCommandErrorTestFileManager(t)
			main, path := installCommandErrorTestScript(t, fm, `def sort_size(ctx):
    nmf.sort(by = "size", temporary = False)
nmf.menu("sort", title = "Sort")
nmf.menu_item("sort", "Sort by File Size", key = "s", fn = sort_size)
def show(ctx):
    nmf.show_menu("sort")
nmf.key("F12", fn = show)
def resumed(ctx):
    nmf.set_clipboard("resumed")
nmf.key("F11", fn = resumed)
`)
			open := &fyne.KeyEvent{Name: fyne.KeyF12}
			fm.fileListView.KeyDown(open)
			fm.fileListView.TypedKey(open)
			menu, ok := fm.window.Canvas().Focused().(*ui.CommandMenu)
			if !ok {
				t.Fatalf("expected command menu, focus=%T", fm.window.Canvas().Focused())
			}
			if activation == "Return" {
				menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			} else {
				menu.TypedRune('s')
			}
			d := fm.commandErrorDialog
			if d == nil || len(fm.window.Canvas().Overlays().List()) != 1 {
				t.Fatal("menu was not replaced with an error dialog")
			}
			d.CopyDetails()
			for _, want := range []string{`Menu "Sort" > "Sort by File Size"`, "temporary=True", path + ":2:"} {
				if !strings.Contains(fyne.CurrentApp().Clipboard().Content(), want) {
					t.Errorf("error details missing %q", want)
				}
			}
			sink, ok := fm.window.Canvas().Focused().(*ui.KeySink)
			if !ok || sink == fm.fileListView {
				t.Fatal("error dialog did not take keyboard focus")
			}
			sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			if fm.commandErrorDialog == nil {
				t.Fatal("carried menu activation dismissed the error")
			}
			close := &fyne.KeyEvent{Name: fyne.KeyEscape}
			sink.KeyDown(close)
			sink.TypedKey(close)
			sink.KeyUp(close)
			if fm.commandErrorDialog != nil || fm.keyManager.GetCurrentHandler() != main || fm.window.Canvas().Focused() != fm.fileListView {
				t.Fatal("dismissing the menu error did not restore the file list")
			}
			resume := &fyne.KeyEvent{Name: fyne.KeyF11}
			fm.fileListView.KeyDown(resume)
			fm.fileListView.TypedKey(resume)
			if got := fyne.CurrentApp().Clipboard().Content(); got != "resumed" {
				t.Fatalf("subsequent command did not execute: clipboard=%q", got)
			}
		})
	}
}

func TestCommandErrorDialogsShareReportAndReleaseOnWindowClose(t *testing.T) {
	fm := newCommandErrorTestFileManager(t)
	fm.ShowCommandError("Command user.first", "Error: first failure")
	d := fm.commandErrorDialog
	if d == nil {
		t.Fatal("error dialog not shown")
	}
	fm.ShowCommandError("Command user.second", "Error: second failure")
	if fm.commandErrorDialog != d || len(fm.window.Canvas().Overlays().List()) != 1 {
		t.Fatal("multiple failures should share one dialog")
	}
	d.CopyDetails()
	for _, want := range []string{"first failure", "second failure"} {
		if !strings.Contains(fyne.CurrentApp().Clipboard().Content(), want) {
			t.Errorf("copied report missing %q", want)
		}
	}
	fm.closeWindow()
	if fm.commandErrorDialog != nil || fm.keyManager.GetCurrentHandler() != nil {
		t.Fatal("window close left a command error dialog or handler")
	}
	fm.ShowCommandError("late", "late error")
	if fm.commandErrorDialog != nil {
		t.Fatal("closed window accepted a late error dialog")
	}
}
