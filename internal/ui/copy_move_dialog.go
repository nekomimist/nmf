package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
)

// Operation represents the requested action
type Operation string

const (
	OpCopy    Operation = "copy"
	OpMove    Operation = "move"
	OpExtract Operation = "extract"
)

// CopyMoveResult describes the accepted copy/move dialog choices.
type CopyMoveResult struct {
	Destination        string
	PreserveTimestamps bool
}

// CopyMoveDialog presents targets and lets user pick destination by filtering history
type CopyMoveDialog struct {
	destinationPicker
	op         Operation
	targets    []string
	preserveCB *widget.Check

	debugPrint func(format string, args ...interface{})
	keyManager *keymanager.KeyManager
	kmToken    keymanager.HandlerToken
	parent     fyne.Window
	dialog     dialog.Dialog
	sink       *KeySink
	closed     bool
	ownerFrame dialogHighlightState

	onAccept   func(CopyMoveResult)
	onOpenDest func(string)
	onClosed   func()
}

// NewCopyMoveDialog creates a new dialog instance
func NewCopyMoveDialog(
	op Operation,
	targets []string,
	destCandidates []DestinationCandidate,
	preserveTimestamps bool,
	km *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
	matchers ...*search.Provider,
) *CopyMoveDialog {
	d := &CopyMoveDialog{
		destinationPicker: destinationPicker{allDest: destCandidates, openDest: destinationOpenMap(destCandidates)},
		op:                op,
		targets:           targets,
		keyManager:        km,
		debugPrint:        debugPrint,
	}
	if op == OpCopy || op == OpExtract {
		d.preserveCB = widget.NewCheck("Preserve timestamps", nil)
		d.preserveCB.SetChecked(preserveTimestamps)
	}
	if len(matchers) > 0 {
		d.matchers = matchers[0]
	}
	d.createWidgets()
	d.updateFiltered("")
	return d
}

func (d *CopyMoveDialog) createWidgets() {
	d.destinationPicker.createWidgets(func() {
		if d.parent != nil && d.sink != nil {
			d.parent.Canvas().Focus(d.sink)
		}
	})
}

// ShowDialog renders and shows the copy/move dialog
func (d *CopyMoveDialog) ShowDialog(parent fyne.Window, onAccept func(CopyMoveResult)) {
	d.parent = parent
	d.onAccept = onAccept
	listWidth := responsiveDialogWidth(parent, searchDialogListWidth)
	targetListWidth := fyne.Max(copyMoveTargetListWidth, listWidth)

	// Title is derived dynamically when creating the dialog below

	// Targets summary
	count := len(d.targets)
	header := widget.NewLabel(fmt.Sprintf("%s %d item(s)", strings.Title(string(d.op)), count))
	header.TextStyle.Bold = true

	// Show up to 20 items, then elide
	maxShow := 20
	var toShow []string
	overflow := 0
	if count > maxShow {
		toShow = d.targets[:maxShow]
		overflow = count - maxShow
	} else {
		toShow = d.targets
	}
	targetsList := widget.NewList(
		func() int { return len(toShow) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			if i >= 0 && int(i) < len(toShow) {
				name := toShow[i]
				if label, ok := obj.(*widget.Label); ok {
					label.TextStyle = fyne.TextStyle{Monospace: true}
					label.SetText(name)
				}
			}
		},
	)
	targetsScroll := container.NewScroll(targetsList)
	targetsScroll.SetMinSize(metricsSize(targetListWidth, copyMoveTargetListHeight))
	overflowLabel := widget.NewLabel("")
	if overflow > 0 {
		overflowLabel.SetText(fmt.Sprintf("... and %d more", overflow))
	}

	// Destination search + list (fixed size like history dialog)
	searchLabel := widget.NewLabel("Destination:")
	openButton := widget.NewButtonWithIcon("Open", fynetheme.FolderNewIcon(), func() {
		d.OpenDestination()
	})
	searchSection := container.NewBorder(nil, nil, searchLabel, openButton, d.searchEntry)
	fixed := d.destinationArea(listWidth, copyMoveDestListHeight)

	contentObjects := []fyne.CanvasObject{
		header,
		targetsScroll,
		overflowLabel,
		widget.NewSeparator(),
		searchSection,
		fixed,
	}
	if d.preserveCB != nil {
		contentObjects = append(contentObjects, d.preserveCB)
	}
	contentObjects = append(contentObjects, dialogButtonBar(dialogCancelButton("Cancel", d.CancelDialog), dialogConfirmButton("OK", d.AcceptSelection)))
	content := container.NewVBox(contentObjects...)

	// Push key handler and wrap content with KeySink
	handler := keymanager.NewCopyMoveDialogKeyHandler(d, d.debugPrint)
	d.kmToken = d.keyManager.PushHandler(handler)
	d.sink = NewKeySink(content, d.keyManager, WithTabCapture(true))
	d.searchEntry.SetFocusRedirect(parent, d.sink)

	// Custom dialog without stock buttons (bar lives inside content)
	framedContent := d.ownerFrame.wrap(d.sink)
	d.dialog = dialog.NewCustomWithoutButtons(fmt.Sprintf("%s To...", strings.Title(string(d.op))), framedContent, parent)
	d.dialog.Show()
	if d.parent != nil && d.sink != nil {
		d.parent.Canvas().Focus(d.sink)
		d.searchEntry.RefreshIMEAnchor()
	}
}

