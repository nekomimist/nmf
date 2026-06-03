package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

const deleteConfirmWord = "DELETE"

// DeleteConfirmDialog confirms trash or permanent deletion for one or more items.
type DeleteConfirmDialog struct {
	targets    []string
	permanent  bool
	entry      *deleteConfirmEntry
	keyManager *keymanager.KeyManager
	parent     fyne.Window
	dialog     dialog.Dialog
	closed     bool
	onAccept   func()
}

func NewDeleteConfirmDialog(targets []string, permanent bool, km *keymanager.KeyManager) *DeleteConfirmDialog {
	d := &DeleteConfirmDialog{
		targets:    append([]string(nil), targets...),
		permanent:  permanent,
		keyManager: km,
	}
	if permanent {
		d.entry = newDeleteConfirmEntry(d.CancelDelete)
		d.entry.OnSubmitted = func(_ string) {
			d.ConfirmDelete()
		}
	}
	return d
}

func (d *DeleteConfirmDialog) ShowDialog(parent fyne.Window, onAccept func()) {
	d.parent = parent
	d.onAccept = onAccept

	title := "Move to Trash"
	action := "Trash"
	if d.permanent {
		title = "Permanently Delete"
		action = "Delete"
	}

	content := container.NewVBox(
		widget.NewLabel(d.message()),
		d.targetList(),
	)
	if d.permanent {
		content.Add(widget.NewLabel(fmt.Sprintf("Type %s to confirm:", deleteConfirmWord)))
		content.Add(d.entry)
	}
	content.Add(dialogButtonRow("Cancel", d.CancelDelete, action, d.ConfirmDelete))

	handler := keymanager.NewDeleteConfirmDialogKeyHandler(d)
	d.keyManager.PushHandler(handler)

	d.dialog = dialog.NewCustomWithoutButtons(title, content, parent)
	d.dialog.SetOnClosed(func() {
		d.CancelDelete()
	})
	d.dialog.Show()
	if d.permanent && d.parent != nil && d.entry != nil {
		d.entry.SetIMEWindow(d.parent)
		d.parent.Canvas().Focus(d.entry)
	}
}

func (d *DeleteConfirmDialog) message() string {
	count := len(d.targets)
	if d.permanent {
		return fmt.Sprintf("Permanently delete %d item(s)? This cannot be undone.", count)
	}
	return fmt.Sprintf("Move %d item(s) to Trash?", count)
}

func (d *DeleteConfirmDialog) targetList() fyne.CanvasObject {
	label := widget.NewLabel(d.targetSummary())
	label.Wrapping = fyne.TextWrapOff
	scroll := container.NewScroll(label)
	scroll.SetMinSize(metricsSize(deleteDialogWidth-40, deleteTargetListHeight))
	return scroll
}

func (d *DeleteConfirmDialog) targetSummary() string {
	if len(d.targets) == 0 {
		return ""
	}
	var lines []string
	for _, target := range d.targets {
		lines = append(lines, "- "+middleEllipsizeFileName(target, renameDisplayedNameMax))
	}
	return strings.Join(lines, "\n")
}

func (d *DeleteConfirmDialog) ConfirmDelete() {
	if d.closed {
		return
	}
	if d.permanent {
		if d.entry == nil || strings.TrimSpace(d.entry.Text) != deleteConfirmWord {
			if d.parent != nil && d.entry != nil {
				d.parent.Canvas().Focus(d.entry)
			}
			return
		}
	}
	d.closed = true
	deferDialogClose(d.keyManager, "delete.confirm", func() {
		d.keyManager.PopHandler()
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.entry)
		if d.onAccept != nil {
			d.onAccept()
		}
	})
}

func (d *DeleteConfirmDialog) CancelDelete() {
	if d.closed {
		return
	}
	d.closed = true
	deferDialogClose(d.keyManager, "delete.cancel", func() {
		d.keyManager.PopHandler()
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.entry)
	})
}

type deleteConfirmEntry struct {
	TabEntry
	onCancel  func()
	imeWindow fyne.Window
}

func newDeleteConfirmEntry(onCancel func()) *deleteConfirmEntry {
	e := &deleteConfirmEntry{onCancel: onCancel}
	e.acceptTab = true
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

func (e *deleteConfirmEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyEscape {
		if e.onCancel != nil {
			e.onCancel()
		}
		return
	}
	e.TabEntry.TypedKey(ev)
	e.UpdateIMEAnchor()
}

func (e *deleteConfirmEntry) TypedRune(r rune) {
	e.TabEntry.TypedRune(r)
	e.UpdateIMEAnchor()
}

func (e *deleteConfirmEntry) FocusGained() {
	e.TabEntry.FocusGained()
	e.UpdateIMEAnchor()
}

func (e *deleteConfirmEntry) SetIMEWindow(window fyne.Window) {
	e.imeWindow = window
	e.UpdateIMEAnchor()
}

func (e *deleteConfirmEntry) SetText(text string) {
	e.TabEntry.SetText(text)
	e.UpdateIMEAnchor()
}

func (e *deleteConfirmEntry) UpdateIMEAnchor() {
	setIMEAnchorAtTextEnd(e.imeWindow, e, e.Text, e.TextStyle)
}
