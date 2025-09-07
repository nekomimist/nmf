package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// Operation represents the requested action
type Operation string

const (
	OpCopy Operation = "copy"
	OpMove Operation = "move"
)

// CopyMoveDialog presents targets and lets user pick destination by filtering history
type CopyMoveDialog struct {
	op           Operation
	targets      []string
	searchEntry  *CustomSearchEntry
	destList     *widget.List
	filteredDest []string
	allDest      []string
	lastUsed     map[string]time.Time
	dataBinding  binding.StringList
	selectedPath string
	selectedIdx  int

	debugPrint func(format string, args ...interface{})
	keyManager *keymanager.KeyManager
	parent     fyne.Window
	dialog     dialog.Dialog
	sink       *KeySink
	closed     bool

	onAccept func(dest string)
}

// NewCopyMoveDialog creates a new dialog instance
func NewCopyMoveDialog(op Operation, targets []string, destCandidates []string, lastUsed map[string]time.Time, km *keymanager.KeyManager, debugPrint func(format string, args ...interface{})) *CopyMoveDialog {
	d := &CopyMoveDialog{
		op:         op,
		targets:    targets,
		allDest:    destCandidates,
		lastUsed:   lastUsed,
		keyManager: km,
		debugPrint: debugPrint,
	}
	d.createWidgets()
	d.updateFiltered("")
	return d
}

func (d *CopyMoveDialog) createWidgets() {
	d.searchEntry = NewCustomSearchEntry()
	d.searchEntry.SetPlaceHolder("Type to filter destination...")
	d.searchEntry.OnChanged = func(q string) { d.updateFiltered(q) }
	d.dataBinding = binding.NewStringList()
	d.destList = widget.NewListWithData(
		d.dataBinding,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(item binding.DataItem, obj fyne.CanvasObject) {
			str, _ := item.(binding.String).Get()
			if label, ok := obj.(*widget.Label); ok {
				label.SetText(str)
			}
		},
	)
	d.destList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && int(id) < len(d.filteredDest) {
			d.selectedIdx = int(id)
			d.selectedPath = d.filteredDest[id]
			if d.parent != nil && d.sink != nil {
				d.parent.Canvas().Focus(d.sink)
			}
		}
	}
}

// ShowDialog renders and shows the copy/move dialog
func (d *CopyMoveDialog) ShowDialog(parent fyne.Window, onAccept func(dest string)) {
	d.parent = parent
	d.onAccept = onAccept

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
					label.SetText(name)
				}
			}
		},
	)
	targetsScroll := container.NewScroll(targetsList)
	targetsScroll.SetMinSize(fyne.NewSize(500, 160))
	overflowLabel := widget.NewLabel("")
	if overflow > 0 {
		overflowLabel.SetText(fmt.Sprintf("... and %d more", overflow))
	}

	// Destination search + list (fixed size like history dialog)
	searchLabel := widget.NewLabel("Destination:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, d.searchEntry)
	destScroll := container.NewScroll(d.destList)
	destScroll.SetMinSize(fyne.NewSize(600, 260))
	empty := widget.NewLabel("No matching destinations")
	empty.Alignment = fyne.TextAlignCenter
	empty.Hide()
	fixed := container.NewWithoutLayout(destScroll, empty)
	fixed.Resize(fyne.NewSize(600, 260))
	destScroll.Resize(fyne.NewSize(600, 260))
	destScroll.Move(fyne.NewPos(0, 0))
	empty.Resize(fyne.NewSize(600, 260))
	empty.Move(fyne.NewPos(0, 0))
	d.searchEntry.OnChanged = func(q string) {
		d.updateFiltered(q)
		if len(d.filteredDest) == 0 {
			destScroll.Hide()
			empty.Show()
		} else {
			empty.Hide()
			destScroll.Show()
		}
	}

	content := container.NewVBox(
		header,
		targetsScroll,
		overflowLabel,
		widget.NewSeparator(),
		searchSection,
		fixed,
	)

	// Push key handler and wrap content with KeySink
	handler := keymanager.NewCopyMoveDialogKeyHandler(d, d.debugPrint)
	d.keyManager.PushHandler(handler)
	d.sink = NewKeySink(content, d.keyManager, WithTabCapture(true))
	d.searchEntry.SetFocusRedirect(parent, d.sink)

	// Custom confirm dialog
	d.dialog = dialog.NewCustomConfirm(
		fmt.Sprintf("%s To...", strings.Title(string(d.op))),
		"OK",
		"Cancel",
		d.sink,
		func(resp bool) {
			if resp {
				d.AcceptSelection()
			} else {
				d.CancelDialog()
			}
		},
		parent,
	)
	d.dialog.Show()
	if d.parent != nil && d.sink != nil {
		d.parent.Canvas().Focus(d.sink)
	}
}

