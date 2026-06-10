package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/keymanager"
)

const (
	conflictDisplayedPathMax = 72

	conflictOverwriteIfNewerLabel = "Overwrite if newer (Alt+N)"
	conflictOverwriteLabel        = "Overwrite (Alt+O)"
	conflictSkipLabel             = "Skip this item (Alt+S)"
	conflictRenameLabel           = "Rename to (Alt+R):"
)

// ConflictDialog resolves one copy/move destination name collision.
type ConflictDialog struct {
	req        jobs.ConflictRequest
	km         *keymanager.KeyManager
	parent     fyne.Window
	dialog     dialog.Dialog
	closed     bool
	choice     *widget.RadioGroup
	nameEntry  *conflictNameEntry
	applyRest  *widget.Check
	errorLabel *widget.Label
	sink       *KeySink
	shortcuts  []*desktop.CustomShortcut
	callback   func(jobs.ConflictResolution)
}

// NewConflictDialog creates a dialog for one name collision.
func NewConflictDialog(req jobs.ConflictRequest, km *keymanager.KeyManager) *ConflictDialog {
	return &ConflictDialog{req: req, km: km}
}

// ShowDialog displays the conflict dialog.
func (d *ConflictDialog) ShowDialog(parent fyne.Window, callback func(jobs.ConflictResolution)) {
	d.parent = parent
	d.callback = callback

	source := widget.NewLabel(middleEllipsizeFileName(d.req.SourcePath, conflictDisplayedPathMax))
	source.TextStyle = fyne.TextStyle{Monospace: true}
	source.Wrapping = fyne.TextWrapWord
	dest := widget.NewLabel(middleEllipsizeFileName(d.req.Destination, conflictDisplayedPathMax))
	dest.TextStyle = fyne.TextStyle{Monospace: true}
	dest.Wrapping = fyne.TextWrapWord
	sourceModified := widget.NewLabel(formatConflictModified(d.req.SourceModified))
	destModified := widget.NewLabel(formatConflictModified(d.req.DestModified))
	suggested := d.req.SuggestedName
	if strings.TrimSpace(suggested) == "" {
		suggested = "auto name"
	}

	options := []string{
		conflictOverwriteIfNewerLabel,
		conflictOverwriteLabel,
		fmt.Sprintf("Auto name (Alt+A): %s", suggested),
		conflictSkipLabel,
		conflictRenameLabel,
	}
	d.choice = widget.NewRadioGroup(options, func(string) {
		d.updateEntryState()
		if d.choice != nil && strings.HasPrefix(d.choice.Selected, "Rename") {
			d.focusCurrent()
		}
	})
	d.choice.Required = true
	switch d.req.DefaultAction {
	case jobs.ConflictOverwriteIfNewer:
		d.choice.SetSelected(options[0])
	case jobs.ConflictOverwrite:
		d.choice.SetSelected(options[1])
	case jobs.ConflictAutoSuffix:
		d.choice.SetSelected(options[2])
	case jobs.ConflictRename:
		d.choice.SetSelected(options[4])
	case jobs.ConflictSkip:
		d.choice.SetSelected(options[3])
	default:
		d.choice.SetSelected(options[0])
	}

	d.nameEntry = newConflictNameEntry(d.km, d.CancelJob)
	d.nameEntry.SetIMEWindow(parent)
	d.nameEntry.SetText(suggested)
	d.nameEntry.OnSubmitted = func(string) {
		d.Continue()
	}
	d.applyRest = widget.NewCheck("Use this choice for remaining conflicts in this job", nil)
	d.errorLabel = widget.NewLabel("")
	d.errorLabel.Wrapping = fyne.TextWrapWord
	d.errorLabel.Hide()
	d.updateEntryState()

	content := container.NewVBox(
		widget.NewLabel("A destination item with the same name already exists."),
		container.NewBorder(nil, nil, widget.NewLabel("Source:"), nil, source),
		container.NewBorder(nil, nil, widget.NewLabel("Modified:"), nil, sourceModified),
		container.NewBorder(nil, nil, widget.NewLabel("Target:"), nil, dest),
		container.NewBorder(nil, nil, widget.NewLabel("Modified:"), nil, destModified),
		widget.NewLabel(""),
		container.NewPadded(d.choice),
		lineEditThemeOverride(d.nameEntry),
		widget.NewLabel(""),
		d.applyRest,
		d.errorLabel,
		widget.NewLabel(""),
	)

	handler := keymanager.NewConflictDialogKeyHandler(d)
	if d.km != nil {
		d.km.PushHandler(handler)
		d.sink = NewKeySink(content, d.km, WithTabCapture(false))
	}
	dialogContent := fyne.CanvasObject(content)
	if d.sink != nil {
		dialogContent = d.sink
	}

	d.dialog = dialog.NewCustomConfirm(
		"Name conflict",
		"Continue",
		"Cancel Job",
		dialogContent,
		func(ok bool) {
			if ok {
				d.Continue()
			} else {
				d.CancelJob()
			}
		},
		parent,
	)
	d.dialog.SetOnClosed(func() {
		d.CancelJob()
	})
	d.dialog.Show()
	d.registerShortcuts()
	d.dialog.Resize(metricsSize(conflictDialogWidth, 0))
	d.focusCurrent()
}

