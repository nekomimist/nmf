package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
)

// FilterDialog represents a file filter dialog with pattern search
type FilterDialog struct {
	searchEntry     *CustomSearchEntry
	filterList      *widget.List
	selectedPattern string
	selectedIndex   int // Currently selected list index
	filteredEntries []config.FilterEntry
	allEntries      []config.FilterEntry
	dataBinding     binding.StringList
	debugPrint      func(format string, args ...interface{})
	keyManager      *keymanager.KeyManager    // Keyboard input manager
	dialog          dialog.Dialog             // Reference to the actual dialog
	callback        func(*config.FilterEntry) // Callback function for selection
	deleteCallback  func(string)              // Callback function for deleting a history entry
	parent          fyne.Window               // Parent window for focus management
	closed          bool                      // Prevent double-close/pop
	sink            *KeySink                  // Key capturing wrapper
	previewLabel    *widget.Label             // Preview of match count
	currentFiles    []fileinfo.FileInfo       // Current directory files for preview
	matchers        *search.Provider
}

// NewFilterDialog creates a new filter dialog
func NewFilterDialog(
	entries []config.FilterEntry,
	currentFiles []fileinfo.FileInfo,
	keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
	matchers ...*search.Provider,
) *FilterDialog {
	dialog := &FilterDialog{
		allEntries:   entries,
		currentFiles: currentFiles,
		debugPrint:   debugPrint,
		keyManager:   keyManager,
	}
	if len(matchers) > 0 {
		dialog.matchers = matchers[0]
	}
	if dialog.matchers == nil {
		dialog.matchers = search.NewPlainProvider()
	}

	dialog.createWidgets()
	dialog.updateFilteredEntries("")
	return dialog
}

// createWidgets creates the UI widgets
func (fd *FilterDialog) createWidgets() {
	// Create search entry - custom entry that redirects focus to KeySink
	fd.searchEntry = NewCustomSearchEntry()
	fd.searchEntry.SetPlaceHolder("Enter filter pattern (e.g., *.go, *.{js,ts}, test*)...")

	// Set up real-time search
	fd.searchEntry.OnChanged = func(query string) {
		fd.updateFilteredEntries(query)
		fd.updatePreview(fd.previewPattern())
	}

	// Create preview label
	fd.previewLabel = widget.NewLabel("")
	fd.previewLabel.TextStyle.Italic = true

	// Create data binding for the list
	fd.dataBinding = binding.NewStringList()

	// Create filter list
	fd.filterList = widget.NewListWithData(
		fd.dataBinding,
		func() fyne.CanvasObject {
			// Template: pattern + usage info
			label := widget.NewLabel("")
			return label
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			str, _ := item.(binding.String).Get()
			if str == "" {
				return
			}

			// Update the label with formatted pattern info
			if label, ok := obj.(*widget.Label); ok {
				label.SetText(str)
			}
		},
	)

	// Set selection handler
	fd.filterList.OnSelected = func(id widget.ListItemID) {
		if id < len(fd.filteredEntries) {
			fd.selectedIndex = int(id)
			entry := fd.filteredEntries[id]
			fd.selectedPattern = entry.Pattern
			fd.debugPrint("FilterDialog: Filter selected: %s (index: %d)", fd.selectedPattern, fd.selectedIndex)
			fd.updatePreview(fd.previewPattern())
			// Keep focus on sink so KeyManager continues to receive keys
			if fd.parent != nil && fd.sink != nil {
				fd.parent.Canvas().Focus(fd.sink)
			}
		}
	}

	// Set list size
	fd.filterList.Resize(filterDialogListSize())
}

