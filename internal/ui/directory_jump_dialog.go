package ui

import (
	"strings"

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
	shortcutTargets map[string]string
	selectedIndex   int
	selectedPath    string
	debugPrint      func(format string, args ...interface{})
	keyManager      *keymanager.KeyManager
	dialog          dialog.Dialog
	callback        func(string)
	parent          fyne.Window
	closed          bool
	sink            *KeySink
}

// NewDirectoryJumpDialog creates a configured directory jump dialog.
func NewDirectoryJumpDialog(
	entries []config.DirectoryJumpEntry,
	keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
) *DirectoryJumpDialog {
	d := &DirectoryJumpDialog{
		allEntries:      copyDirectoryJumpEntries(entries),
		shortcutTargets: buildDirectoryJumpShortcutTargets(entries),
		debugPrint:      debugPrint,
		keyManager:      keyManager,
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

func buildDirectoryJumpShortcutTargets(entries []config.DirectoryJumpEntry) map[string]string {
	targets := make(map[string]string)
	for _, entry := range entries {
		key := NormalizeDirectoryJumpShortcut(entry.Shortcut)
		if key == "" {
			continue
		}
		if _, exists := targets[key]; !exists {
			targets[key] = entry.Directory
		}
	}
	return targets
}

func filterDirectoryJumpEntries(entries []config.DirectoryJumpEntry, query string) []config.DirectoryJumpEntry {
	if query == "" {
		return entries
	}

	needle := strings.ToLower(query)
	filtered := []config.DirectoryJumpEntry{}
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Directory), needle) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// NormalizeDirectoryJumpShortcut normalizes configured and typed shortcut keys.
func NormalizeDirectoryJumpShortcut(shortcut string) string {
	runes := []rune(strings.TrimSpace(shortcut))
	if len(runes) != 1 {
		return ""
	}
	return strings.ToLower(string(runes[0]))
}

func (d *DirectoryJumpDialog) createWidgets() {
	d.searchEntry = NewCustomSearchEntry()
	d.searchEntry.SetPlaceHolder("Type to filter directories...")
	d.searchEntry.OnChanged = func(query string) {
		d.updateFilteredEntries(query)
	}

	d.jumpList = widget.NewList(
		func() int {
			return len(d.filteredEntries)
		},
		func() fyne.CanvasObject {
			shortcut := widget.NewLabel("")
			shortcut.Wrapping = fyne.TextTruncate
			shortcutBox := container.NewGridWrap(directoryJumpShortcutCellSize(), shortcut)
			path := widget.NewLabel("")
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
	d.jumpList.Resize(fyne.NewSize(600, 400))
}

func directoryJumpShortcutCellSize() fyne.Size {
	appTheme := fyne.CurrentApp().Settings().Theme()
	textSize := appTheme.Size(theme.SizeNameText)
	padding := appTheme.Size(theme.SizeNamePadding)
	innerPadding := appTheme.Size(theme.SizeNameInnerPadding)

	return fyne.NewSize(textSize+padding*2, textSize+innerPadding*2)
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
}

// ShowDialog shows the configured directory jump dialog.
func (d *DirectoryJumpDialog) ShowDialog(parent fyne.Window, callback func(string)) {
	titleLabel := widget.NewLabel("Directory Jumps")
	titleLabel.TextStyle.Bold = true

	searchLabel := widget.NewLabel("Filter:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, d.searchEntry)

	listScroll := container.NewScroll(d.jumpList)
	listScroll.SetMinSize(fyne.NewSize(600, 400))

	emptyLabel := widget.NewLabel("No matching directories found")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyLabel.Hide()

	fixedContainer := container.NewWithoutLayout(listScroll, emptyLabel)
	fixedContainer.Resize(fyne.NewSize(600, 400))
	listScroll.Resize(fyne.NewSize(600, 400))
	listScroll.Move(fyne.NewPos(0, 0))
	emptyLabel.Resize(fyne.NewSize(600, 400))
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
	content.Resize(fyne.NewSize(650, 500))

	d.callback = callback
	d.parent = parent

	handler := keymanager.NewDirectoryJumpDialogKeyHandler(d, d.debugPrint)
	d.keyManager.PushHandler(handler)

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
	}
}

// BackspaceSearch removes the last byte from search text.
func (d *DirectoryJumpDialog) BackspaceSearch() {
	if d.searchEntry != nil {
		current := d.searchEntry.Text
		if len(current) > 0 {
			d.searchEntry.SetText(current[:len(current)-1])
			d.debugPrint("DirectoryJumpDialog: backspace search=%s", d.searchEntry.Text)
		}
	}
}

// CopySelectedPathToSearch copies the selected directory into the search text.
func (d *DirectoryJumpDialog) CopySelectedPathToSearch() {
	if d.searchEntry != nil && d.selectedPath != "" {
		d.searchEntry.SetText(d.selectedPath)
		d.debugPrint("DirectoryJumpDialog: copy selected path=%s", d.selectedPath)
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

// AcceptShortcut accepts an Alt+shortcut match, ignoring current filter state.
func (d *DirectoryJumpDialog) AcceptShortcut(shortcut string) {
	key := NormalizeDirectoryJumpShortcut(shortcut)
	path := d.shortcutTargets[key]
	if path == "" {
		return
	}
	d.debugPrint("DirectoryJumpDialog: shortcut %s path=%s", key, path)
	d.acceptPath(path)
}

func (d *DirectoryJumpDialog) acceptPath(path string) {
	if d.closed {
		return
	}
	d.closed = true
	d.keyManager.PopHandler()

	if d.callback != nil && path != "" {
		d.callback(path)
	}
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
}

// CancelDialog cancels the dialog without selection.
func (d *DirectoryJumpDialog) CancelDialog() {
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