func formatConflictModified(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// Continue accepts the current conflict choice.
func (d *ConflictDialog) Continue() {
	if d.closed {
		return
	}
	selected := ""
	if d.choice != nil {
		selected = d.choice.Selected
	}
	res := jobs.ConflictResolution{Action: jobs.ConflictAutoSuffix}
	if d.applyRest != nil {
		res.ApplyToRest = d.applyRest.Checked
	}

	switch {
	case strings.HasPrefix(selected, "Overwrite if newer"):
		res.Action = jobs.ConflictOverwriteIfNewer
	case strings.HasPrefix(selected, "Overwrite"):
		res.Action = jobs.ConflictOverwrite
	case strings.HasPrefix(selected, "Rename"):
		name := ""
		if d.nameEntry != nil {
			name = d.nameEntry.Text
		}
		if _, err := fileinfo.ValidateRenameName(name); err != nil {
			d.showError(err.Error())
			d.focusCurrent()
			return
		}
		res.Action = jobs.ConflictRename
		res.NewName = strings.TrimSpace(name)
	case strings.HasPrefix(selected, "Skip"):
		res.Action = jobs.ConflictSkip
	default:
		res.Action = jobs.ConflictAutoSuffix
	}

	d.finish(res)
}

// CancelJob cancels the whole running job.
func (d *ConflictDialog) CancelJob() {
	if d.closed {
		return
	}
	d.finish(jobs.ConflictResolution{Action: jobs.ConflictCancelJob})
}

// SelectOverwriteIfNewer selects conditional overwrite.
func (d *ConflictDialog) SelectOverwriteIfNewer() {
	d.selectChoiceExact(conflictOverwriteIfNewerLabel)
}

// SelectOverwrite selects unconditional overwrite.
func (d *ConflictDialog) SelectOverwrite() {
	d.selectChoiceExact(conflictOverwriteLabel)
}

// SelectAutoName selects automatic suffix naming.
func (d *ConflictDialog) SelectAutoName() {
	d.selectChoiceByPrefix("Auto name")
}

// SelectRename selects manual rename and focuses the name entry.
func (d *ConflictDialog) SelectRename() {
	d.selectChoiceByPrefix("Rename")
	d.focusCurrent()
}

// SelectSkip selects skipping this item.
func (d *ConflictDialog) SelectSkip() {
	d.selectChoiceByPrefix("Skip")
}

func (d *ConflictDialog) finish(res jobs.ConflictResolution) {
	if d.closed {
		return
	}
	d.closed = true
	d.unregisterShortcuts()
	deferDialogClose(d.km, "conflict.close", func() {
		if d.km != nil {
			d.km.PopHandler()
		}
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink, d.nameEntry)
		if d.callback != nil {
			d.callback(res)
		}
	})
}

func (d *ConflictDialog) updateEntryState() {
	if d.nameEntry == nil || d.choice == nil {
		return
	}
	if strings.HasPrefix(d.choice.Selected, "Rename") {
		d.nameEntry.Enable()
	} else {
		d.nameEntry.Disable()
	}
}

func (d *ConflictDialog) selectChoiceByPrefix(prefix string) {
	if d.choice == nil {
		return
	}
	for _, option := range d.choice.Options {
		if strings.HasPrefix(option, prefix) {
			d.choice.SetSelected(option)
			d.updateEntryState()
			return
		}
	}
}

func (d *ConflictDialog) selectChoiceExact(label string) {
	if d.choice == nil {
		return
	}
	for _, option := range d.choice.Options {
		if option == label {
			d.choice.SetSelected(option)
			d.updateEntryState()
			return
		}
	}
}

