package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
)

// SortDialog represents a file sorting configuration dialog
type SortDialog struct {
	sortByRadio        *widget.RadioGroup
	sortOrderRadio     *widget.RadioGroup
	directoriesFirstCB *widget.Check

	currentConfig config.SortConfig
	debugPrint    func(format string, args ...interface{})
	onApply       func(sortConfig config.SortConfig)
	onCancel      func()
	dialog        dialog.Dialog
	keyHandler    interface{} // KeyHandler interface to avoid circular import
	closed        bool        // Flag to prevent multiple close calls
	onCleanup     func()      // Cleanup callback (e.g., pop key handler)
}

// NewSortDialog creates a new sort configuration dialog
func NewSortDialog(currentConfig config.SortConfig,
	debugPrint func(format string, args ...interface{})) *SortDialog {

	sd := &SortDialog{
		currentConfig: currentConfig,
		debugPrint:    debugPrint,
	}

	sd.createWidgets()
	return sd
}

// createWidgets initializes all UI widgets
func (sd *SortDialog) createWidgets() {
	// Sort by radio group
	sd.sortByRadio = widget.NewRadioGroup([]string{
		"Name",
		"Size",
		"Modified",
		"Extension",
	}, func(selected string) {
		sd.debugPrint("Sort by selected: %s", selected)
		// Prevent deselection - ensure at least one option is always selected
		if selected == "" {
			sd.debugPrint("Preventing sort by deselection, restoring previous selection")
			sd.loadCurrentSortBySelection()
		}
	})

	// Sort order radio group
	sd.sortOrderRadio = widget.NewRadioGroup([]string{
		"Ascending",
		"Descending",
	}, func(selected string) {
		sd.debugPrint("Sort order selected: %s", selected)
		// Prevent deselection - ensure at least one option is always selected
		if selected == "" {
			sd.debugPrint("Preventing sort order deselection, restoring previous selection")
			sd.loadCurrentSortOrderSelection()
		}
	})

	// Directories first checkbox
	sd.directoriesFirstCB = widget.NewCheck("Directories first", func(checked bool) {
		sd.debugPrint("Directories first: %t", checked)
	})

	// Set current values
	sd.loadCurrentSettings()
}

// loadCurrentSettings sets the dialog widgets to match current configuration
func (sd *SortDialog) loadCurrentSettings() {
	// Set sort by
	switch sd.currentConfig.SortBy {
	case "name":
		sd.sortByRadio.SetSelected("Name")
	case "size":
		sd.sortByRadio.SetSelected("Size")
	case "modified":
		sd.sortByRadio.SetSelected("Modified")
	case "extension":
		sd.sortByRadio.SetSelected("Extension")
	default:
		sd.sortByRadio.SetSelected("Name")
	}

	// Set sort order
	if sd.currentConfig.SortOrder == "desc" {
		sd.sortOrderRadio.SetSelected("Descending")
	} else {
		sd.sortOrderRadio.SetSelected("Ascending")
	}

	// Set directories first
	sd.directoriesFirstCB.SetChecked(sd.currentConfig.DirectoriesFirst)
}

// loadCurrentSortBySelection restores the current sort by selection
func (sd *SortDialog) loadCurrentSortBySelection() {
	switch sd.currentConfig.SortBy {
	case "name":
		sd.sortByRadio.SetSelected("Name")
	case "size":
		sd.sortByRadio.SetSelected("Size")
	case "modified":
		sd.sortByRadio.SetSelected("Modified")
	case "extension":
		sd.sortByRadio.SetSelected("Extension")
	default:
		sd.sortByRadio.SetSelected("Name")
	}
}

// loadCurrentSortOrderSelection restores the current sort order selection
func (sd *SortDialog) loadCurrentSortOrderSelection() {
	if sd.currentConfig.SortOrder == "desc" {
		sd.sortOrderRadio.SetSelected("Descending")
	} else {
		sd.sortOrderRadio.SetSelected("Ascending")
	}
}

