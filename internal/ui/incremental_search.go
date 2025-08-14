package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

// IncrementalSearchOverlay represents a lightweight search overlay
type IncrementalSearchOverlay struct {
	searchTerm     string              // Current search term
	matchedFiles   []fileinfo.FileInfo // Files matching the search
	currentMatch   int                 // Index of current match in matchedFiles
	container      *fyne.Container     // Main container for the overlay
	searchLabel    *widget.Label       // Shows current search term and match info
	visible        bool                // Whether the overlay is currently visible
	debugPrint     func(format string, args ...interface{})
	keyManager     *keymanager.KeyManager                // Keyboard input manager
	callback       func(selectedFile *fileinfo.FileInfo) // Callback for file selection
	cancelCallback func()                                // Callback for cancellation
	allFiles       []fileinfo.FileInfo                   // All files in current directory
	parent         fyne.Window                           // Parent window for positioning
	closed         bool                                  // Prevent double-close
	initialState   bool                                  // True when in initial state (no input yet)
}

// NewIncrementalSearchOverlay creates a new incremental search overlay
func NewIncrementalSearchOverlay(
	files []fileinfo.FileInfo,
	keyManager *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
) *IncrementalSearchOverlay {
	overlay := &IncrementalSearchOverlay{
		searchTerm:   "",
		allFiles:     files,
		matchedFiles: make([]fileinfo.FileInfo, 0),
		currentMatch: -1,
		debugPrint:   debugPrint,
		keyManager:   keyManager,
		visible:      false,
		initialState: true, // Start in initial state
	}

	overlay.createWidgets()
	return overlay
}

// createWidgets creates the UI widgets for the overlay
func (iso *IncrementalSearchOverlay) createWidgets() {
	// Create search label that shows current search term and match info
	iso.searchLabel = widget.NewLabel("")
	iso.searchLabel.TextStyle.Bold = true

	// Create background rectangle with high contrast colors
	background := canvas.NewRectangle(theme.BackgroundColor())
	// Use inverted colors for high visibility
	if isDarkTheme() {
		background.FillColor = color.RGBA{220, 220, 220, 240} // Light background
		iso.searchLabel.Importance = widget.HighImportance    // Dark text
	} else {
		background.FillColor = color.RGBA{40, 40, 40, 240}   // Dark background
		iso.searchLabel.Importance = widget.MediumImportance // Light text
	}

	// Add padding around the label
	paddedLabel := container.NewPadded(iso.searchLabel)

	// Stack background and label
	iso.container = container.NewMax(background, paddedLabel)
	iso.container.Hide() // Initially hidden
}

// isDarkTheme attempts to detect if we're using a dark theme
func isDarkTheme() bool {
	// Simple heuristic: check if background is darker than foreground
	bg := theme.BackgroundColor()
	fg := theme.ForegroundColor()

	// Convert to RGBA
	bgRGBA := color.RGBAModel.Convert(bg).(color.RGBA)
	fgRGBA := color.RGBAModel.Convert(fg).(color.RGBA)

	// Calculate luminance (simplified)
	bgLum := float64(bgRGBA.R) + float64(bgRGBA.G) + float64(bgRGBA.B)
	fgLum := float64(fgRGBA.R) + float64(fgRGBA.G) + float64(fgRGBA.B)

	return bgLum < fgLum
}

// GetContainer returns the container widget for the overlay
func (iso *IncrementalSearchOverlay) GetContainer() *fyne.Container {
	return iso.container
}

// SetCallback sets the callback function for file selection
func (iso *IncrementalSearchOverlay) SetCallback(callback func(selectedFile *fileinfo.FileInfo)) {
	iso.callback = callback
}

// SetCancelCallback sets the callback function for cancellation
func (iso *IncrementalSearchOverlay) SetCancelCallback(callback func()) {
	iso.cancelCallback = callback
}

// Show displays the overlay and enters search mode
func (iso *IncrementalSearchOverlay) Show(parent fyne.Window) {
	if iso.visible {
		return
	}

	iso.parent = parent
	iso.visible = true
	iso.closed = false
	iso.searchTerm = ""
	iso.currentMatch = -1
	iso.initialState = true // Reset to initial state

	// Update files list
	iso.updateSearch()

	// Show container
	iso.container.Show()

	// Refresh the parent window canvas to ensure layout is updated
	if parent != nil && parent.Canvas() != nil {
		parent.Canvas().Refresh(iso.container)
	}

	iso.debugPrint("IncrementalSearchOverlay: Showing overlay")
}

// Hide hides the overlay and exits search mode
func (iso *IncrementalSearchOverlay) Hide() {
	if !iso.visible || iso.closed {
		return
	}

	iso.visible = false
	iso.closed = true
	iso.container.Hide()

	// Call cancel callback if set
	if iso.cancelCallback != nil {
		iso.cancelCallback()
	}

	iso.debugPrint("IncrementalSearchOverlay: Hiding overlay")
}

// UpdateFiles updates the file list for searching
func (iso *IncrementalSearchOverlay) UpdateFiles(files []fileinfo.FileInfo) {
	iso.allFiles = files
	if iso.visible {
		iso.updateSearch()
	}
}

// AddCharacter adds a character to the search term
func (iso *IncrementalSearchOverlay) AddCharacter(char rune) {
	if !iso.visible {
		return
	}

	iso.initialState = false // No longer in initial state
	iso.searchTerm += string(char)
	iso.updateSearch()
	iso.debugPrint("IncrementalSearchOverlay: Added char '%c', term: '%s'", char, iso.searchTerm)
}