func (d *ConflictDialog) registerShortcuts() {
	if d.parent == nil {
		return
	}
	c := d.parent.Canvas()
	d.shortcuts = []*desktop.CustomShortcut{
		{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierAlt},
		{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierAlt},
		{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierAlt},
		{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierAlt},
		{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierAlt},
	}
	c.AddShortcut(d.shortcuts[0], func(fyne.Shortcut) { d.SelectOverwriteIfNewer() })
	c.AddShortcut(d.shortcuts[1], func(fyne.Shortcut) { d.SelectOverwrite() })
	c.AddShortcut(d.shortcuts[2], func(fyne.Shortcut) { d.SelectAutoName() })
	c.AddShortcut(d.shortcuts[3], func(fyne.Shortcut) { d.SelectRename() })
	c.AddShortcut(d.shortcuts[4], func(fyne.Shortcut) { d.SelectSkip() })
}

func (d *ConflictDialog) unregisterShortcuts() {
	if d.parent == nil {
		return
	}
	c := d.parent.Canvas()
	for _, shortcut := range d.shortcuts {
		c.RemoveShortcut(shortcut)
	}
	d.shortcuts = nil
}

func (d *ConflictDialog) showError(message string) {
	if d.errorLabel == nil {
		return
	}
	d.errorLabel.SetText(message)
	d.errorLabel.Show()
}

func (d *ConflictDialog) focusCurrent() {
	if d.parent == nil {
		return
	}
	if d.choice != nil && strings.HasPrefix(d.choice.Selected, "Rename") && d.nameEntry != nil {
		d.parent.Canvas().Focus(d.nameEntry)
		return
	}
	if d.sink != nil {
		d.parent.Canvas().Focus(d.sink)
	}
}

type conflictNameEntry struct {
	TabEntry
	km        *keymanager.KeyManager
	onCancel  func()
	imeWindow fyne.Window
	focused   bool
	disabled  bool
}

func newConflictNameEntry(km *keymanager.KeyManager, onCancel func()) *conflictNameEntry {
	e := &conflictNameEntry{km: km, onCancel: onCancel}
	e.acceptTab = true
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

func (e *conflictNameEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		if e.onCancel != nil {
			e.onCancel()
		}
		return
	}
	e.TabEntry.TypedKey(ev)
	e.UpdateIMEAnchor()
}

func (e *conflictNameEntry) TypedRune(r rune) {
	e.TabEntry.TypedRune(r)
	e.UpdateIMEAnchor()
}

func (e *conflictNameEntry) KeyDown(ev *fyne.KeyEvent) {
	if e.km != nil {
		e.km.HandleKeyDown(ev)
	}
	e.TabEntry.KeyDown(ev)
}

func (e *conflictNameEntry) KeyUp(ev *fyne.KeyEvent) {
	if e.km != nil {
		e.km.HandleKeyUp(ev)
	}
	e.TabEntry.KeyUp(ev)
}

func (e *conflictNameEntry) FocusGained() {
	e.focused = true
	e.TabEntry.FocusGained()
	e.UpdateIMEAnchor()
}

func (e *conflictNameEntry) FocusLost() {
	e.focused = false
	e.TabEntry.FocusLost()
}

func (e *conflictNameEntry) Disable() {
	e.disabled = true
	e.TabEntry.Disable()
}

func (e *conflictNameEntry) Enable() {
	e.disabled = false
	e.TabEntry.Enable()
}

func (e *conflictNameEntry) SetIMEWindow(window fyne.Window) {
	e.imeWindow = window
	e.UpdateIMEAnchor()
}

func (e *conflictNameEntry) SetText(text string) {
	e.TabEntry.SetText(text)
	e.UpdateIMEAnchor()
}

func (e *conflictNameEntry) UpdateIMEAnchor() {
	if e.disabled {
		return
	}
	setIMEAnchorAtTextEnd(e.imeWindow, e, e.Text, e.TextStyle)
}

func (e *conflictNameEntry) CreateRenderer() fyne.WidgetRenderer {
	caret := canvas.NewRectangle(color.Transparent)
	caret.Hide()
	return &lineEditEntryRenderer{
		entry: e,
		base:  e.TabEntry.CreateRenderer(),
		caret: caret,
	}
}

func (e *conflictNameEntry) lineEditFocused() bool {
	return e.focused
}

func (e *conflictNameEntry) lineEditDisabled() bool {
	return e.disabled
}

func (e *conflictNameEntry) lineEditTextStyle() fyne.TextStyle {
	return e.TextStyle
}