// Show displays the sort dialog
func (sd *SortDialog) Show(parent fyne.Window, keyHandler interface{}) {
	// Create content layout
	content := sd.createContent()

	// Create custom confirm dialog
	sd.dialog = dialog.NewCustomConfirm(
		"Sort Settings",
		"Apply",
		"Cancel",
		content,
		func(response bool) {
			if response {
				// Apply button was clicked
				sd.applySettings()
			} else {
				// Cancel button was clicked
				sd.cancel()
			}
		},
		parent,
	)
	sd.dialog.Resize(fyne.NewSize(400, 350))

	// Show dialog
	sd.dialog.Show()

	// Store keyboard handler for cleanup
	sd.keyHandler = keyHandler
}

// createContent creates the dialog content layout
func (sd *SortDialog) createContent() *fyne.Container {
	// Sort by section
	sortByLabel := widget.NewLabel("Sort by: (1-4)")
	sortByContainer := container.NewVBox(sortByLabel, sd.sortByRadio)

	// Sort order section
	sortOrderLabel := widget.NewLabel("")
	sortOrderLabel2 := widget.NewLabel("Order: (O)")
	sortOrderContainer := container.NewVBox(sortOrderLabel, sortOrderLabel2, sd.sortOrderRadio)

	// Options section
	optionsLabel := widget.NewLabel("")
	optionsLabel2 := widget.NewLabel("Options: (D)")
	optionsContainer := container.NewVBox(optionsLabel, optionsLabel2, sd.directoriesFirstCB)

	// Keyboard shortcuts help
	shortcutsHelp := widget.NewLabel("Shortcuts: Enter=Apply, Esc=Cancel, Tab=Navigate")
	shortcutsHelp.TextStyle.Italic = true

	// Main layout
	content := container.NewVBox(
		sortByContainer,
		widget.NewSeparator(),
		sortOrderContainer,
		widget.NewSeparator(),
		optionsContainer,
		widget.NewSeparator(),
		shortcutsHelp,
	)

	return content
}

// applySettings applies the current dialog settings
func (sd *SortDialog) applySettings() {
	if sd.closed {
		sd.debugPrint("applySettings: Dialog already closed, ignoring")
		return
	}

	sd.debugPrint("Applying sort settings")

	// Build sort config from UI
	sortConfig := config.SortConfig{
		DirectoriesFirst: sd.directoriesFirstCB.Checked,
	}

	// Convert sort by selection to config value
	switch sd.sortByRadio.Selected {
	case "Name":
		sortConfig.SortBy = "name"
	case "Size":
		sortConfig.SortBy = "size"
	case "Modified":
		sortConfig.SortBy = "modified"
	case "Extension":
		sortConfig.SortBy = "extension"
	default:
		sortConfig.SortBy = "name"
	}

	// Convert sort order selection to config value
	if sd.sortOrderRadio.Selected == "Descending" {
		sortConfig.SortOrder = "desc"
	} else {
		sortConfig.SortOrder = "asc"
	}

	sd.debugPrint("Applying sort config: %+v", sortConfig)

	// Call apply callback
	if sd.onApply != nil {
		sd.onApply(sortConfig)
	}

	sd.close()
}

// cancel cancels the dialog
func (sd *SortDialog) cancel() {
	if sd.closed {
		sd.debugPrint("cancel: Dialog already closed, ignoring")
		return
	}

	sd.debugPrint("Sort dialog cancelled")

	// Call cancel callback
	if sd.onCancel != nil {
		sd.onCancel()
	}

	sd.close()
}

// close closes the dialog and cleans up
func (sd *SortDialog) close() {
	if sd.closed {
		sd.debugPrint("close: Dialog already closed, ignoring")
		return
	}

	sd.closed = true
	sd.debugPrint("Closing sort dialog")

	// Call cleanup callback (e.g., to pop key handler)
	if sd.onCleanup != nil {
		sd.onCleanup()
	}

	// Hide dialog
	if sd.dialog != nil {
		sd.dialog.Hide()
	}
}

// SetOnApply sets the callback for when settings are applied
func (sd *SortDialog) SetOnApply(callback func(sortConfig config.SortConfig)) {
	sd.onApply = callback
}

