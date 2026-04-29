package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

const (
	renameDialogWidth      float32 = 480
	renameDialogHeight     float32 = 150
	renameDisplayedNameMax         = 56
)

// RenameDialog edits a single filename.
type RenameDialog struct {
	oldName    string
	entry      *renameEntry
	keyManager *keymanager.KeyManager
	parent     fyne.Window
	dialog     dialog.Dialog
	closed     bool

	onAccept func(string) bool
}

// NewRenameDialog creates a rename dialog for one item.
func NewRenameDialog(oldName string, km *keymanager.KeyManager) *RenameDialog {
	d := &RenameDialog{
		oldName:    oldName,
		keyManager: km,
	}
	d.entry = newRenameEntry(d.CancelDialog)
	d.entry.SetText(oldName)
	d.entry.OnSubmitted = func(_ string) {
		d.AcceptRename()
	}
	return d
}

// ShowDialog displays the rename dialog.
func (d *RenameDialog) ShowDialog(parent fyne.Window, onAccept func(string) bool) {
	d.parent = parent
	d.onAccept = onAccept

	currentLabel := widget.NewLabel("Current:")
	currentName := widget.NewLabel(middleEllipsizeFileName(d.oldName, renameDisplayedNameMax))
	currentName.Truncation = fyne.TextTruncateClip
	nameLabel := widget.NewLabel("New name:")

	content := container.NewVBox(
		container.NewBorder(nil, nil, currentLabel, nil, currentName),
		nameLabel,
		d.entry,
		container.NewGridWithColumns(
			2,
			widget.NewButton("Cancel", d.CancelDialog),
			widget.NewButton("Rename", d.AcceptRename),
		),
	)

	handler := keymanager.NewRenameDialogKeyHandler(d)
	d.keyManager.PushHandler(handler)

	d.dialog = dialog.NewCustomWithoutButtons(
		"Rename",
		content,
		parent,
	)
	d.dialog.SetOnClosed(func() {
		d.CancelDialog()
	})
	d.dialog.Show()
	d.dialog.Resize(fyne.NewSize(renameDialogWidth, renameDialogHeight))
	if d.parent != nil && d.entry != nil {
		d.parent.Canvas().Focus(d.entry)
	}
}

type renameEntry struct {
	TabEntry
	onCancel func()
}

func newRenameEntry(onCancel func()) *renameEntry {
	e := &renameEntry{onCancel: onCancel}
	e.acceptTab = true
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

func (e *renameEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		if e.onCancel != nil {
			e.onCancel()
		}
		return
	}
	e.TabEntry.TypedKey(ev)
}

// AcceptRename commits the entered filename.
func (d *RenameDialog) AcceptRename() {
	if d.closed {
		return
	}

	if d.onAccept != nil && d.entry != nil {
		if !d.onAccept(d.entry.Text) {
			if d.parent != nil && d.entry != nil {
				d.parent.Canvas().Focus(d.entry)
			}
			return
		}
	}
	d.closed = true
	d.keyManager.PopHandler()
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
}

func middleEllipsizeFileName(name string, maxRunes int) string {
	runes := []rune(name)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return name
	}
	const marker = "..."
	markerLen := len([]rune(marker))
	if maxRunes <= markerLen {
		return string(runes[:maxRunes])
	}

	available := maxRunes - markerLen
	prefixLen := available / 3
	if prefixLen < 1 {
		prefixLen = 1
	}
	suffixLen := available - prefixLen
	return string(runes[:prefixLen]) + marker + string(runes[len(runes)-suffixLen:])
}

// CancelDialog closes the dialog without changing anything.
func (d *RenameDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.closed = true
	d.keyManager.PopHandler()
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
}