// RemoveLastCharacter removes the last character from the search term
func (iso *IncrementalSearchOverlay) RemoveLastCharacter() {
	if !iso.visible || len(iso.searchTerm) == 0 {
		return
	}

	iso.searchTerm = iso.searchTerm[:len(iso.searchTerm)-1]
	iso.updateSearch()
	iso.debugPrint("IncrementalSearchOverlay: Removed last char, term: '%s'", iso.searchTerm)
}

// NextMatch moves to the next matching file
func (iso *IncrementalSearchOverlay) NextMatch() {
	if !iso.visible || len(iso.matchedFiles) == 0 {
		return
	}

	iso.currentMatch = (iso.currentMatch + 1) % len(iso.matchedFiles)
	iso.updateDisplay()
	iso.debugPrint("IncrementalSearchOverlay: Next match, index: %d", iso.currentMatch)
}

// PreviousMatch moves to the previous matching file
func (iso *IncrementalSearchOverlay) PreviousMatch() {
	if !iso.visible || len(iso.matchedFiles) == 0 {
		return
	}

	iso.currentMatch = (iso.currentMatch - 1 + len(iso.matchedFiles)) % len(iso.matchedFiles)
	iso.updateDisplay()
	iso.debugPrint("IncrementalSearchOverlay: Previous match, index: %d", iso.currentMatch)
}

// SelectCurrentMatch selects the currently highlighted match
func (iso *IncrementalSearchOverlay) SelectCurrentMatch() {
	if !iso.visible || iso.currentMatch < 0 || iso.currentMatch >= len(iso.matchedFiles) {
		return
	}

	selectedFile := &iso.matchedFiles[iso.currentMatch]

	// Hide overlay first
	iso.Hide()

	// Call callback if set
	if iso.callback != nil {
		iso.callback(selectedFile)
	}

	iso.debugPrint("IncrementalSearchOverlay: Selected match: %s", selectedFile.Name)
}

// GetCurrentMatch returns the currently selected file, or nil if none
func (iso *IncrementalSearchOverlay) GetCurrentMatch() *fileinfo.FileInfo {
	if !iso.visible || iso.currentMatch < 0 || iso.currentMatch >= len(iso.matchedFiles) {
		return nil
	}
	return &iso.matchedFiles[iso.currentMatch]
}

// GetSearchTerm returns the current search term
func (iso *IncrementalSearchOverlay) GetSearchTerm() string {
	return iso.searchTerm
}

// IsVisible returns whether the overlay is currently visible
func (iso *IncrementalSearchOverlay) IsVisible() bool {
	return iso.visible && !iso.closed
}

// updateSearch updates the matched files based on current search term
func (iso *IncrementalSearchOverlay) updateSearch() {
	iso.matchedFiles = iso.matchedFiles[:0] // Clear slice but keep capacity

	if iso.initialState {
		// In initial state, don't populate matchedFiles yet
		// This ensures the initial message is shown
		iso.currentMatch = -1
	} else if iso.searchTerm == "" {
		// User deleted all characters - show all files
		iso.matchedFiles = append(iso.matchedFiles, iso.allFiles...)
		if len(iso.matchedFiles) > 0 {
			iso.currentMatch = 0
		} else {
			iso.currentMatch = -1
		}
	} else {
		// Find files that match the search term (case-insensitive matching)
		searchLower := strings.ToLower(iso.searchTerm)
		for _, file := range iso.allFiles {
			if strings.Index(strings.ToLower(file.Name), searchLower) != -1 {
				iso.matchedFiles = append(iso.matchedFiles, file)
			}
		}

		// Reset current match to first result if we have matches
		if len(iso.matchedFiles) > 0 {
			iso.currentMatch = 0
		} else {
			iso.currentMatch = -1
		}
	}

	iso.updateDisplay()
}

// updateDisplay updates the overlay display with current search state
func (iso *IncrementalSearchOverlay) updateDisplay() {
	if iso.initialState {
		// Show initial message when first activated
		iso.searchLabel.SetText("🔍 Incremental Search - Type to search files (↑↓: navigate, Enter: select, Esc: cancel)")
	} else if len(iso.matchedFiles) == 0 {
		if iso.searchTerm == "" {
			// User deleted all search terms - this shouldn't happen with current logic, but just in case
			iso.searchLabel.SetText("🔍 Incremental Search - Type to search files")
		} else {
			iso.searchLabel.SetText(fmt.Sprintf("🔍 Search: %s (no matches found)", iso.searchTerm))
		}
	} else {
		matchInfo := fmt.Sprintf("[%d/%d]", iso.currentMatch+1, len(iso.matchedFiles))
		if iso.searchTerm == "" {
			iso.searchLabel.SetText(fmt.Sprintf("🔍 Incremental Search %s - Type to narrow down", matchInfo))
		} else {
			currentFileName := ""
			if iso.currentMatch >= 0 && iso.currentMatch < len(iso.matchedFiles) {
				currentFileName = fmt.Sprintf(" → %s", iso.matchedFiles[iso.currentMatch].Name)
			}
			iso.searchLabel.SetText(fmt.Sprintf("🔍 Search: %s %s%s", iso.searchTerm, matchInfo, currentFileName))
		}
	}
}
