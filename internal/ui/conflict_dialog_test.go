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