// updateFilteredEntries updates the filtered entries based on query
func (fd *FilterDialog) updateFilteredEntries(query string) {
	if query == "" {
		fd.filteredEntries = fd.allEntries
	} else {
		matcher := fd.matchers.Build(query)
		fd.filteredEntries = []config.FilterEntry{}

		for _, entry := range fd.allEntries {
			if matcher.Match(entry.Pattern) {
				fd.filteredEntries = append(fd.filteredEntries, entry)
			}
		}
	}

	// Format entries for display.
	displayEntries := make([]string, len(fd.filteredEntries))
	for i, entry := range fd.filteredEntries {
		displayEntries[i] = entry.Pattern
	}

	// Update data binding with the formatted entries
	fd.dataBinding.Set(displayEntries)
	fd.filterList.Refresh()

	// Set initial selection to first item if available
	if len(fd.filteredEntries) > 0 {
		fd.selectedIndex = 0
		fd.selectedPattern = fd.filteredEntries[0].Pattern
		fd.filterList.Select(0)
	} else {
		fd.selectedIndex = -1
		fd.selectedPattern = ""
	}
}

// updatePreview updates the preview label with match count
func (fd *FilterDialog) updatePreview(pattern string) {
	effectivePattern := config.EffectiveFilterPattern(pattern)
	if effectivePattern == "" {
		fd.previewLabel.SetText("")
		return
	}

	// Count matches in current directory
	matchCount := 0
	dirCount := 0
	for _, file := range fd.currentFiles {
		if file.IsDir {
			dirCount++ // Count directories separately
			continue   // Directories are always shown, so don't include in match count
		}

		matched, err := fileinfo.MatchesPattern(file.Name, effectivePattern)
		if err == nil && matched {
			matchCount++
		}
	}

	if matchCount == 0 {
		if dirCount > 0 {
			fd.previewLabel.SetText(fmt.Sprintf("No file matches, %d directories shown", dirCount))
		} else {
			fd.previewLabel.SetText("No matches in current directory")
		}
	} else {
		fd.previewLabel.SetText(fmt.Sprintf("Matches: %d files + %d directories", matchCount, dirCount))
	}
}

func (fd *FilterDialog) previewPattern() string {
	if fd.selectedIndex >= 0 && fd.selectedIndex < len(fd.filteredEntries) {
		return fd.filteredEntries[fd.selectedIndex].Pattern
	}
	if fd.searchEntry != nil {
		return fd.searchEntry.Text
	}
	return ""
}

