package ui

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// NavigationHistoryDialog represents a navigation history dialog with search
type NavigationHistoryDialog struct {
	searchEntry   *widget.Entry
	historyList   *widget.List
	selectedPath  string
	filteredPaths []string
	allPaths      []string
	lastUsed      map[string]time.Time
	dataBinding   binding.StringList
	debugPrint    func(format string, args ...interface{})
	keyManager    *keymanager.KeyManager // Keyboard input manager
	dialog        dialog.Dialog          // Reference to the actual dialog
	callback      func(string)           // Callback function for selection
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
	// Create search entry
	nhd.searchEntry = widget.NewEntry()
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
			nhd.selectedPath = nhd.filteredPaths[id]
			nhd.debugPrint("History selected: %s", nhd.selectedPath)
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

	// Store callback for use by key handler
	nhd.callback = callback

	// Create and push history dialog key handler
	historyHandler := keymanager.NewHistoryDialogKeyHandler(nhd, nhd.debugPrint)
	nhd.keyManager.PushHandler(historyHandler)

	// Create custom dialog with proper focus handling
	nhd.dialog = dialog.NewCustomConfirm(
		"Select Directory",
		"OK",
		"Cancel",
		content,
		func(response bool) {
			// Pop the history handler when dialog closes
			nhd.keyManager.PopHandler()

			// Remove focus when dialog is closed
			parent.Canvas().Unfocus()

			if response && nhd.selectedPath != "" {
				callback(nhd.selectedPath)
			}
		},
		parent,
	)

	// Show dialog
	nhd.dialog.Show()

	// Set focus to search entry (safe approach - check if it's focusable)
	if nhd.searchEntry != nil {
		parent.Canvas().Focus(nhd.searchEntry)
	}
}

// HistoryDialogInterface implementation methods

// MoveUp moves the selection up in the history list
func (nhd *NavigationHistoryDialog) MoveUp() {
	if nhd.historyList != nil {
		// Get current selection and move up
		// Note: This is a placeholder implementation
		// Actual implementation would depend on the widget's selection handling
		nhd.debugPrint("HistoryDialog: Move up")
	}
}

// MoveDown moves the selection down in the history list
func (nhd *NavigationHistoryDialog) MoveDown() {
	if nhd.historyList != nil {
		nhd.debugPrint("HistoryDialog: Move down")
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

// FocusSearch focuses the search entry
func (nhd *NavigationHistoryDialog) FocusSearch() {
	if nhd.searchEntry != nil {
		nhd.searchEntry.FocusGained()
		nhd.debugPrint("HistoryDialog: Focus search")
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
	// Pop the handler first
	nhd.keyManager.PopHandler()

	if nhd.callback != nil && nhd.selectedPath != "" {
		nhd.callback(nhd.selectedPath)
	}
	if nhd.dialog != nil {
		nhd.dialog.Hide()
	}
}

// CancelDialog cancels the dialog without selection
func (nhd *NavigationHistoryDialog) CancelDialog() {
	// Pop the handler first
	nhd.keyManager.PopHandler()

	if nhd.dialog != nil {
		nhd.dialog.Hide()
	}
}
