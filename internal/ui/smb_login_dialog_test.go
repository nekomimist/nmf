package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	customtheme "nmf/internal/theme"
)

type smbLoginRecordingHandler struct {
	keys []fyne.KeyName
}

func (h *smbLoginRecordingHandler) OnKeyActivated(ev *fyne.KeyEvent, _ keymanager.ModifierState) bool {
	h.keys = append(h.keys, ev.Name)
	return true
}

func (h *smbLoginRecordingHandler) OnTypedRune(_ rune, _ keymanager.ModifierState) bool {
	return true
}

func (h *smbLoginRecordingHandler) GetName() string { return "recording" }

func TestSMBLoginDialogTabCyclesInputFieldsOnly(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(nil)
	defer w.Close()
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := newSMBLoginDialog("server", "share", w, km, nil, nil)
	d.show()
	defer func() {
		d.CancelDialog()
		fyne.DoAndWait(func() {})
	}()

	if got := w.Canvas().Focused(); got != d.username {
		t.Fatalf("initial focus = %T, want username entry", got)
	}

	typeEntryKey(d.username, fyne.KeyTab)
	if got := w.Canvas().Focused(); got != d.password {
		t.Fatalf("focus after tab = %T, want password entry", got)
	}

	typeEntryKey(d.password, fyne.KeyTab)
	if got := w.Canvas().Focused(); got != d.domain {
		t.Fatalf("focus after second tab = %T, want domain entry", got)
	}

	typeShiftEntryKey(d.domain, fyne.KeyTab)
	if got := w.Canvas().Focused(); got != d.password {
		t.Fatalf("focus after shift-tab = %T, want password entry", got)
	}
}

func TestSMBLoginDialogDoesNotFallThroughToUnderlyingHandler(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	underlying := &smbLoginRecordingHandler{}
	km.PushHandler(underlying)
	d := newSMBLoginDialog("server", "share", nil, km, nil, nil)
	handler := newSMBLoginKeyHandler(d, nil, km.Debugf)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyDown})
	if handled := km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyDown}); handled {
		t.Fatal("down key should not be handled by SMB login line-edit handler")
	}
	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyDown})

	if len(underlying.keys) != 0 {
		t.Fatalf("underlying handler saw keys: %v", underlying.keys)
	}
}

func TestSMBLoginDialogEnterAcceptsCredentials(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(nil)
	defer w.Close()
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	var accepted bool
	var got fileinfo.Credentials
	d := newSMBLoginDialog("server", "share", w, km, nil, func(ok bool, creds fileinfo.Credentials) {
		accepted = ok
		got = creds
	})
	d.show()
	d.domain.SetText("DOMAIN")
	d.username.SetText("alice")
	d.password.SetText("secret")
	d.saveCheck.SetChecked(true)

	typeEntryKey(d.username, fyne.KeyReturn)
	fyne.DoAndWait(func() {})

	if !accepted {
		t.Fatal("enter did not accept SMB login")
	}
	if got.Domain != "DOMAIN" || got.Username != "alice" || got.Password != "secret" || !got.Persist {
		t.Fatalf("credentials = %+v, want entered values", got)
	}
}

func TestSMBLoginDialogPasswordUsesLineEditEntry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(nil)
	defer w.Close()

	d := newSMBLoginDialog("server", "share", w, nil, nil, nil)
	if d.password == nil {
		t.Fatal("password entry is nil")
	}
	if !d.password.Password {
		t.Fatal("password entry should use password masking")
	}
}

func TestSMBLoginDialogEntriesUseLineEditThemeColors(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cfg := &config.Config{
		Theme: config.ThemeConfig{
			Colors: map[string]config.ThemeColorConfig{
				customtheme.ColorLineEditCursor: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{1, 2, 3, 255}, IsRGBA: true},
				},
				customtheme.ColorLineEditSelection: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{4, 5, 6, 255}, IsRGBA: true},
				},
			},
		},
	}
	appTheme := customtheme.NewCustomTheme(cfg, func(string, ...interface{}) {})
	app.Settings().SetTheme(appTheme)
	w := test.NewWindow(nil)
	defer w.Close()

	d := newSMBLoginDialog("server", "share", w, nil, nil, nil)
	for name, entry := range map[string]*LineEditEntry{
		"domain":   d.domain,
		"username": d.username,
		"password": d.password,
	} {
		w.SetContent(lineEditThemeOverride(entry))
		w.Canvas().Focus(entry)
		renderer := test.WidgetRenderer(entry).(*lineEditEntryRenderer)
		renderer.Refresh()

		wantCursor := color.RGBAModel.Convert(appTheme.GetCustomColor(customtheme.ColorLineEditCursor))
		if got := color.RGBAModel.Convert(renderer.caret.FillColor); got != wantCursor {
			t.Fatalf("%s caret = %#v, want %#v", name, got, wantCursor)
		}

		variant := app.Settings().ThemeVariant()
		wantSelection := color.RGBAModel.Convert(appTheme.GetCustomColor(customtheme.ColorLineEditSelection))
		if got := color.RGBAModel.Convert(entry.Theme().Color(fynetheme.ColorNameSelection, variant)); got != wantSelection {
			t.Fatalf("%s selection = %#v, want %#v", name, got, wantSelection)
		}
	}
}

func typeEntryKey(entry *LineEditEntry, key fyne.KeyName) {
	entry.KeyDown(&fyne.KeyEvent{Name: key})
	entry.TypedKey(&fyne.KeyEvent{Name: key})
	entry.KeyUp(&fyne.KeyEvent{Name: key})
}

func typeShiftEntryKey(entry *LineEditEntry, key fyne.KeyName) {
	entry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	entry.KeyDown(&fyne.KeyEvent{Name: key})
	entry.TypedKey(&fyne.KeyEvent{Name: key})
	entry.KeyUp(&fyne.KeyEvent{Name: key})
	entry.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
}
