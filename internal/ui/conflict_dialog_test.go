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

type conflictEntryTestHandler struct{}

func (conflictEntryTestHandler) OnKeyDown(_ *fyne.KeyEvent, _ keymanager.ModifierState) bool {
	return false
}
func (conflictEntryTestHandler) OnKeyUp(_ *fyne.KeyEvent, _ keymanager.ModifierState) bool {
	return false
}
func (conflictEntryTestHandler) OnTypedKey(_ *fyne.KeyEvent, _ keymanager.ModifierState) bool {
	return false
}
func (conflictEntryTestHandler) OnTypedRune(_ rune, _ keymanager.ModifierState) bool {
	return false
}
func (conflictEntryTestHandler) GetName() string { return "test" }

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

func TestConflictNameEntryKeyUpClearsShortcutKeys(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	km.PushHandler(conflictEntryTestHandler{})
	entry := newConflictNameEntry(km, nil)

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyAltLeft})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyR})

	entry.KeyUp(&fyne.KeyEvent{Name: fyne.KeyR})
	entry.KeyUp(&fyne.KeyEvent{Name: desktop.KeyAltLeft})

	closed := false
	km.DeferUntilKeysReleased("test.close", func() {
		closed = true
	})

	if !closed {
		t.Fatal("deferred close should run after focused entry releases Alt+R")
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