// updateFiltered updates destination list
func (d *CopyMoveDialog) updateFiltered(q string) {
	if q == "" {
		d.filteredDest = d.allDest
	} else {
		ql := strings.ToLower(q)
		d.filteredDest = d.filteredDest[:0]
		for _, p := range d.allDest {
			if strings.Contains(strings.ToLower(p), ql) {
				d.filteredDest = append(d.filteredDest, p)
			}
		}
	}
	d.dataBinding.Set(d.filteredDest)
	if len(d.filteredDest) > 0 {
		d.selectedIdx = 0
		d.selectedPath = d.filteredDest[0]
		d.destList.Select(0)
	} else {
		d.selectedIdx = -1
		d.selectedPath = ""
	}
	d.destList.Refresh()
}

// Interface methods used by key handler
func (d *CopyMoveDialog) MoveUp() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		i := d.selectedIdx - 1
		if i < 0 {
			i = 0
		}
		if i != d.selectedIdx {
			d.destList.Select(widget.ListItemID(i))
		}
	}
}
func (d *CopyMoveDialog) MoveDown() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		i := d.selectedIdx + 1
		m := len(d.filteredDest) - 1
		if i > m {
			i = m
		}
		if i != d.selectedIdx {
			d.destList.Select(widget.ListItemID(i))
		}
	}
}
func (d *CopyMoveDialog) MoveToTop() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		d.destList.Select(0)
	}
}
func (d *CopyMoveDialog) MoveToBottom() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		d.destList.Select(len(d.filteredDest) - 1)
	}
}
func (d *CopyMoveDialog) ClearSearch() {
	if d.searchEntry != nil {
		d.searchEntry.SetText("")
	}
}
func (d *CopyMoveDialog) AppendToSearch(c string) {
	if d.searchEntry != nil {
		d.searchEntry.SetText(d.searchEntry.Text + c)
	}
}
func (d *CopyMoveDialog) BackspaceSearch() {
	if d.searchEntry != nil {
		t := d.searchEntry.Text
		if len(t) > 0 {
			d.searchEntry.SetText(t[:len(t)-1])
		}
	}
}
func (d *CopyMoveDialog) GetSearchText() string {
	if d.searchEntry != nil {
		return d.searchEntry.Text
	}
	return ""
}
func (d *CopyMoveDialog) CopySelectedPathToSearch() {
	if d.searchEntry != nil && d.selectedPath != "" {
		d.searchEntry.SetText(d.selectedPath)
	}
}
func (d *CopyMoveDialog) SelectCurrentItem() {
	d.debugPrint("CopyMoveDialog: Select current dest: %s", d.selectedPath)
}

func (d *CopyMoveDialog) AcceptSelection() {
	if d.closed {
		return
	}
	d.closed = true
	d.keyManager.PopHandler()

	// Allow direct path via search text when no list match
	search := d.GetSearchText()
	if search != "" && d.isValidDir(search) && len(d.filteredDest) == 0 {
		abs := d.absPath(search)
		d.debugPrint("CopyMoveDialog: direct path accept: %s", abs)
		if d.onAccept != nil {
			d.onAccept(abs)
		}
	} else if d.onAccept != nil && d.selectedPath != "" {
		d.onAccept(d.selectedPath)
	}
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
}

func (d *CopyMoveDialog) AcceptDirectPath() {
	if d.closed {
		return
	}
	d.closed = true
	d.keyManager.PopHandler()
	search := d.GetSearchText()
	if search != "" && d.isValidDir(search) {
		abs := d.absPath(search)
		d.debugPrint("CopyMoveDialog: Ctrl+Enter direct: %s", abs)
		if d.onAccept != nil {
			d.onAccept(abs)
		}
	} else if d.onAccept != nil && d.selectedPath != "" {
		d.debugPrint("CopyMoveDialog: invalid direct path; fallback to selection: %s", d.selectedPath)
		d.onAccept(d.selectedPath)
	}
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
}

func (d *CopyMoveDialog) CancelDialog() {
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

// Helpers
func (d *CopyMoveDialog) isValidDir(p string) bool {
	if strings.TrimSpace(p) == "" {
		return false
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	inf, err := os.Stat(abs)
	if err != nil {
		return false
	}
	return inf.IsDir()
}
func (d *CopyMoveDialog) absPath(p string) string {
	if a, e := filepath.Abs(p); e == nil {
		return a
	}
	return p
}
