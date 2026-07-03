package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/jobs"
	"nmf/internal/keymanager"
	customtheme "nmf/internal/theme"
)

type conflictEntryTestHandler struct {
	keys []fyne.KeyName
	mods []keymanager.ModifierState
}

func (h *conflictEntryTestHandler) OnKeyActivated(ev *fyne.KeyEvent, mods keymanager.ModifierState) bool {
	h.keys = append(h.keys, ev.Name)
	h.mods = append(h.mods, mods)
	return true
}
func (h *conflictEntryTestHandler) OnTypedRune(_ rune, _ keymanager.ModifierState) bool {
	return false
}
func (h *conflictEntryTestHandler) GetName() string { return "test" }

func TestConflictDialogSelectOverwriteUsesExactOption(t *testing.T) {
	dialog := &ConflictDialog{}
	dialog.choice = widget.NewRadioGroup([]string{
		conflictOverwriteIfNewerLabel,
		conflictOverwriteLabel,
	}, nil)
	dialog.choice.SetSelected(conflictOverwriteIfNewerLabel)

	dialog.SelectOverwrite()

	if got := dialog.choice.Selected; got != conflictOverwriteLabel {
		t.Fatalf("SelectOverwrite selected %q, want %q", got, conflictOverwriteLabel)
	}
}

func TestConflictDialogChoiceRequiresSelection(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	parent := test.NewWindow(widget.NewLabel(""))
	defer parent.Close()

	dialog := NewConflictDialog(jobs.ConflictRequest{
		SuggestedName: "file (1)",
		DefaultAction: jobs.ConflictOverwriteIfNewer,
	}, nil)
	dialog.ShowDialog(parent, func(jobs.ConflictResolution) {})
	defer dialog.CancelJob()

	if dialog.choice == nil {
		t.Fatal("conflict choice radio group was not created")
	}
	if !dialog.choice.Required {
		t.Fatal("conflict choice radio group should require one selected option")
	}
	if got := dialog.choice.Selected; got == "" {
		t.Fatal("conflict choice radio group should have a default selection")
	}
}

func TestConflictNameEntryForwardsAltShortcutsToKeyManager(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	handler := &conflictEntryTestHandler{}
	km.PushHandler(handler)
	entry := newConflictNameEntry(km, nil)

	entry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyAltLeft}) // modifier: tracked, does not arm
	entry.KeyDown(&fyne.KeyEvent{Name: fyne.KeyN})          // fresh press arms the gate
	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierAlt})

	if len(handler.keys) != 1 || handler.keys[0] != fyne.KeyN {
		t.Fatalf("forwarded keys = %v, want [N]", handler.keys)
	}
	if !handler.mods[0].AltPressed {
		t.Fatalf("forwarded mods = %+v, want AltPressed", handler.mods[0])
	}
}

func TestConflictNameEntryKeepsNonAltShortcuts(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	handler := &conflictEntryTestHandler{}
	km.PushHandler(handler)
	entry := newConflictNameEntry(km, nil)

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyL, Modifier: fyne.KeyModifierControl})

	if len(handler.keys) != 0 {
		t.Fatalf("non-Alt shortcut should stay with the entry, got %v", handler.keys)
	}
}

