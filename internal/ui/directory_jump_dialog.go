package ui

import (
	"sort"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/keymanager"
)

// DirectoryJumpDialog shows manually configured directory jump targets.
type DirectoryJumpDialog struct {
	searchEntry     *CustomSearchEntry
	jumpList        *widget.List
	allEntries      []config.DirectoryJumpEntry
	filteredEntries []config.DirectoryJumpEntry
	selectedIndex   int
	selectedPath    string
	debugPrint      func(format string, args ...interface{})
	keyManager      *keymanager.KeyManager
	kmToken         keymanager.HandlerToken
	dialog          dialog.Dialog
	callback        func(string)
	parent          fyne.Window
	closed          bool
	sink            *KeySink

	pendingAutoAcceptPath string
	autoAcceptScheduled   bool
}

// NewDirectoryJumpDialog creates a configured directory jump dialog.
func NewDirectoryJumpDialog(
	entries []config.DirectoryJumpEntry,
	keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
) *DirectoryJumpDialog {
	d := &DirectoryJumpDialog{
		allEntries: sortDirectoryJumpEntries(copyDirectoryJumpEntries(entries)),
		debugPrint: debugPrint,
		keyManager: keyManager,
	}
	d.createWidgets()
	d.updateFilteredEntries("")
	return d
}

func copyDirectoryJumpEntries(entries []config.DirectoryJumpEntry) []config.DirectoryJumpEntry {
	copied := make([]config.DirectoryJumpEntry, len(entries))
	copy(copied, entries)
	return copied
}

func sortDirectoryJumpEntries(entries []config.DirectoryJumpEntry) []config.DirectoryJumpEntry {
	sort.SliceStable(entries, func(i, j int) bool {
		leftShortcut := NormalizeDirectoryJumpShortcut(entries[i].Shortcut)
		rightShortcut := NormalizeDirectoryJumpShortcut(entries[j].Shortcut)
		if leftShortcut == "" || rightShortcut == "" {
			return leftShortcut != "" && rightShortcut == ""
		}
		leftLen := utf8.RuneCountInString(leftShortcut)
		rightLen := utf8.RuneCountInString(rightShortcut)
		if leftLen != rightLen {
			return leftLen < rightLen
		}
		if leftShortcut != rightShortcut {
			return leftShortcut < rightShortcut
		}
		return strings.ToLower(entries[i].Directory) < strings.ToLower(entries[j].Directory)
	})
	return entries
}

func filterDirectoryJumpEntries(entries []config.DirectoryJumpEntry, query string) []config.DirectoryJumpEntry {
	needle := NormalizeDirectoryJumpShortcut(query)
	if needle == "" {
		return entries
	}

	filtered := []config.DirectoryJumpEntry{}
	for _, entry := range entries {
		shortcut := NormalizeDirectoryJumpShortcut(entry.Shortcut)
		if shortcut != "" && strings.HasPrefix(shortcut, needle) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// NormalizeDirectoryJumpShortcut normalizes configured and typed shortcut keys.
func NormalizeDirectoryJumpShortcut(shortcut string) string {
	return strings.ToLower(strings.TrimSpace(shortcut))
}

func (d *DirectoryJumpDialog) createWidgets() {
	d.searchEntry = NewCustomSearchEntry()
	d.searchEntry.SetPlaceHolder("Type shortcut prefix...")
	d.searchEntry.OnChanged = func(query string) {
		d.updateFilteredEntries(query)
	}

	d.jumpList = widget.NewList(
		func() int {
			return len(d.filteredEntries)
		},
		func() fyne.CanvasObject {
			shortcut := widget.NewLabel("")
			shortcut.TextStyle = fyne.TextStyle{Monospace: true}
			shortcut.Wrapping = fyne.TextTruncate
			shortcutBox := container.NewGridWrap(directoryJumpShortcutCellSize(), shortcut)
			path := widget.NewLabel("")
			path.TextStyle = fyne.TextStyle{Monospace: true}
			return container.NewHBox(shortcutBox, path)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || int(id) >= len(d.filteredEntries) {
				return
			}
			entry := d.filteredEntries[id]
			row, ok := obj.(*fyne.Container)
			if !ok || len(row.Objects) < 2 {
				return
			}
			shortcutBox, _ := row.Objects[0].(*fyne.Container)
			pathLabel, _ := row.Objects[1].(*widget.Label)
			if shortcutBox != nil && len(shortcutBox.Objects) > 0 {
				if shortcutLabel, ok := shortcutBox.Objects[0].(*widget.Label); ok {
					shortcutLabel.SetText(entry.Shortcut)
				}
			}
			if pathLabel != nil {
				pathLabel.SetText(entry.Directory)
			}
		},
	)

	d.jumpList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && int(id) < len(d.filteredEntries) {
			d.selectedIndex = int(id)
			d.selectedPath = d.filteredEntries[id].Directory
			d.debugPrint("DirectoryJumpDialog: selected %s index=%d", d.selectedPath, d.selectedIndex)
			if d.parent != nil && d.sink != nil {
				d.parent.Canvas().Focus(d.sink)
			}
		}
	}
	d.jumpList.Resize(searchDialogListSize())
}

func directoryJumpShortcutCellSize() fyne.Size {
	appTheme := fyne.CurrentApp().Settings().Theme()
	textSize := appTheme.Size(theme.SizeNameText)
	padding := appTheme.Size(theme.SizeNamePadding)
	innerPadding := appTheme.Size(theme.SizeNameInnerPadding)

	return fyne.NewSize(textSize*6+padding*2, textSize+innerPadding*2)
}

func directoryJumpListWidth(entries []config.DirectoryJumpEntry, minimum float32) float32 {
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = entry.Directory
	}
	width := dialogTextWidth(paths, minimum) + directoryJumpShortcutCellSize().Width
	if width < minimum {
		return minimum
	}
	return width
}

