package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/keymanager"
)

// Sort dialog field indices for Tab/Shift-Tab navigation (focusless design:
// the KeySink keeps real Fyne focus; these track a virtual "current field"
// cursor highlighted via background color, matching the cursor model used
// elsewhere for keyboard-driven lists).
const (
	sortFieldSortBy = iota
	sortFieldSortOrder
	sortFieldOptions
	sortDialogFieldCount
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
	closed        bool   // Flag to prevent multiple close calls
	onCleanup     func() // Cleanup callback (e.g., pop key handler)

	keyManager *keymanager.KeyManager // Keyboard input manager
	parent     fyne.Window            // Parent window for focus management
	sink       *KeySink               // Key capturing wrapper

	fieldIndex  int // Currently highlighted field (sortField* constants)
	sortByBG    *canvas.Rectangle
	sortOrderBG *canvas.Rectangle
	optionsBG   *canvas.Rectangle
}

// NewSortDialog creates a new sort configuration dialog
func NewSortDialog(currentConfig config.SortConfig, keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{})) *SortDialog {

	sd := &SortDialog{
		currentConfig: currentConfig,
		keyManager:    keyManager,
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
		sd.debugPrint("SortDialog: Sort by selected: %s", selected)
		// Prevent deselection - ensure at least one option is always selected
		if selected == "" {
			sd.debugPrint("SortDialog: Preventing sort by deselection, restoring previous selection")
			sd.loadCurrentSortBySelection()
		}
		sd.setCurrentField(sortFieldSortBy)
	})

	// Sort order radio group
	sd.sortOrderRadio = widget.NewRadioGroup([]string{
		"Ascending",
		"Descending",
	}, func(selected string) {
		sd.debugPrint("SortDialog: Sort order selected: %s", selected)
		// Prevent deselection - ensure at least one option is always selected
		if selected == "" {
			sd.debugPrint("SortDialog: Preventing sort order deselection, restoring previous selection")
			sd.loadCurrentSortOrderSelection()
		}
		sd.setCurrentField(sortFieldSortOrder)
	})

	// Directories first checkbox
	sd.directoriesFirstCB = widget.NewCheck("Directories first", func(checked bool) {
		sd.debugPrint("SortDialog: Directories first: %t", checked)
		sd.setCurrentField(sortFieldOptions)
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
func (sd *SortDialog) Show(parent fyne.Window, keyManager *keymanager.KeyManager) {
	sd.parent = parent
	sd.keyManager = keyManager
	// Always open with the first field highlighted, regardless of any field
	// changes construction-time SetSelected/SetChecked calls triggered.
	sd.fieldIndex = sortFieldSortBy

	// Create content layout
	content := sd.createContent()
	sd.updateFieldHighlight()

	// Wrap content with KeySink to capture Tab and forward keys to KeyManager
	sd.sink = NewKeySink(content, sd.keyManager, WithTabCapture(true))

	// Create custom dialog without stock buttons (bar lives inside content)
	sd.dialog = dialog.NewCustomWithoutButtons("Sort Settings", sd.sink, parent)
	sd.dialog.Resize(metricsSize(sortDialogWidth, sortDialogHeight))

	// Show dialog and ensure focus stays on sink so KeyManager gets keys
	sd.dialog.Show()
	if sd.parent != nil && sd.sink != nil {
		sd.parent.Canvas().Focus(sd.sink)
	}
}

// createContent creates the dialog content layout
func (sd *SortDialog) createContent() *fyne.Container {
	// Sort by section
	sortByLabel := widget.NewLabel("Sort by: (1-4)")
	sd.sortByBG = canvas.NewRectangle(color.Transparent)
	sortByContainer := container.NewStack(sd.sortByBG, container.NewVBox(sortByLabel, sd.sortByRadio))

	// Sort order section
	sortOrderLabel := widget.NewLabel("")
	sortOrderLabel2 := widget.NewLabel("Order: (O)")
	sd.sortOrderBG = canvas.NewRectangle(color.Transparent)
	sortOrderContainer := container.NewStack(sd.sortOrderBG, container.NewVBox(sortOrderLabel, sortOrderLabel2, sd.sortOrderRadio))

	// Options section
	optionsLabel := widget.NewLabel("")
	optionsLabel2 := widget.NewLabel("Options: (D)")
	sd.optionsBG = canvas.NewRectangle(color.Transparent)
	optionsContainer := container.NewStack(sd.optionsBG, container.NewVBox(optionsLabel, optionsLabel2, sd.directoriesFirstCB))

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
		dialogButtonBar(dialogCancelButton("Cancel", sd.CancelDialog), dialogConfirmButton("Apply", sd.AcceptSettings)),
	)

	return content
}

// applySettings applies the current dialog settings
func (sd *SortDialog) applySettings() {
	if sd.closed {
		sd.debugPrint("SortDialog: Dialog already closed in applySettings, ignoring")
		return
	}

	sd.debugPrint("SortDialog: Applying sort settings")

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

	sd.debugPrint("SortDialog: Applying sort config: %+v", sortConfig)

	// Call apply callback
	if sd.onApply != nil {
		sd.onApply(sortConfig)
	}

	sd.close()
}

// cancel cancels the dialog
func (sd *SortDialog) cancel() {
	if sd.closed {
		sd.debugPrint("SortDialog: Dialog already closed in cancel, ignoring")
		return
	}

	sd.debugPrint("SortDialog: Sort dialog cancelled")

	// Call cancel callback
	if sd.onCancel != nil {
		sd.onCancel()
	}

	sd.close()
}

// close closes the dialog and cleans up
func (sd *SortDialog) close() {
	if sd.closed {
		sd.debugPrint("SortDialog: Dialog already closed in close, ignoring")
		return
	}

	sd.closed = true
	sd.debugPrint("SortDialog: Closing sort dialog")

	// Call cleanup callback (e.g., to pop key handler)
	if sd.onCleanup != nil {
		sd.onCleanup()
	}

	// Hide dialog
	if sd.dialog != nil {
		sd.dialog.Hide()
	}
	unfocusIfDialogOwned(sd.parent, sd.sink)
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
//
// The dialog keeps real Fyne focus on its KeySink at all times (focusless
// design, matching FilterDialog/TreeDialog): Tab/Shift-Tab move a virtual
// "current field" cursor between the three field groups, highlighted with a
// background color, instead of moving native widget focus. Native focus
// traversal is never used here: RadioGroup doesn't implement fyne.Focusable
// at the group level (only its unexported per-option radioItem does, and
// radioItem.TypedKey is a no-op), so real Tab-driven traversal would land
// keyboard focus on a widget that silently swallows every further key press.

// setCurrentField sets the highlighted field and refreshes the highlight.
// Also called from mouse-driven OnChanged callbacks so the keyboard cursor
// follows mouse interaction, and refocuses the sink so KeyManager keeps
// receiving keys after the click.
func (sd *SortDialog) setCurrentField(field int) {
	sd.fieldIndex = field
	sd.updateFieldHighlight()
	sd.refocusSink()
}

// refocusSink returns real Fyne focus to the KeySink after a mouse click on
// an inner widget, mirroring the pattern used by FilterDialog/TreeDialog.
func (sd *SortDialog) refocusSink() {
	if sd.parent != nil && sd.sink != nil {
		sd.parent.Canvas().Focus(sd.sink)
	}
}

// applyFieldHighlight sets a field's background color to reflect whether it
// is the current field. Safe to call before the backgrounds are created.
func (sd *SortDialog) applyFieldHighlight(rect *canvas.Rectangle, active bool) {
	if rect == nil {
		return
	}
	if active {
		rect.FillColor = currentAppThemeColor(theme.ColorNameFocus)
	} else {
		rect.FillColor = color.Transparent
	}
	rect.Refresh()
}

// updateFieldHighlight refreshes all field backgrounds from sd.fieldIndex.
func (sd *SortDialog) updateFieldHighlight() {
	sd.applyFieldHighlight(sd.sortByBG, sd.fieldIndex == sortFieldSortBy)
	sd.applyFieldHighlight(sd.sortOrderBG, sd.fieldIndex == sortFieldSortOrder)
	sd.applyFieldHighlight(sd.optionsBG, sd.fieldIndex == sortFieldOptions)
}

// MoveToPreviousField moves the field cursor to the previous field (Shift-Tab)
func (sd *SortDialog) MoveToPreviousField() {
	sd.fieldIndex = (sd.fieldIndex - 1 + sortDialogFieldCount) % sortDialogFieldCount
	sd.debugPrint("SortDialog: Move to previous field (index=%d)", sd.fieldIndex)
	sd.updateFieldHighlight()
}

// MoveToNextField moves the field cursor to the next field (Tab)
func (sd *SortDialog) MoveToNextField() {
	sd.fieldIndex = (sd.fieldIndex + 1) % sortDialogFieldCount
	sd.debugPrint("SortDialog: Move to next field (index=%d)", sd.fieldIndex)
	sd.updateFieldHighlight()
}

// ToggleCurrentField toggles/cycles the currently highlighted field (Space key)
func (sd *SortDialog) ToggleCurrentField() {
	sd.debugPrint("SortDialog: Toggle current field (index=%d)", sd.fieldIndex)
	switch sd.fieldIndex {
	case sortFieldSortBy:
		sd.cycleSortBy()
	case sortFieldSortOrder:
		sd.ToggleSortOrder()
	case sortFieldOptions:
		sd.ToggleDirectoriesFirst()
	}
}

// cycleSortBy advances the sort-by radio group to its next option, used by
// Space when the sort-by field is highlighted.
func (sd *SortDialog) cycleSortBy() {
	options := sd.sortByRadio.Options
	if len(options) == 0 {
		return
	}
	idx := 0
	for i, opt := range options {
		if opt == sd.sortByRadio.Selected {
			idx = i
			break
		}
	}
	next := options[(idx+1)%len(options)]
	sd.debugPrint("SortDialog: Cycle sort by to %s", next)
	sd.sortByRadio.SetSelected(next)
}

// AcceptSettings applies the current settings (Enter key)
func (sd *SortDialog) AcceptSettings() {
	sd.debugPrint("SortDialog: Accept settings via keyboard")
	sd.applySettings()
}

// CancelDialog cancels the dialog (Escape key)
func (sd *SortDialog) CancelDialog() {
	sd.debugPrint("SortDialog: Cancel dialog via keyboard")
	sd.cancel()
}

// Shortcut key methods for quick access

// SetSortByName sets sort by to Name (1 key)
func (sd *SortDialog) SetSortByName() {
	sd.debugPrint("SortDialog: Keyboard shortcut: Set sort by Name")
	sd.sortByRadio.SetSelected("Name")
}

// SetSortBySize sets sort by to Size (2 key)
func (sd *SortDialog) SetSortBySize() {
	sd.debugPrint("SortDialog: Keyboard shortcut: Set sort by Size")
	sd.sortByRadio.SetSelected("Size")
}

// SetSortByModified sets sort by to Modified (3 key)
func (sd *SortDialog) SetSortByModified() {
	sd.debugPrint("SortDialog: Keyboard shortcut: Set sort by Modified")
	sd.sortByRadio.SetSelected("Modified")
}

// SetSortByExtension sets sort by to Extension (4 key)
func (sd *SortDialog) SetSortByExtension() {
	sd.debugPrint("SortDialog: Keyboard shortcut: Set sort by Extension")
	sd.sortByRadio.SetSelected("Extension")
}

// ToggleSortOrder toggles between Ascending and Descending (O key)
func (sd *SortDialog) ToggleSortOrder() {
	sd.debugPrint("SortDialog: Keyboard shortcut: Toggle sort order")
	if sd.sortOrderRadio.Selected == "Ascending" {
		sd.sortOrderRadio.SetSelected("Descending")
	} else {
		sd.sortOrderRadio.SetSelected("Ascending")
	}
}

// ToggleDirectoriesFirst toggles the directories first option (D key)
func (sd *SortDialog) ToggleDirectoriesFirst() {
	sd.debugPrint("SortDialog: Keyboard shortcut: Toggle directories first")
	sd.directoriesFirstCB.SetChecked(!sd.directoriesFirstCB.Checked)
}