// TestConflictNameEntryTypedKeyUsesTrackedModifierState is a regression test
// for a bug where conflictNameEntry.TypedKey passed a hardcoded zero
// ModifierState to its lineHandler, so Shift+Left/Right/Home/End falsely
// matched the unmodified lineEdit bindings: the cursor moved but Fyne's
// native shift-selection in the TypedKey fallthrough never ran. TypedKey must
// use the KeyManager's tracked modifier state instead, which KeyDown/KeyUp
// already feed.
func TestConflictNameEntryTypedKeyUsesTrackedModifierState(t *testing.T) {
	// Shift+Left tracked via KeyDown must reach TypedKey's modifier check, so
	// the plain "Left" lineEdit binding does not falsely match: the event
	// should fall through to the base entry's native shift-selection instead
	// of the lineEdit cursor-left command.
	shiftKm := keymanager.NewKeyManager(func(string, ...interface{}) {})
	shiftEntry := newConflictNameEntry(shiftKm, nil)
	shiftEntry.SetText("abcd")
	shiftEntry.MoveCursorEnd()

	shiftEntry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	shiftEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})

	if shiftEntry.CursorColumn != 3 {
		t.Fatalf("cursor after shift+left = %d, want 3", shiftEntry.CursorColumn)
	}
	if got := shiftEntry.SelectedText(); got != "d" {
		t.Fatalf("selection after shift+left = %q, want %q (event should fall through to native selection, not the lineEdit cursor-left command)", got, "d")
	}

	// Without Shift tracked, the same key must still hit the lineEdit
	// cursor-left command and must not create a selection.
	plainKm := keymanager.NewKeyManager(func(string, ...interface{}) {})
	plainEntry := newConflictNameEntry(plainKm, nil)
	plainEntry.SetText("abcd")
	plainEntry.MoveCursorEnd()

	plainEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})

	if plainEntry.CursorColumn != 3 {
		t.Fatalf("cursor after plain left = %d, want 3", plainEntry.CursorColumn)
	}
	if got := plainEntry.SelectedText(); got != "" {
		t.Fatalf("selection after plain left = %q, want empty (lineEdit cursor-left command should intercept)", got)
	}
}

func TestConflictNameEntryUsesLineEditBindings(t *testing.T) {
	entry := newConflictNameEntry(nil, nil, []config.KeyBindingEntry{
		{Target: keymanager.KeyBindingTargetLineEdit, Key: "C-A", Command: keymanager.CommandNoop},
		{Target: keymanager.KeyBindingTargetLineEdit, Key: "C-E", Command: keymanager.CommandLineEditCursorStart},
	})
	entry.SetText("abcd")
	entry.MoveCursorEnd()

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierControl})
	if entry.CursorColumn != 4 {
		t.Fatalf("cursor after noop ctrl-a = %d, want 4", entry.CursorColumn)
	}

	entry.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyE, Modifier: fyne.KeyModifierControl})
	if entry.CursorColumn != 0 {
		t.Fatalf("cursor after configured ctrl-e = %d, want 0", entry.CursorColumn)
	}
}

func TestConflictNameEntryUsesLineEditCursorColor(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cfg := &config.Config{
		Theme: config.ThemeConfig{
			Colors: map[string]config.ThemeColorConfig{
				customtheme.ColorLineEditCursor: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{255, 255, 0, 255}, IsRGBA: true},
				},
			},
		},
	}
	appTheme := customtheme.NewCustomTheme(cfg, func(string, ...interface{}) {})
	app.Settings().SetTheme(appTheme)

	entry := newConflictNameEntry(nil, nil)
	w := test.NewWindow(lineEditThemeOverride(entry))
	defer w.Close()
	w.Canvas().Focus(entry)

	renderer := test.WidgetRenderer(entry).(*lineEditEntryRenderer)
	renderer.Refresh()
	border := renderer.borderRectangle()
	if border == nil {
		t.Fatal("conflict rename border rectangle was not found")
	}

	got := color.RGBAModel.Convert(border.StrokeColor)
	want := color.RGBAModel.Convert(appTheme.Color(fynetheme.ColorNamePrimary, app.Settings().ThemeVariant()))
	cursor := color.RGBAModel.Convert(appTheme.GetCustomColor(customtheme.ColorLineEditCursor))
	if got != want {
		t.Fatalf("focused border color = %#v, want app primary %#v", got, want)
	}
	if caret := color.RGBAModel.Convert(renderer.caret.FillColor); caret != cursor {
		t.Fatalf("conflict rename caret color = %#v, want cursor color %#v", caret, cursor)
	}
}
