package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/keymanager"
)

const (
	conflictDialogWidth      float32 = 620
	conflictDisplayedPathMax         = 72
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
	source.Wrapping = fyne.TextWrapWord
	dest := widget.NewLabel(middleEllipsizeFileName(d.req.Destination, conflictDisplayedPathMax))
	dest.Wrapping = fyne.TextWrapWord
	suggested := d.req.SuggestedName
	if strings.TrimSpace(suggested) == "" {
		suggested = "auto name"
	}

	options := []string{
		fmt.Sprintf("Auto name (Alt+A): %s", suggested),
		"Skip this item (Alt+S)",
		"Rename to (Alt+R):",
	}
	d.choice = widget.NewRadioGroup(options, func(string) {
		d.updateEntryState()
		if d.choice != nil && strings.HasPrefix(d.choice.Selected, "Rename") {
			d.focusCurrent()
		}
	})
	switch d.req.DefaultAction {
	case jobs.ConflictRename:
		d.choice.SetSelected(options[2])
	case jobs.ConflictSkip:
		d.choice.SetSelected(options[1])
	default:
		d.choice.SetSelected(options[0])
	}

	d.nameEntry = newConflictNameEntry(d.CancelJob)
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
		container.NewBorder(nil, nil, widget.NewLabel("Target:"), nil, dest),
		widget.NewLabel(""),
		container.NewPadded(d.choice),
		d.nameEntry,
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
	d.dialog.Resize(fyne.NewSize(conflictDialogWidth, 0))
	d.focusCurrent()
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
	if d.km != nil {
		d.km.PopHandler()
	}
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
	if d.callback != nil {
		d.callback(res)
	}
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

func (d *ConflictDialog) registerShortcuts() {
	if d.parent == nil {
		return
	}
	c := d.parent.Canvas()
	d.shortcuts = []*desktop.CustomShortcut{
		{KeyName: fyne.KeyA, Modifier: fyne.KeyModifierAlt},
		{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierAlt},
		{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierAlt},
	}
	c.AddShortcut(d.shortcuts[0], func(fyne.Shortcut) { d.SelectAutoName() })
	c.AddShortcut(d.shortcuts[1], func(fyne.Shortcut) { d.SelectRename() })
	c.AddShortcut(d.shortcuts[2], func(fyne.Shortcut) { d.SelectSkip() })
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
	onCancel func()
}

func newConflictNameEntry(onCancel func()) *conflictNameEntry {
	e := &conflictNameEntry{onCancel: onCancel}
	e.acceptTab = true
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
}