// SetOnCancel sets the callback for when dialog is cancelled
func (sd *SortDialog) SetOnCancel(callback func()) {
	sd.onCancel = callback
}

// SetOnCleanup sets the callback for cleanup operations (e.g., popping key handler)
func (sd *SortDialog) SetOnCleanup(callback func()) {
	sd.onCleanup = callback
}

// GetCurrentSelection returns the currently selected sort configuration
func (sd *SortDialog) GetCurrentSelection() config.SortConfig {
	sortConfig := config.SortConfig{
		DirectoriesFirst: sd.directoriesFirstCB.Checked,
	}

	// Convert sort by selection to config value
	switch sd.sortByRadio.Selected {
	case "Name":
		sortConfig.SortBy = "name"
	case "Size":
		sortConfig.SortBy = "size"
	case "Modified":
		sortConfig.SortBy = "modified"
	case "Extension":
		sortConfig.SortBy = "extension"
	default:
		sortConfig.SortBy = "name"
	}

	// Convert sort order selection to config value
	if sd.sortOrderRadio.Selected == "Descending" {
		sortConfig.SortOrder = "desc"
	} else {
		sortConfig.SortOrder = "asc"
	}

	return sortConfig
}

// Keyboard handler interface methods

// MoveToPreviousField moves focus to the previous field (Tab navigation)
func (sd *SortDialog) MoveToPreviousField() {
	sd.debugPrint("Move to previous field")
	// Implementation handled by Fyne's built-in Tab navigation
}

// MoveToNextField moves focus to the next field (Tab navigation)
func (sd *SortDialog) MoveToNextField() {
	sd.debugPrint("Move to next field")
	// Implementation handled by Fyne's built-in Tab navigation
}

// ToggleCurrentField toggles the currently focused field (Space key)
func (sd *SortDialog) ToggleCurrentField() {
	sd.debugPrint("Toggle current field")
	// For radio buttons and checkboxes, this is handled by their native behavior
}

// AcceptSettings applies the current settings (Enter key)
func (sd *SortDialog) AcceptSettings() {
	sd.debugPrint("Accept settings via keyboard")
	sd.applySettings()
}

// CancelDialog cancels the dialog (Escape key)
func (sd *SortDialog) CancelDialog() {
	sd.debugPrint("Cancel dialog via keyboard")
	sd.cancel()
}

// Shortcut key methods for quick access

// SetSortByName sets sort by to Name (1 key)
func (sd *SortDialog) SetSortByName() {
	sd.debugPrint("Keyboard shortcut: Set sort by Name")
	sd.sortByRadio.SetSelected("Name")
}

// SetSortBySize sets sort by to Size (2 key)
func (sd *SortDialog) SetSortBySize() {
	sd.debugPrint("Keyboard shortcut: Set sort by Size")
	sd.sortByRadio.SetSelected("Size")
}

// SetSortByModified sets sort by to Modified (3 key)
func (sd *SortDialog) SetSortByModified() {
	sd.debugPrint("Keyboard shortcut: Set sort by Modified")
	sd.sortByRadio.SetSelected("Modified")
}

// SetSortByExtension sets sort by to Extension (4 key)
func (sd *SortDialog) SetSortByExtension() {
	sd.debugPrint("Keyboard shortcut: Set sort by Extension")
	sd.sortByRadio.SetSelected("Extension")
}

// ToggleSortOrder toggles between Ascending and Descending (O key)
func (sd *SortDialog) ToggleSortOrder() {
	sd.debugPrint("Keyboard shortcut: Toggle sort order")
	if sd.sortOrderRadio.Selected == "Ascending" {
		sd.sortOrderRadio.SetSelected("Descending")
	} else {
		sd.sortOrderRadio.SetSelected("Ascending")
	}
}

// ToggleDirectoriesFirst toggles the directories first option (D key)
func (sd *SortDialog) ToggleDirectoriesFirst() {
	sd.debugPrint("Keyboard shortcut: Toggle directories first")
	sd.directoriesFirstCB.SetChecked(!sd.directoriesFirstCB.Checked)
}