func (d *DirectoryJumpDialog) updateFilteredEntries(query string) {
	d.filteredEntries = filterDirectoryJumpEntries(d.allEntries, query)

	d.jumpList.Refresh()

	if len(d.filteredEntries) > 0 {
		d.selectedIndex = 0
		d.selectedPath = d.filteredEntries[0].Directory
		d.jumpList.Select(0)
	} else {
		d.selectedIndex = -1
		d.selectedPath = ""
	}

	if NormalizeDirectoryJumpShortcut(query) != "" && len(d.filteredEntries) == 1 {
		d.debugPrint("DirectoryJumpDialog: unique shortcut match path=%s", d.filteredEntries[0].Directory)
		d.pendingAutoAcceptPath = d.filteredEntries[0].Directory
		d.autoAcceptScheduled = false
	} else {
		d.pendingAutoAcceptPath = ""
		d.autoAcceptScheduled = false
	}
}

// ShowDialog shows the configured directory jump dialog.
func (d *DirectoryJumpDialog) ShowDialog(parent fyne.Window, callback func(string)) {
	titleLabel := widget.NewLabel("Directory Jumps")
	titleLabel.TextStyle.Bold = true

	searchLabel := widget.NewLabel("Filter:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, d.searchEntry)

	listScroll := newScrollableDialogList(d.jumpList, directoryJumpListWidth(d.allEntries, searchDialogListWidth), searchDialogListWidth, searchDialogListHeight)

	emptyLabel := widget.NewLabel("No matching shortcuts found")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyLabel.Hide()

	fixedContainer := container.NewWithoutLayout(listScroll, emptyLabel)
	fixedContainer.Resize(searchDialogListSize())
	listScroll.Resize(searchDialogListSize())
	listScroll.Move(fyne.NewPos(0, 0))
	emptyLabel.Resize(searchDialogListSize())
	emptyLabel.Move(fyne.NewPos(0, 0))

	d.searchEntry.OnChanged = func(query string) {
		d.updateFilteredEntries(query)
		if len(d.filteredEntries) == 0 {
			listScroll.Hide()
			emptyLabel.Show()
		} else {
			emptyLabel.Hide()
			listScroll.Show()
		}
	}

	content := container.NewBorder(
		container.NewVBox(titleLabel, searchSection),
		nil,
		nil,
		nil,
		fixedContainer,
	)
	content.Resize(searchDialogContentSize())

	d.callback = callback
	d.parent = parent

	handler := keymanager.NewDirectoryJumpDialogKeyHandler(d, d.debugPrint)
	d.kmToken = d.keyManager.PushHandler(handler)

	d.sink = NewKeySink(content, d.keyManager, WithTabCapture(true))
	d.searchEntry.SetFocusRedirect(parent, d.sink)

	d.dialog = dialog.NewCustomConfirm(
		"Jump To Directory",
		"OK",
		"Cancel",
		d.sink,
		func(response bool) {
			if response {
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
		d.searchEntry.RefreshIMEAnchor()
	}
}

// MoveUp moves the selection up.
func (d *DirectoryJumpDialog) MoveUp() {
	if d.jumpList != nil && len(d.filteredEntries) > 0 {
		newIndex := d.selectedIndex - 1
		if newIndex < 0 {
			newIndex = 0
		}
		if newIndex != d.selectedIndex {
			d.jumpList.Select(widget.ListItemID(newIndex))
			d.debugPrint("DirectoryJumpDialog: move up index=%d", newIndex)
		}
	}
}

// MoveDown moves the selection down.
func (d *DirectoryJumpDialog) MoveDown() {
	if d.jumpList != nil && len(d.filteredEntries) > 0 {
		newIndex := d.selectedIndex + 1
		maxIndex := len(d.filteredEntries) - 1
		if newIndex > maxIndex {
			newIndex = maxIndex
		}
		if newIndex != d.selectedIndex {
			d.jumpList.Select(widget.ListItemID(newIndex))
			d.debugPrint("DirectoryJumpDialog: move down index=%d", newIndex)
		}
	}
}

// MoveToTop moves selection to the top.
func (d *DirectoryJumpDialog) MoveToTop() {
	if d.jumpList != nil && len(d.filteredEntries) > 0 {
		d.jumpList.Select(0)
		d.selectedPath = d.filteredEntries[0].Directory
		d.debugPrint("DirectoryJumpDialog: move top")
	}
}

// MoveToBottom moves selection to the bottom.
func (d *DirectoryJumpDialog) MoveToBottom() {
	if d.jumpList != nil && len(d.filteredEntries) > 0 {
		last := len(d.filteredEntries) - 1
		d.jumpList.Select(widget.ListItemID(last))
		d.selectedPath = d.filteredEntries[last].Directory
		d.debugPrint("DirectoryJumpDialog: move bottom")
	}
}

// ClearSearch clears the search text.
func (d *DirectoryJumpDialog) ClearSearch() {
	if d.searchEntry != nil {
		d.searchEntry.SetText("")
		d.debugPrint("DirectoryJumpDialog: clear search")
	}
}

// AppendToSearch appends a character to the search text.
func (d *DirectoryJumpDialog) AppendToSearch(char string) {
	if d.searchEntry != nil {
		d.searchEntry.SetText(d.searchEntry.Text + char)
		d.debugPrint("DirectoryJumpDialog: append search=%s", d.searchEntry.Text)
		d.scheduleAutoAccept()
	}
}

func (d *DirectoryJumpDialog) scheduleAutoAccept() {
	if d.closed || d.autoAcceptScheduled || d.pendingAutoAcceptPath == "" {
		return
	}
	path := d.pendingAutoAcceptPath
	d.autoAcceptScheduled = true
	d.keyManager.DeferUntilKeysReleased("directoryJump.autoAccept", func() {
		if d.closed || d.pendingAutoAcceptPath != path {
			return
		}
		d.acceptPath(path)
	})
}

// BackspaceSearch removes the last character from search text.
func (d *DirectoryJumpDialog) BackspaceSearch() {
	if d.searchEntry != nil {
		current := d.searchEntry.Text
		if len(current) > 0 {
			d.searchEntry.SetText(trimLastRune(current))
			d.debugPrint("DirectoryJumpDialog: backspace search=%s", d.searchEntry.Text)
		}
	}
}

// CopySelectedShortcutToSearch copies the selected shortcut into the search text.
func (d *DirectoryJumpDialog) CopySelectedShortcutToSearch() {
	if d.searchEntry != nil && d.selectedIndex >= 0 && d.selectedIndex < len(d.filteredEntries) {
		shortcut := d.filteredEntries[d.selectedIndex].Shortcut
		if shortcut != "" {
			d.searchEntry.SetText(shortcut)
			d.debugPrint("DirectoryJumpDialog: copy selected shortcut=%s", shortcut)
		}
	}
}

// SelectCurrentItem selects the current list item.
func (d *DirectoryJumpDialog) SelectCurrentItem() {
	d.debugPrint("DirectoryJumpDialog: select current path=%s", d.selectedPath)
}

// AcceptSelection accepts the selected row.
func (d *DirectoryJumpDialog) AcceptSelection() {
	d.acceptPath(d.selectedPath)
}

func (d *DirectoryJumpDialog) acceptPath(path string) {
	if d.closed {
		return
	}
	d.closed = true

	deferDialogClose(d.keyManager, "directoryJump.accept", func() {
		d.keyManager.RemoveHandler(d.kmToken)
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink, d.searchEntry)
		if d.callback != nil && path != "" {
			d.callback(path)
		}
	})
}

// CancelDialog cancels the dialog without selection.
func (d *DirectoryJumpDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.closed = true

	deferDialogClose(d.keyManager, "directoryJump.cancel", func() {
		d.keyManager.RemoveHandler(d.kmToken)
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink, d.searchEntry)
	})
}
