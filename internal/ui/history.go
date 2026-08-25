package ui

import (
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
	customtheme "nmf/internal/theme"
)

const (
	navigationHistoryOrderFrecency = "Frecency"
	navigationHistoryOrderLastUsed = "Last Access"
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
	c.updateIMEAnchor()
	// Redirect focus immediately to sink to prevent entry from handling input
	if c.parent != nil && c.sink != nil {
		c.parent.Canvas().Focus(c.sink)
	}
}

// SetFocusRedirect sets the parent window and sink for focus redirection
func (c *CustomSearchEntry) SetFocusRedirect(parent fyne.Window, sink *KeySink) {
	c.parent = parent
	c.sink = sink
	c.updateIMEAnchor()
}

func (c *CustomSearchEntry) SetText(text string) {
	c.Entry.SetText(text)
	c.updateIMEAnchor()
}

func (c *CustomSearchEntry) RefreshIMEAnchor() {
	c.updateIMEAnchor()
	if fyne.CurrentApp() != nil {
		fyne.Do(func() {
			c.updateIMEAnchor()
		})
	}
}

func (c *CustomSearchEntry) updateIMEAnchor() {
	setIMEAnchorAtTextEnd(c.parent, c, c.Text, c.TextStyle)
}

// NavigationHistoryDialog represents a navigation history dialog with search
type NavigationHistoryDialog struct {
	searchEntry      *CustomSearchEntry
	orderRadio       *widget.RadioGroup
	orderMode        string
	historyList      *widget.List
	selectedPath     string
	selectedIndex    int // Currently selected list index
	filteredPaths    []string
	allPaths         []string
	openPaths        map[string]bool
	pinnedPaths      map[string]bool
	unpinRemovesPath map[string]bool
	lastUsed         map[string]time.Time
	dataBinding      binding.StringList
	debugPrint       func(format string, args ...interface{})
	keyManager       *keymanager.KeyManager // Keyboard input manager
	kmToken          keymanager.HandlerToken
	dialog           dialog.Dialog // Reference to the actual dialog
	callback         func(string)  // Callback function for selection
	unpinCallback    func(string) bool
	onPathChanged    func(string)
	parent           fyne.Window // Parent window for focus management
	closed           bool        // Prevent double-close/pop
	sink             *KeySink    // Key capturing wrapper
	matchers         *search.Provider
	listScroller     *dialogListScroller
	scrollRight      bool
	ownerFrame       dialogHighlightState
}

// NewNavigationHistoryDialog creates a new navigation history dialog
func NewNavigationHistoryDialog(
	paths []string,
	openPaths map[string]bool,
	pinnedPaths map[string]bool,
	unpinRemovesPath map[string]bool,
	lastUsed map[string]time.Time,
	keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
	matchers ...*search.Provider,
) *NavigationHistoryDialog {
	dialog := &NavigationHistoryDialog{
		allPaths:         paths,
		openPaths:        openPaths,
		pinnedPaths:      pinnedPaths,
		unpinRemovesPath: unpinRemovesPath,
		lastUsed:         lastUsed,
		debugPrint:       debugPrint,
		keyManager:       keyManager,
	}
	if len(matchers) > 0 {
		dialog.matchers = matchers[0]
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
			text := canvas.NewText("", currentAppThemeColor(fynetheme.ColorNameForeground))
			text.TextStyle = fyne.TextStyle{Monospace: true}
			text.TextSize = fynetheme.TextSize()
			return text
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			path, _ := item.(binding.String).Get()
			if path == "" {
				return
			}

			if text, ok := obj.(*canvas.Text); ok {
				text.Text = nhd.displayPath(path)
				text.TextSize = fynetheme.TextSize()
				text.Color = nhd.historyTextColor(path)
				text.Refresh()
			}
		},
	)

	// Set selection handler
	nhd.historyList.OnSelected = func(id widget.ListItemID) {
		if id < len(nhd.filteredPaths) {
			nhd.selectedIndex = int(id)
			nhd.selectedPath = nhd.filteredPaths[id]
			nhd.debugPrint("HistoryDialog: History selected: %s (index: %d)", nhd.selectedPath, nhd.selectedIndex)
			nhd.notifySelectedPathChanged()
			nhd.applyHorizontalScroll()
			// Keep focus on sink so KeyManager continues to receive keys
			if nhd.parent != nil && nhd.sink != nil {
				nhd.parent.Canvas().Focus(nhd.sink)
			}
		}
	}

	// Set list size
	nhd.historyList.Resize(searchDialogListSize())

	// Keep frecency as the default while allowing a chronological view of the
	// same candidates. Required prevents clicking the selected option from
	// leaving the dialog without an active ordering mode.
	nhd.orderRadio = widget.NewRadioGroup([]string{
		navigationHistoryOrderFrecency,
		navigationHistoryOrderLastUsed,
	}, func(selected string) {
		nhd.setOrderMode(selected)
	})
	nhd.orderRadio.Horizontal = true
	nhd.orderRadio.Required = true
	nhd.orderRadio.SetSelected(navigationHistoryOrderFrecency)
}

