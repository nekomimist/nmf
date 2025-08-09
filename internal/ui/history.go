package ui

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
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
}

// NewNavigationHistoryDialog creates a new navigation history dialog
func NewNavigationHistoryDialog(
	paths []string,
	lastUsed map[string]time.Time,
	debugPrint func(format string, args ...interface{}),
) *NavigationHistoryDialog {
	dialog := &NavigationHistoryDialog{
		allPaths:   paths,
		lastUsed:   lastUsed,
		debugPrint: debugPrint,
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

	// Create custom dialog with proper focus handling
	d := dialog.NewCustomConfirm(
		"Select Directory",
		"OK",
		"Cancel",
		content,
		func(response bool) {
			// Remove focus when dialog is closed
			parent.Canvas().Unfocus()

			if response && nhd.selectedPath != "" {
				callback(nhd.selectedPath)
			}
		},
		parent,
	)

	// Show dialog
	d.Show()

	// Set focus to search entry (safe approach - check if it's focusable)
	if nhd.searchEntry != nil {
		parent.Canvas().Focus(nhd.searchEntry)
	}
}
