package ui

import (
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

// CustomSearchEntry is a custom entry that redirects focus to KeySink
type CustomSearchEntry struct {
	widget.Entry
	parent fyne.Window
	sink   *KeySink
}

// NewCustomSearchEntry creates a new custom search entry
func NewCustomSearchEntry() *CustomSearchEntry {
	entry := &CustomSearchEntry{}
	entry.ExtendBaseWidget(entry)
	return entry
}

// FocusGained is called when the entry gains focus - immediately redirect to sink
func (c *CustomSearchEntry) FocusGained() {
	// Redirect focus immediately to sink to prevent entry from handling input
	if c.parent != nil && c.sink != nil {
		c.parent.Canvas().Focus(c.sink)
	}
}

// SetFocusRedirect sets the parent window and sink for focus redirection
func (c *CustomSearchEntry) SetFocusRedirect(parent fyne.Window, sink *KeySink) {
	c.parent = parent
	c.sink = sink
}

// NavigationHistoryDialog represents a navigation history dialog with search
type NavigationHistoryDialog struct {
	searchEntry   *CustomSearchEntry
	historyList   *widget.List
	selectedPath  string
	selectedIndex int // Currently selected list index
	filteredPaths []string
	allPaths      []string
	lastUsed      map[string]time.Time
	dataBinding   binding.StringList
	debugPrint    func(format string, args ...interface{})
	keyManager    *keymanager.KeyManager // Keyboard input manager
	dialog        dialog.Dialog          // Reference to the actual dialog
	callback      func(string)           // Callback function for selection
	parent        fyne.Window            // Parent window for focus management
	closed        bool                   // Prevent double-close/pop
	sink          *KeySink               // Key capturing wrapper
}

// NewNavigationHistoryDialog creates a new navigation history dialog
func NewNavigationHistoryDialog(
	paths []string,
	lastUsed map[string]time.Time,
	keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
) *NavigationHistoryDialog {
	dialog := &NavigationHistoryDialog{
		allPaths:   paths,
		lastUsed:   lastUsed,
		debugPrint: debugPrint,
		keyManager: keyManager,
	}

	dialog.createWidgets()
	dialog.updateFilteredPaths("")
	return dialog
}

// createWidgets creates the UI widgets
func (nhd *NavigationHistoryDialog) createWidgets() {
	// Create search entry - custom entry that redirects focus to KeySink
	nhd.searchEntry = NewCustomSearchEntry()
	nhd.searchEntry.SetPlaceHolder("Type to filter paths...")

	// Set up real-time search
	nhd.searchEntry.OnChanged = func(query string) {
		nhd.updateFilteredPaths(query)
	}

	// Create data binding for the list
	nhd.dataBinding = binding.NewStringList()

	// Create history list
	nhd.historyList = widget.NewListWithData(
		nhd.dataBinding,
		func() fyne.CanvasObject {
			// Simple template: just a label showing the full path
			label := widget.NewLabel("")
			return label
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			str, _ := item.(binding.String).Get()
			if str == "" {
				return
			}

			// Update the label with the full path
			if label, ok := obj.(*widget.Label); ok {
				label.SetText(str)
			}
		},
	)

	// Set selection handler
	nhd.historyList.OnSelected = func(id widget.ListItemID) {
		if id < len(nhd.filteredPaths) {
			nhd.selectedIndex = int(id)
			nhd.selectedPath = nhd.filteredPaths[id]
			nhd.debugPrint("History selected: %s (index: %d)", nhd.selectedPath, nhd.selectedIndex)
			// Keep focus on sink so KeyManager continues to receive keys
			if nhd.parent != nil && nhd.sink != nil {
				nhd.parent.Canvas().Focus(nhd.sink)
			}
		}
	}

	// Set list size
	nhd.historyList.Resize(fyne.NewSize(600, 400))
}

// updateFilteredPaths updates the filtered paths based on query
func (nhd *NavigationHistoryDialog) updateFilteredPaths(query string) {
	if query == "" {
		nhd.filteredPaths = nhd.allPaths
	} else {
		query = strings.ToLower(query)
		nhd.filteredPaths = []string{}

		for _, path := range nhd.allPaths {
			if strings.Contains(strings.ToLower(path), query) {
				nhd.filteredPaths = append(nhd.filteredPaths, path)
			}
		}
	}

	// Update data binding with the filtered paths directly
	nhd.dataBinding.Set(nhd.filteredPaths)
	nhd.historyList.Refresh()

	// Set initial selection to first item if available
	if len(nhd.filteredPaths) > 0 {
		nhd.selectedIndex = 0
		nhd.selectedPath = nhd.filteredPaths[0]
		nhd.historyList.Select(0)
	} else {
		nhd.selectedIndex = -1
		nhd.selectedPath = ""
	}
}

// ShowDialog shows the navigation history dialog
func (nhd *NavigationHistoryDialog) ShowDialog(parent fyne.Window, callback func(string)) {
	// Create title label
	titleLabel := widget.NewLabel("Navigation History")
	titleLabel.TextStyle.Bold = true

	// Create search section
	searchLabel := widget.NewLabel("Filter:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, nhd.searchEntry)

	// Create scrollable list container
	listScroll := container.NewScroll(nhd.historyList)
	listScroll.SetMinSize(fyne.NewSize(600, 400))

	// Create empty state message
	emptyLabel := widget.NewLabel("No matching paths found")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyLabel.Hide()

	// Create a fixed-size container that maintains its size
	fixedContainer := container.NewWithoutLayout(listScroll, emptyLabel)
	fixedContainer.Resize(fyne.NewSize(600, 400))

	// Position elements manually to fill the container
	listScroll.Resize(fyne.NewSize(600, 400))
	listScroll.Move(fyne.NewPos(0, 0))

	emptyLabel.Resize(fyne.NewSize(600, 400))
	emptyLabel.Move(fyne.NewPos(0, 0))

	// Note: Container doesn't have SetMinSize, but the manual layout should maintain size

	// Show/hide empty state based on filtered results
	nhd.searchEntry.OnChanged = func(query string) {
		nhd.updateFilteredPaths(query)
		if len(nhd.filteredPaths) == 0 {
			listScroll.Hide()
			emptyLabel.Show()
		} else {
			emptyLabel.Hide()
			listScroll.Show()
		}
	}

	// Create main content
	content := container.NewBorder(
		container.NewVBox(titleLabel, searchSection), // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		fixedContainer, // center - fixed size container
	)

	// Set minimum size
	content.Resize(fyne.NewSize(650, 500))

	// Store callback and parent for use by key handler
	nhd.callback = callback
	nhd.parent = parent

	// Create and push history dialog key handler
	historyHandler := keymanager.NewHistoryDialogKeyHandler(nhd, nhd.debugPrint)
	nhd.keyManager.PushHandler(historyHandler)

	// Wrap content with KeySink to capture Tab and forward keys
	nhd.sink = NewKeySink(content, nhd.keyManager, WithTabCapture(true))

	// Configure search entry to redirect focus to sink
	nhd.searchEntry.SetFocusRedirect(parent, nhd.sink)

	// Create custom dialog with proper focus handling (wrapped by sink)
	nhd.dialog = dialog.NewCustomConfirm(
		"Select Directory",
		"OK",
		"Cancel",
		nhd.sink,
		func(response bool) {
			if response {
				nhd.AcceptSelection()
			} else {
				nhd.CancelDialog()
			}
		},
		parent,
	)

	// Show dialog and ensure focus stays on sink so KeyManager gets keys
	nhd.dialog.Show()
	if nhd.parent != nil && nhd.sink != nil {
		nhd.parent.Canvas().Focus(nhd.sink)
	}
}

// HistoryDialogInterface implementation methods

// MoveUp moves the selection up in the history list
func (nhd *NavigationHistoryDialog) MoveUp() {
	if nhd.historyList != nil && len(nhd.filteredPaths) > 0 {
		newIndex := nhd.selectedIndex - 1
		if newIndex < 0 {
			newIndex = 0 // Stay at top
		}
		if newIndex != nhd.selectedIndex {
			nhd.historyList.Select(widget.ListItemID(newIndex))
			nhd.debugPrint("HistoryDialog: Move up to index %d", newIndex)
		}
	}
}

// MoveDown moves the selection down in the history list
func (nhd *NavigationHistoryDialog) MoveDown() {
	if nhd.historyList != nil && len(nhd.filteredPaths) > 0 {
		newIndex := nhd.selectedIndex + 1
		maxIndex := len(nhd.filteredPaths) - 1
		if newIndex > maxIndex {
			newIndex = maxIndex // Stay at bottom
		}
		if newIndex != nhd.selectedIndex {
			nhd.historyList.Select(widget.ListItemID(newIndex))
			nhd.debugPrint("HistoryDialog: Move down to index %d", newIndex)
		}
	}
}

// MoveToTop moves selection to the top of the list
func (nhd *NavigationHistoryDialog) MoveToTop() {
	if nhd.historyList != nil && len(nhd.filteredPaths) > 0 {
		nhd.historyList.Select(0)
		if len(nhd.filteredPaths) > 0 {
			nhd.selectedPath = nhd.filteredPaths[0]
		}
		nhd.debugPrint("HistoryDialog: Move to top")
	}
}

// MoveToBottom moves selection to the bottom of the list
func (nhd *NavigationHistoryDialog) MoveToBottom() {
	if nhd.historyList != nil && len(nhd.filteredPaths) > 0 {
		lastIdx := len(nhd.filteredPaths) - 1
		nhd.historyList.Select(lastIdx)
		nhd.selectedPath = nhd.filteredPaths[lastIdx]
		nhd.debugPrint("HistoryDialog: Move to bottom")
	}
}

// ClearSearch clears the search entry
func (nhd *NavigationHistoryDialog) ClearSearch() {
	if nhd.searchEntry != nil {
		nhd.searchEntry.SetText("")
		nhd.debugPrint("HistoryDialog: Clear search")
	}
}

// SelectCurrentItem selects the current item in the list
func (nhd *NavigationHistoryDialog) SelectCurrentItem() {
	// The selection is already handled by the list widget
	nhd.debugPrint("HistoryDialog: Select current item: %s", nhd.selectedPath)
}

// AcceptSelection accepts the current selection and closes the dialog
func (nhd *NavigationHistoryDialog) AcceptSelection() {
	if nhd.closed {
		return
	}
	nhd.closed = true
	// Pop the handler first
	nhd.keyManager.PopHandler()

	// Check if search text is a valid path and no history match exists
	searchText := nhd.GetSearchText()
	if searchText != "" && nhd.isValidPath(searchText) && len(nhd.filteredPaths) == 0 {
		// Use search text as direct path navigation
		absPath := nhd.getAbsolutePath(searchText)
		nhd.debugPrint("HistoryDialog: No history match, using search text as path: %s", absPath)
		if nhd.callback != nil {
			nhd.callback(absPath)
		}
	} else if nhd.callback != nil && nhd.selectedPath != "" {
		// Use selected history path
		nhd.callback(nhd.selectedPath)
	}

	if nhd.dialog != nil {
		nhd.dialog.Hide()
	}
	if nhd.parent != nil {
		nhd.parent.Canvas().Unfocus()
	}
}

// AcceptDirectPathNavigation accepts direct path navigation (Ctrl+Enter)
func (nhd *NavigationHistoryDialog) AcceptDirectPathNavigation() {
	if nhd.closed {
		return
	}
	nhd.closed = true
	// Pop the handler first
	nhd.keyManager.PopHandler()

	searchText := nhd.GetSearchText()
	if searchText != "" && nhd.isValidPath(searchText) {
		// Use search text as direct path navigation, ignoring history
		absPath := nhd.getAbsolutePath(searchText)
		nhd.debugPrint("HistoryDialog: Direct path navigation (Ctrl+Enter): %s", absPath)
		if nhd.callback != nil {
			nhd.callback(absPath)
		}
	} else if nhd.callback != nil && nhd.selectedPath != "" {
		// Fallback to selected history path if search text is not valid
		nhd.debugPrint("HistoryDialog: Invalid search path, falling back to selected history: %s", nhd.selectedPath)
		nhd.callback(nhd.selectedPath)
	}

	if nhd.dialog != nil {
		nhd.dialog.Hide()
	}
	if nhd.parent != nil {
		nhd.parent.Canvas().Unfocus()
	}
}

// CancelDialog cancels the dialog without selection
func (nhd *NavigationHistoryDialog) CancelDialog() {
	if nhd.closed {
		return
	}
	nhd.closed = true
	// Pop the handler first
	nhd.keyManager.PopHandler()

	if nhd.dialog != nil {
		nhd.dialog.Hide()
	}
	if nhd.parent != nil {
		nhd.parent.Canvas().Unfocus()
	}
}

// IsSearchFocused returns true if the search entry has focus
func (nhd *NavigationHistoryDialog) IsSearchFocused() bool {
	if nhd.searchEntry == nil || nhd.parent == nil {
		return false
	}

	// Check if searchEntry is the focused object
	focused := nhd.parent.Canvas().Focused()
	return focused == nhd.searchEntry
}

// FocusList moves focus to the history list (deprecated in focusless design)
func (nhd *NavigationHistoryDialog) FocusList() {
	// In focusless design, this is a no-op but kept for interface compatibility
	nhd.debugPrint("HistoryDialog: FocusList called (focusless mode)")
}

// AppendToSearch appends a character to the search entry
func (nhd *NavigationHistoryDialog) AppendToSearch(char string) {
	if nhd.searchEntry != nil {
		current := nhd.searchEntry.Text
		nhd.searchEntry.SetText(current + char)
		nhd.debugPrint("HistoryDialog: Appended '%s' to search, now: '%s'", char, nhd.searchEntry.Text)
	}
}

// BackspaceSearch removes the last character from search
func (nhd *NavigationHistoryDialog) BackspaceSearch() {
	if nhd.searchEntry != nil {
		current := nhd.searchEntry.Text
		if len(current) > 0 {
			newText := current[:len(current)-1]
			nhd.searchEntry.SetText(newText)
			nhd.debugPrint("HistoryDialog: Backspaced search, now: '%s'", newText)
		}
	}
}

// GetSearchText returns current search text
func (nhd *NavigationHistoryDialog) GetSearchText() string {
	if nhd.searchEntry != nil {
		return nhd.searchEntry.Text
	}
	return ""
}

// CopySelectedPathToSearch copies the currently selected path to search entry
func (nhd *NavigationHistoryDialog) CopySelectedPathToSearch() {
	if nhd.searchEntry != nil && nhd.selectedPath != "" {
		nhd.searchEntry.SetText(nhd.selectedPath)
		nhd.debugPrint("HistoryDialog: Copied selected path to search: %s", nhd.selectedPath)
	}
}

// isValidPath checks if the given path is a valid directory path
func (nhd *NavigationHistoryDialog) isValidPath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		nhd.debugPrint("HistoryDialog: Failed to get absolute path for '%s': %v", path, err)
		return false
	}

	// Check if path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		nhd.debugPrint("HistoryDialog: Path does not exist: '%s'", absPath)
		return false
	}

	if !info.IsDir() {
		nhd.debugPrint("HistoryDialog: Path is not a directory: '%s'", absPath)
		return false
	}

	return true
}

// getAbsolutePath returns the absolute path for the given path
func (nhd *NavigationHistoryDialog) getAbsolutePath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absPath
}