// PreserveTimestamps reports whether accepted copy should preserve timestamps.
func (d *CopyMoveDialog) PreserveTimestamps() bool {
	return d.preserveCB != nil && d.preserveCB.Checked
}

// SetOwnerHighlighted changes the accent frame around the owning dialog.
func (d *CopyMoveDialog) SetOwnerHighlighted(highlighted bool) {
	d.ownerFrame.setHighlighted(highlighted)
}

// SetOnOpenDestination sets a callback for opening the currently selected destination.
func (d *CopyMoveDialog) SetOnOpenDestination(callback func(string)) {
	d.onOpenDest = callback
}

// SetOnClosed sets a callback fired after the dialog is accepted or canceled.
func (d *CopyMoveDialog) SetOnClosed(callback func()) {
	d.onClosed = callback
}

func (d *CopyMoveDialog) SelectCurrentItem() {
	d.debugPrint("CopyMoveDialog: Select current dest: %s", d.selectedPath)
}

func (d *CopyMoveDialog) OpenDestination() {
	if d.onOpenDest == nil {
		return
	}
	path := d.selectedPath
	if path == "" {
		if resolved, ok := d.resolveDirectoryPath(d.GetSearchText()); ok {
			path = resolved
		}
	}
	if path == "" {
		return
	}
	d.debugPrint("CopyMoveDialog: Open destination: %s", path)
	d.onOpenDest(path)
}

func (d *CopyMoveDialog) AcceptSelection() {
	d.accept(false)
}

func (d *CopyMoveDialog) AcceptDirectPath() {
	d.accept(true)
}

func (d *CopyMoveDialog) accept(direct bool) {
	if d.closed {
		return
	}
	d.closed = true
	acceptedPath := ""
	search := d.GetSearchText()
	if search != "" && (direct || len(d.filteredDest) == 0) {
		if resolvedPath, ok := d.resolveDirectoryPath(search); ok {
			d.debugPrint("CopyMoveDialog: direct path accept: %s", resolvedPath)
			acceptedPath = resolvedPath
		} else if d.selectedPath != "" {
			acceptedPath = d.selectedPath
		}
	} else if d.selectedPath != "" {
		acceptedPath = d.selectedPath
	}
	deferDialogClose(d.keyManager, "copyMove.accept", func() {
		d.notifyDialogClosed()
		d.keyManager.RemoveHandler(d.kmToken)
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink, d.searchEntry)
		if d.onAccept != nil && acceptedPath != "" {
			d.onAccept(CopyMoveResult{Destination: acceptedPath, PreserveTimestamps: d.PreserveTimestamps()})
		}
	})
}

func (d *CopyMoveDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.closed = true
	deferDialogClose(d.keyManager, "copyMove.cancel", func() {
		d.notifyDialogClosed()
		d.keyManager.RemoveHandler(d.kmToken)
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink, d.searchEntry)
	})
}

func (d *CopyMoveDialog) notifyDialogClosed() {
	if d.onPathChanged != nil {
		d.onPathChanged("")
	}
	if d.onClosed != nil {
		d.onClosed()
		d.onClosed = nil
	}
}

// Helpers
func (d *CopyMoveDialog) resolveDirectoryPath(p string) (string, bool) {
	resolved, _, err := fileinfo.CanonicalDisplayPath(p)
	if err != nil {
		d.debugPrint("CopyMoveDialog: Path is invalid: '%s' (%v)", p, err)
		return "", false
	}
	return resolved, true
}
