package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

// FilterDialog represents a file filter dialog with pattern search
type FilterDialog struct {
	searchEntry     *widget.Entry
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
	parent          fyne.Window               // Parent window for focus management
	closed          bool                      // Prevent double-close/pop
	sink            *KeySink                  // Key capturing wrapper
	previewLabel    *widget.Label             // Preview of match count
	currentFiles    []fileinfo.FileInfo       // Current directory files for preview
}

// NewFilterDialog creates a new filter dialog
func NewFilterDialog(
	entries []config.FilterEntry,
	currentFiles []fileinfo.FileInfo,
	keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
) *FilterDialog {
	dialog := &FilterDialog{
		allEntries:   entries,
		currentFiles: currentFiles,
		debugPrint:   debugPrint,
		keyManager:   keyManager,
	}

	dialog.createWidgets()
	dialog.updateFilteredEntries("")
	return dialog
}

// createWidgets creates the UI widgets
func (fd *FilterDialog) createWidgets() {
	// Create search entry
	fd.searchEntry = widget.NewEntry()
	fd.searchEntry.SetPlaceHolder("Enter filter pattern (e.g., *.go, *.{js,ts}, test*)...")
	// Display-only: disable focus/input; KeyManager drives updates
	fd.searchEntry.Disable()

	// Set up real-time search
	fd.searchEntry.OnChanged = func(query string) {
		fd.updateFilteredEntries(query)
		fd.updatePreview(query)
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
			fd.debugPrint("Filter selected: %s (index: %d)", fd.selectedPattern, fd.selectedIndex)
			// Keep focus on sink so KeyManager continues to receive keys
			if fd.parent != nil && fd.sink != nil {
				fd.parent.Canvas().Focus(fd.sink)
			}
		}
	}

	// Set list size
	fd.filterList.Resize(fyne.NewSize(600, 350))
}

// updateFilteredEntries updates the filtered entries based on query
func (fd *FilterDialog) updateFilteredEntries(query string) {
	if query == "" {
		fd.filteredEntries = fd.allEntries
	} else {
		query = strings.ToLower(query)
		fd.filteredEntries = []config.FilterEntry{}

		for _, entry := range fd.allEntries {
			if strings.Contains(strings.ToLower(entry.Pattern), query) {
				fd.filteredEntries = append(fd.filteredEntries, entry)
			}
		}
	}

	// Sort entries by usage frequency and recency
	sort.Slice(fd.filteredEntries, func(i, j int) bool {
		// First priority: usage count
		if fd.filteredEntries[i].UseCount != fd.filteredEntries[j].UseCount {
			return fd.filteredEntries[i].UseCount > fd.filteredEntries[j].UseCount
		}
		// Second priority: most recent usage
		return fd.filteredEntries[i].LastUsed.After(fd.filteredEntries[j].LastUsed)
	})

	// Format entries for display
	displayEntries := make([]string, len(fd.filteredEntries))
	for i, entry := range fd.filteredEntries {
		usageInfo := ""
		if entry.UseCount > 0 {
			usageInfo = fmt.Sprintf(" (used %d times)", entry.UseCount)
		}
		displayEntries[i] = entry.Pattern + usageInfo
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
	if pattern == "" {
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

		matched, err := fileinfo.MatchesPattern(file.Name, pattern)
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

// ShowDialog shows the file filter dialog
func (fd *FilterDialog) ShowDialog(parent fyne.Window, callback func(*config.FilterEntry)) {
	// Create title label
	titleLabel := widget.NewLabel("File Filter")
	titleLabel.TextStyle.Bold = true

	// Create search section
	searchLabel := widget.NewLabel("Pattern:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, fd.searchEntry)

	// Create scrollable list container
	listScroll := container.NewScroll(fd.filterList)
	listScroll.SetMinSize(fyne.NewSize(600, 350))

	// Create empty state message
	emptyLabel := widget.NewLabel("No matching patterns found")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyLabel.Hide()

	// Create a fixed-size container that maintains its size
	fixedContainer := container.NewWithoutLayout(listScroll, emptyLabel)
	fixedContainer.Resize(fyne.NewSize(600, 350))

	// Position elements manually to fill the container
	listScroll.Resize(fyne.NewSize(600, 350))
	listScroll.Move(fyne.NewPos(0, 0))

	emptyLabel.Resize(fyne.NewSize(600, 350))
	emptyLabel.Move(fyne.NewPos(0, 0))

	// Show/hide empty state based on filtered results
	fd.searchEntry.OnChanged = func(query string) {
		fd.updateFilteredEntries(query)
		fd.updatePreview(query)

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
	content.Resize(fyne.NewSize(650, 500))

	// Store callback and parent for use by key handler
	fd.callback = callback
	fd.parent = parent

	// Create and push filter dialog key handler
	filterHandler := keymanager.NewFilterDialogKeyHandler(fd, fd.debugPrint)
	fd.keyManager.PushHandler(filterHandler)

	// Wrap content with KeySink to capture Tab and forward keys
	fd.sink = NewKeySink(content, fd.keyManager, WithTabCapture(true))

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

// FocusSearch focuses the search entry
func (fd *FilterDialog) FocusSearch() {
	if fd.searchEntry != nil {
		fd.searchEntry.FocusGained()
		fd.debugPrint("FilterDialog: Focus search")
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
	// Pop the handler first
	fd.keyManager.PopHandler()

	var selectedEntry *config.FilterEntry

	// Use current input as pattern if no existing selection
	currentInput := fd.searchEntry.Text
	if currentInput != "" {
		// Create new filter entry from current input
		selectedEntry = &config.FilterEntry{
			Pattern:  currentInput,
			LastUsed: time.Now(),
			UseCount: 1,
		}
	} else if fd.selectedIndex >= 0 && fd.selectedIndex < len(fd.filteredEntries) {
		// Use selected entry from list
		selectedEntry = &fd.filteredEntries[fd.selectedIndex]
		selectedEntry.LastUsed = time.Now()
		selectedEntry.UseCount++
	}

	if fd.callback != nil && selectedEntry != nil {
		fd.callback(selectedEntry)
	}

	if fd.dialog != nil {
		fd.dialog.Hide()
	}
	if fd.parent != nil {
		fd.parent.Canvas().Unfocus()
	}
}

// CancelDialog cancels the dialog without selection
func (fd *FilterDialog) CancelDialog() {
	if fd.closed {
		return
	}
	fd.closed = true
	// Pop the handler first
	fd.keyManager.PopHandler()

	if fd.dialog != nil {
		fd.dialog.Hide()
	}
	if fd.parent != nil {
		fd.parent.Canvas().Unfocus()
	}
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
			newText := current[:len(current)-1]
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
