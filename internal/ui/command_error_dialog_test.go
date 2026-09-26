package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

type commandErrorTestMainHandler struct{ calls int }

func (h *commandErrorTestMainHandler) GetName() string { return "main" }
func (h *commandErrorTestMainHandler) OnKeyActivated(*fyne.KeyEvent, keymanager.ModifierState) bool {
	h.calls++
	return true
}
func (h *commandErrorTestMainHandler) OnTypedRune(rune, keymanager.ModifierState) bool {
	h.calls++
	return true
}

func TestCommandErrorDialogDismissalRestoresInputOwner(t *testing.T) {
	for _, closePath := range []string{"Return", "Enter", "Escape", "button", "hide", "window"} {
		t.Run(closePath, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			km := keymanager.NewKeyManager(func(string, ...interface{}) {})
			main := &commandErrorTestMainHandler{}
			km.PushHandler(main)
			mainSink := NewKeySink(widget.NewLabel("Main"), km)
			w := test.NewWindow(mainSink)
			defer w.Close()
			w.Resize(fyne.NewSize(900, 600))
			w.Canvas().Focus(mainSink)
			km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
			d := NewCommandErrorDialog("Command user.broken", "Error: test failure", km)
			closed := 0
			d.Show(w, func() {
				closed++
				w.Canvas().Focus(mainSink)
			})
			// The carried activation, repeat, and key-up must not dismiss it.
			d.sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			d.sink.KeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
			if closed != 0 || w.Canvas().Focused() != d.sink {
				t.Fatal("opening key dismissed the dialog or lost its focus")
			}
			d.sink.KeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
			d.sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyA})
			d.sink.TypedRune('a')
			if main.calls != 0 {
				t.Fatal("dialog input reached the main screen")
			}
			switch closePath {
			case "button":
				test.Tap(d.okButton)
			case "hide":
				d.dialog.Hide()
			case "window":
				d.Dismiss()
			default:
				ev := &fyne.KeyEvent{Name: fyne.KeyName(closePath)}
				d.sink.KeyDown(ev)
				d.sink.TypedKey(ev)
				d.sink.KeyUp(ev)
			}
			d.Close()
			d.Dismiss()
			if closed != 1 || km.GetCurrentHandler() != main || w.Canvas().Focused() != mainSink {
				t.Fatalf("close count=%d handler=%v focus=%T", closed, km.GetCurrentHandler(), w.Canvas().Focused())
			}
			if len(w.Canvas().Overlays().List()) != 0 {
				t.Fatal("dialog overlay remains after closing")
			}
			mainSink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
			if main.calls != 0 {
				t.Fatal("closing key repeat reached the main screen")
			}
			mainSink.KeyDown(&fyne.KeyEvent{Name: fyne.KeyDown})
			mainSink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
			if main.calls != 1 {
				t.Fatal("main screen did not resume on a fresh key press")
			}
		})
	}
}

func TestCommandErrorDialogScrollsAndCopiesFullDetails(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(widget.NewLabel("Main"))
	defer w.Close()
	w.Resize(fyne.NewSize(800, 600))
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	details := "Error: test failure\nLocation: helper.star:12:5 in broken\n\n" + strings.Repeat("Traceback frame\n", 100)
	d := NewCommandErrorDialog("Key C-E", details, km)
	d.Show(w, nil)
	defer d.Dismiss()
	if size := d.sink.Size(); size.Width > w.Canvas().Size().Width || size.Height > w.Canvas().Size().Height {
		t.Fatalf("dialog size %v exceeds canvas %v", size, w.Canvas().Size())
	}
	if d.scroll.Content.MinSize().Height <= d.scroll.Size().Height {
		t.Fatal("long report did not overflow its scroll viewport")
	}
	d.sink.KeyDown(&fyne.KeyEvent{Name: fyne.KeyPageDown})
	d.sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyPageDown})
	if d.scroll.Offset.Y <= 0 {
		t.Fatal("Page Down did not scroll the details")
	}
	d.Append("Command user.second", "Error: second failure")
	w.Canvas().Unfocus()
	test.Tap(d.copyButton)
	for _, want := range []string{"Key C-E", details, "Command user.second", "Error: second failure"} {
		if !strings.Contains(app.Clipboard().Content(), want) {
			t.Errorf("copied report missing %q", want)
		}
	}
	if w.Canvas().Focused() != d.sink {
		t.Fatal("copy button did not restore keyboard focus")
	}
	app.Clipboard().SetContent("")
	d.sink.KeyDown(&fyne.KeyEvent{Name: fyne.KeyC})
	d.sink.TypedShortcut(&fyne.ShortcutCopy{})
	if !strings.Contains(app.Clipboard().Content(), details) {
		t.Fatal("Ctrl+C did not copy the complete details")
	}
}