// ShowDialog shows the file filter dialog
func (fd *FilterDialog) ShowDialog(parent fyne.Window, callback func(*config.FilterEntry), deleteCallback func(string)) {
	// Create title label
	titleLabel := widget.NewLabel("File Filter")
	titleLabel.TextStyle.Bold = true

	// Create search section
	searchLabel := widget.NewLabel("Pattern:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, fd.searchEntry)

	// Create scrollable list container
	listScroll := container.NewScroll(dialogListThemeOverride(fd.filterList))
	listScroll.SetMinSize(filterDialogListSize())

	// Create empty state message
	emptyLabel := widget.NewLabel("No matching patterns found")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyLabel.Hide()

	// Create a fixed-size container that maintains its size
	fixedContainer := container.NewWithoutLayout(listScroll, emptyLabel)
	fixedContainer.Resize(filterDialogListSize())

	// Position elements manually to fill the container
	listScroll.Resize(filterDialogListSize())
	listScroll.Move(fyne.NewPos(0, 0))

	emptyLabel.Resize(filterDialogListSize())
	emptyLabel.Move(fyne.NewPos(0, 0))

	// Show/hide empty state based on filtered results
	fd.searchEntry.OnChanged = func(query string) {
		fd.updateFilteredEntries(query)
		fd.updatePreview(fd.previewPattern())

		// Only show "No matching patterns found" if:
		// 1. We have history entries to search through AND
		// 2. The query doesn't match any existing history
		if len(fd.allEntries) > 0 && len(fd.filteredEntries) == 0 && query != "" {
			listScroll.Hide()
			emptyLabel.Show()
		} else {
			emptyLabel.Hide()
			listScroll.Show()
		}
	}

	// Create main content
	content := container.NewBorder(
		container.NewVBox(titleLabel, searchSection, fd.previewLabel), // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		fixedContainer, // center - fixed size container
	)

	// Set minimum size
	content.Resize(searchDialogContentSize())

	// Store callback and parent for use by key handler
	fd.callback = callback
	fd.deleteCallback = deleteCallback
	fd.parent = parent

	// Create and push filter dialog key handler
	filterHandler := keymanager.NewFilterDialogKeyHandler(fd, fd.debugPrint)
	fd.keyManager.PushHandler(filterHandler)

	// Wrap content with KeySink to capture Tab and forward keys
	fd.sink = NewKeySink(content, fd.keyManager, WithTabCapture(true))

	// Configure search entry to redirect focus to sink
	fd.searchEntry.SetFocusRedirect(parent, fd.sink)

	// Create custom dialog with proper focus handling (wrapped by sink)
	fd.dialog = dialog.NewCustomConfirm(
		"Apply Filter",
		"OK",
		"Cancel",
		fd.sink,
		func(response bool) {
			if response {
				fd.AcceptSelection()
			} else {
				fd.CancelDialog()
			}
		},
		parent,
	)

	// Show dialog and ensure focus stays on sink so KeyManager gets keys
	fd.dialog.Show()
	if fd.parent != nil && fd.sink != nil {
		fd.parent.Canvas().Focus(fd.sink)
		fd.searchEntry.RefreshIMEAnchor()
	}
}

// FilterDialogInterface implementation methods

// MoveUp moves the selection up in the filter list
func (fd *FilterDialog) MoveUp() {
	if fd.filterList != nil && len(fd.filteredEntries) > 0 {
		newIndex := fd.selectedIndex - 1
		if newIndex < 0 {
			newIndex = 0 // Stay at top
		}
		if newIndex != fd.selectedIndex {
			fd.filterList.Select(widget.ListItemID(newIndex))
			fd.debugPrint("FilterDialog: Move up to index %d", newIndex)
		}
	}
}

// MoveDown moves the selection down in the filter list
func (fd *FilterDialog) MoveDown() {
	if fd.filterList != nil && len(fd.filteredEntries) > 0 {
		newIndex := fd.selectedIndex + 1
		maxIndex := len(fd.filteredEntries) - 1
		if newIndex > maxIndex {
			newIndex = maxIndex // Stay at bottom
		}
		if newIndex != fd.selectedIndex {
			fd.filterList.Select(widget.ListItemID(newIndex))
			fd.debugPrint("FilterDialog: Move down to index %d", newIndex)
		}
	}
}

// MoveToTop moves selection to the top of the list
func (fd *FilterDialog) MoveToTop() {
	if fd.filterList != nil && len(fd.filteredEntries) > 0 {
		fd.filterList.Select(0)
		if len(fd.filteredEntries) > 0 {
			fd.selectedPattern = fd.filteredEntries[0].Pattern
		}
		fd.debugPrint("FilterDialog: Move to top")
	}
}

// MoveToBottom moves selection to the bottom of the list
func (fd *FilterDialog) MoveToBottom() {
	if fd.filterList != nil && len(fd.filteredEntries) > 0 {
		lastIdx := len(fd.filteredEntries) - 1
		fd.filterList.Select(lastIdx)
		fd.selectedPattern = fd.filteredEntries[lastIdx].Pattern
		fd.debugPrint("FilterDialog: Move to bottom")
	}
}

// ClearSearch clears the search entry
func (fd *FilterDialog) ClearSearch() {
	if fd.searchEntry != nil {
		fd.searchEntry.SetText("")
		fd.debugPrint("FilterDialog: Clear search")
	}
}

// SelectCurrentItem selects the current item in the list
func (fd *FilterDialog) SelectCurrentItem() {
	// The selection is already handled by the list widget
	fd.debugPrint("FilterDialog: Select current item: %s", fd.selectedPattern)
}

// AcceptSelection accepts the current selection and closes the dialog
func (fd *FilterDialog) AcceptSelection() {
	if fd.closed {
		return
	}
	fd.closed = true

	var selectedEntry *config.FilterEntry

	if fd.selectedIndex >= 0 && fd.selectedIndex < len(fd.filteredEntries) {
		// Use selected entry from list
		entry := fd.filteredEntries[fd.selectedIndex]
		selectedEntry = &entry
	} else if fd.searchEntry != nil && strings.TrimSpace(fd.searchEntry.Text) != "" {
		// Create new filter entry from current input when no history item matches.
		selectedEntry = &config.FilterEntry{
			Pattern: strings.TrimSpace(fd.searchEntry.Text),
		}
	}

	deferDialogClose(fd.keyManager, "filter.accept", func() {
		fd.keyManager.PopHandler()
		if fd.callback != nil && selectedEntry != nil {
			fd.callback(selectedEntry)
		}
		if fd.dialog != nil {
			fd.dialog.Hide()
		}
		unfocusIfDialogOwned(fd.parent, fd.sink, fd.searchEntry)
	})
}

// AcceptDirectInput accepts the current search text as a new filter.
func (fd *FilterDialog) AcceptDirectInput() {
	if fd.closed {
		return
	}
	if fd.searchEntry == nil || strings.TrimSpace(fd.searchEntry.Text) == "" {
		return
	}
	fd.closed = true

	selectedEntry := &config.FilterEntry{
		Pattern: strings.TrimSpace(fd.searchEntry.Text),
	}

	deferDialogClose(fd.keyManager, "filter.acceptDirect", func() {
		fd.keyManager.PopHandler()
		if fd.callback != nil && selectedEntry != nil {
			fd.callback(selectedEntry)
		}
		if fd.dialog != nil {
			fd.dialog.Hide()
		}
		unfocusIfDialogOwned(fd.parent, fd.sink, fd.searchEntry)
	})
}

// DeleteSelectedEntry removes the selected filter history entry.
func (fd *FilterDialog) DeleteSelectedEntry() {
	if fd.selectedIndex < 0 || fd.selectedIndex >= len(fd.filteredEntries) {
		return
	}
	pattern := fd.filteredEntries[fd.selectedIndex].Pattern
	fd.debugPrint("FilterDialog: Delete selected entry: %s", pattern)

	for i := range fd.allEntries {
		if fd.allEntries[i].Pattern == pattern {
			fd.allEntries = append(fd.allEntries[:i], fd.allEntries[i+1:]...)
			break
		}
	}
	if fd.deleteCallback != nil {
		fd.deleteCallback(pattern)
	}
	query := ""
	if fd.searchEntry != nil {
		query = fd.searchEntry.Text
	}
	fd.updateFilteredEntries(query)
	fd.updatePreview(fd.previewPattern())
}

// CancelDialog cancels the dialog without selection
func (fd *FilterDialog) CancelDialog() {
	if fd.closed {
		return
	}
	fd.closed = true

	deferDialogClose(fd.keyManager, "filter.cancel", func() {
		fd.keyManager.PopHandler()
		if fd.dialog != nil {
			fd.dialog.Hide()
		}
		unfocusIfDialogOwned(fd.parent, fd.sink, fd.searchEntry)
	})
}

// IsSearchFocused returns true if the search entry has focus
func (fd *FilterDialog) IsSearchFocused() bool {
	if fd.searchEntry == nil || fd.parent == nil {
		return false
	}

	// Check if searchEntry is the focused object
	focused := fd.parent.Canvas().Focused()
	return focused == fd.searchEntry
}

// FocusList moves focus to the filter list (deprecated in focusless design)
func (fd *FilterDialog) FocusList() {
	// In focusless design, this is a no-op but kept for interface compatibility
	fd.debugPrint("FilterDialog: FocusList called (focusless mode)")
}

// AppendToSearch appends a character to the search entry
func (fd *FilterDialog) AppendToSearch(char string) {
	if fd.searchEntry != nil {
		current := fd.searchEntry.Text
		fd.searchEntry.SetText(current + char)
		fd.debugPrint("FilterDialog: Appended '%s' to search, now: '%s'", char, fd.searchEntry.Text)
	}
}

// BackspaceSearch removes the last character from search
func (fd *FilterDialog) BackspaceSearch() {
	if fd.searchEntry != nil {
		current := fd.searchEntry.Text
		if len(current) > 0 {
			newText := trimLastRune(current)
			fd.searchEntry.SetText(newText)
			fd.debugPrint("FilterDialog: Backspaced search, now: '%s'", newText)
		}
	}
}

// GetSearchText returns current search text
func (fd *FilterDialog) GetSearchText() string {
	if fd.searchEntry != nil {
		return fd.searchEntry.Text
	}
	return ""
}