// updateFilteredPaths updates the filtered paths based on query
func (nhd *NavigationHistoryDialog) updateFilteredPaths(query string) {
	orderedPaths := nhd.pathsInDisplayOrder()
	if query == "" {
		nhd.filteredPaths = orderedPaths
	} else {
		matcher := nhd.matchers.Build(query)
		nhd.filteredPaths = []string{}

		for _, path := range orderedPaths {
			if matcher.Match(path) {
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
		nhd.notifySelectedPathChanged()
		nhd.applyHorizontalScroll()
	} else {
		nhd.selectedIndex = -1
		nhd.selectedPath = ""
		nhd.notifySelectedPathChanged()
	}
}

func (nhd *NavigationHistoryDialog) pathsInDisplayOrder() []string {
	paths := append([]string(nil), nhd.allPaths...)
	if nhd.orderMode != navigationHistoryOrderLastUsed {
		return paths
	}

	sort.SliceStable(paths, func(i, j int) bool {
		return nhd.lastUsed[paths[i]].After(nhd.lastUsed[paths[j]])
	})
	return paths
}

func (nhd *NavigationHistoryDialog) setOrderMode(mode string) {
	if mode != navigationHistoryOrderFrecency && mode != navigationHistoryOrderLastUsed {
		return
	}
	if nhd.orderMode == mode {
		return
	}

	nhd.orderMode = mode
	query := ""
	if nhd.searchEntry != nil {
		query = nhd.searchEntry.Text
	}
	nhd.updateFilteredPaths(query)
	nhd.debugPrint("HistoryDialog: Order mode=%s", mode)
	if nhd.parent != nil && nhd.sink != nil {
		nhd.parent.Canvas().Focus(nhd.sink)
	}
}

// SetOnSelectedPathChanged sets a callback for selection changes.
func (nhd *NavigationHistoryDialog) SetOnSelectedPathChanged(callback func(string)) {
	nhd.onPathChanged = callback
	nhd.notifySelectedPathChanged()
}

// SetOwnerHighlighted changes the accent frame around the owning dialog.
func (nhd *NavigationHistoryDialog) SetOwnerHighlighted(highlighted bool) {
	nhd.ownerFrame.setHighlighted(highlighted)
}

func (nhd *NavigationHistoryDialog) notifySelectedPathChanged() {
	if nhd.onPathChanged != nil {
		nhd.onPathChanged(nhd.selectedPath)
	}
}

// ShowDialog shows the navigation history dialog.
func (nhd *NavigationHistoryDialog) ShowDialog(parent fyne.Window, callback func(string), unpinCallback ...func(string) bool) {
	listWidth := responsiveDialogWidth(parent, searchDialogListWidth)
	contentWidth := responsiveDialogWidth(parent, searchDialogContentWidth)
	listSize := metricsSize(listWidth, searchDialogListHeight)
	contentSize := metricsSize(contentWidth, searchDialogContentHeight)

	// Create title label
	titleLabel := widget.NewLabel("Navigation History")
	titleLabel.TextStyle.Bold = true

	// Create search section
	searchLabel := widget.NewLabel("Filter:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, nhd.searchEntry)
	orderLabel := widget.NewLabel("Order (Ctrl+R):")
	orderSection := container.NewBorder(nil, nil, orderLabel, nil, nhd.orderRadio)

	// Create scrollable list container
	listScroll := newDialogListScroller(nhd.historyList, dialogTextWidth(nhd.allPaths, listWidth), listWidth, searchDialogListHeight)
	nhd.listScroller = listScroll

	// Create empty state message
	emptyLabel := widget.NewLabel("No matching paths found")
	emptyLabel.Alignment = fyne.TextAlignCenter
	emptyLabel.Hide()

	// Create a fixed-size container that maintains its size
	fixedContainer := container.NewWithoutLayout(listScroll, emptyLabel)
	fixedContainer.Resize(listSize)

	// Position elements manually to fill the container
	listScroll.Resize(listSize)
	listScroll.Move(fyne.NewPos(0, 0))

	emptyLabel.Resize(listSize)
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
		container.NewVBox(titleLabel, orderSection, searchSection),                                                      // top
		dialogButtonBar(dialogCancelButton("Cancel", nhd.CancelDialog), dialogConfirmButton("OK", nhd.AcceptSelection)), // bottom
		nil,            // left
		nil,            // right
		fixedContainer, // center - fixed size container
	)

	// Set minimum size
	content.Resize(contentSize)

	// Store callback and parent for use by key handler
	nhd.callback = callback
	if len(unpinCallback) > 0 {
		nhd.unpinCallback = unpinCallback[0]
	}
	nhd.parent = parent

	// Create and push history dialog key handler
	historyHandler := keymanager.NewHistoryDialogKeyHandler(nhd, nhd.debugPrint)
	nhd.kmToken = nhd.keyManager.PushHandler(historyHandler)

	// Wrap content with KeySink to capture Tab and forward keys
	nhd.sink = NewKeySink(content, nhd.keyManager, WithTabCapture(true))

	// Configure search entry to redirect focus to sink
	nhd.searchEntry.SetFocusRedirect(parent, nhd.sink)

	// Create custom dialog with proper focus handling (wrapped by sink)
	framedContent := nhd.ownerFrame.wrap(nhd.sink)
	nhd.dialog = dialog.NewCustomWithoutButtons("Select Directory", framedContent, parent)

	// Show dialog and ensure focus stays on sink so KeyManager gets keys
	nhd.dialog.Show()
	if nhd.parent != nil && nhd.sink != nil {
		nhd.parent.Canvas().Focus(nhd.sink)
		nhd.searchEntry.RefreshIMEAnchor()
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

// ToggleHistoryOrder switches between frecency and last-access ordering.
func (nhd *NavigationHistoryDialog) ToggleHistoryOrder() {
	if nhd.orderRadio == nil {
		return
	}
	if nhd.orderMode == navigationHistoryOrderLastUsed {
		nhd.orderRadio.SetSelected(navigationHistoryOrderFrecency)
		return
	}
	nhd.orderRadio.SetSelected(navigationHistoryOrderLastUsed)
}

// AcceptSelection accepts the current selection and closes the dialog
func (nhd *NavigationHistoryDialog) AcceptSelection() {
	if nhd.closed {
		return
	}
	nhd.closed = true

	// Check if search text is a valid path and no history match exists
	searchText := nhd.GetSearchText()
	selectedPath := ""
	if searchText != "" && len(nhd.filteredPaths) == 0 {
		if resolvedPath, ok := nhd.resolveDirectoryPath(searchText); ok {
			nhd.debugPrint("HistoryDialog: No history match, using search text as path: %s", resolvedPath)
			selectedPath = resolvedPath
		}
	} else if nhd.selectedPath != "" {
		// Use selected history path
		selectedPath = nhd.selectedPath
	}

	deferDialogClose(nhd.keyManager, "history.accept", func() {
		nhd.notifyDialogClosed()
		nhd.keyManager.RemoveHandler(nhd.kmToken)
		if nhd.callback != nil && selectedPath != "" {
			nhd.callback(selectedPath)
		}
		if nhd.dialog != nil {
			nhd.dialog.Hide()
		}
		unfocusIfDialogOwned(nhd.parent, nhd.sink, nhd.searchEntry)
	})
}

// AcceptDirectPathNavigation accepts direct path navigation (Ctrl+Enter)
func (nhd *NavigationHistoryDialog) AcceptDirectPathNavigation() {
	if nhd.closed {
		return
	}
	nhd.closed = true

	searchText := nhd.GetSearchText()
	selectedPath := ""
	if searchText != "" {
		if resolvedPath, ok := nhd.resolveDirectoryPath(searchText); ok {
			// Use search text as direct path navigation, ignoring history
			nhd.debugPrint("HistoryDialog: Direct path navigation (Ctrl+Enter): %s", resolvedPath)
			selectedPath = resolvedPath
		} else if nhd.callback != nil && nhd.selectedPath != "" {
			// Fallback to selected history path if search text is not valid
			nhd.debugPrint("HistoryDialog: Invalid search path, falling back to selected history: %s", nhd.selectedPath)
			selectedPath = nhd.selectedPath
		}
	} else if nhd.selectedPath != "" {
		// Empty search text; use selected history path.
		selectedPath = nhd.selectedPath
	}

	deferDialogClose(nhd.keyManager, "history.acceptDirect", func() {
		nhd.notifyDialogClosed()
		nhd.keyManager.RemoveHandler(nhd.kmToken)
		if nhd.callback != nil && selectedPath != "" {
			nhd.callback(selectedPath)
		}
		if nhd.dialog != nil {
			nhd.dialog.Hide()
		}
		unfocusIfDialogOwned(nhd.parent, nhd.sink, nhd.searchEntry)
	})
}

// CancelDialog cancels the dialog without selection
func (nhd *NavigationHistoryDialog) CancelDialog() {
	if nhd.closed {
		return
	}
	nhd.closed = true

	deferDialogClose(nhd.keyManager, "history.cancel", func() {
		nhd.notifyDialogClosed()
		nhd.keyManager.RemoveHandler(nhd.kmToken)
		if nhd.dialog != nil {
			nhd.dialog.Hide()
		}
		unfocusIfDialogOwned(nhd.parent, nhd.sink, nhd.searchEntry)
	})
}

func (nhd *NavigationHistoryDialog) notifyDialogClosed() {
	if nhd.onPathChanged != nil {
		nhd.onPathChanged("")
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

// PasteFromClipboard appends clipboard text to the focusless search entry.
// The search field is single-line, so match LineEditEntry's newline handling.
func (nhd *NavigationHistoryDialog) PasteFromClipboard() {
	app := fyne.CurrentApp()
	if nhd.searchEntry == nil || app == nil || app.Clipboard() == nil {
		return
	}
	text := app.Clipboard().Content()
	if text == "" {
		return
	}
	nhd.AppendToSearch(strings.ReplaceAll(text, "\n", " "))
	nhd.debugPrint("HistoryDialog: Pasted search text")
}

// BackspaceSearch removes the last character from search
func (nhd *NavigationHistoryDialog) BackspaceSearch() {
	if nhd.searchEntry != nil {
		current := nhd.searchEntry.Text
		if len(current) > 0 {
			newText := trimLastRune(current)
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

func (nhd *NavigationHistoryDialog) UnpinSelectedPath() {
	if nhd.selectedPath == "" || !nhd.pinnedPaths[nhd.selectedPath] {
		return
	}
	path := nhd.selectedPath
	nhd.debugPrint("HistoryDialog: Unpin selected path: %s", path)
	if nhd.unpinCallback != nil && !nhd.unpinCallback(path) {
		return
	}
	delete(nhd.pinnedPaths, path)
	if nhd.unpinRemovesPath[path] {
		nhd.removePath(path)
	}
	query := ""
	if nhd.searchEntry != nil {
		query = nhd.searchEntry.Text
	}
	nhd.updateFilteredPaths(query)
}

func (nhd *NavigationHistoryDialog) removePath(path string) {
	nhd.allPaths = removeString(nhd.allPaths, path)
	nhd.filteredPaths = removeString(nhd.filteredPaths, path)
}

func (nhd *NavigationHistoryDialog) ScrollSelectedRight() {
	nhd.scrollRight = true
	nhd.applyHorizontalScroll()
}

func (nhd *NavigationHistoryDialog) ResetHorizontalScroll() {
	nhd.scrollRight = false
	if nhd.listScroller != nil {
		nhd.listScroller.ResetHorizontalScroll()
	}
}

func (nhd *NavigationHistoryDialog) applyHorizontalScroll() {
	if !nhd.scrollRight || nhd.listScroller == nil || nhd.selectedPath == "" {
		return
	}
	nhd.listScroller.ScrollPathRight(nhd.selectedPath)
}

func (nhd *NavigationHistoryDialog) resolveDirectoryPath(path string) (string, bool) {
	resolved, _, err := fileinfo.CanonicalDisplayPath(path)
	if err != nil {
		nhd.debugPrint("HistoryDialog: Path is invalid: '%s' (%v)", path, err)
		return "", false
	}
	return resolved, true
}

func (nhd *NavigationHistoryDialog) historyTextColor(path string) color.Color {
	if nhd.openPaths[path] {
		themeProvider := currentThemeColorProvider()
		if themeProvider != nil {
			return themeProvider.GetCustomColor(customtheme.ColorCopyMoveOpenDestination)
		}
	}
	return currentAppThemeColor(fynetheme.ColorNameForeground)
}

func (nhd *NavigationHistoryDialog) displayPath(path string) string {
	if nhd.pinnedPaths[path] {
		return "* " + path
	}
	return path
}

func removeString(values []string, value string) []string {
	for i, entry := range values {
		if entry == value {
			return append(values[:i], values[i+1:]...)
		}
	}
	return values
}
