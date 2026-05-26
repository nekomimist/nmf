package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
	customtheme "nmf/internal/theme"
)

// IncrementalSearchOverlay represents a lightweight search overlay
type IncrementalSearchOverlay struct {
	searchTerm     string              // Current search term
	matchedFiles   []fileinfo.FileInfo // Files matching the search
	currentMatch   int                 // Index of current match in matchedFiles
	container      *fyne.Container     // Main container for the overlay
	searchLabel    *canvas.Text        // Shows current search term and match info
	searchText     *shrinkingTextLabel // Keeps long match names from expanding the window
	visible        bool                // Whether the overlay is currently visible
	debugPrint     func(format string, args ...interface{})
	keyManager     *keymanager.KeyManager // Keyboard input manager
	cancelCallback func()                 // Callback for cancellation
	allFiles       []fileinfo.FileInfo    // All files in current directory
	parent         fyne.Window            // Parent window for positioning
	closed         bool                   // Prevent double-close
	themeProvider  ThemeColorProvider
	matchers       *search.Provider
}

// NewIncrementalSearchOverlay creates a new incremental search overlay
func NewIncrementalSearchOverlay(
	files []fileinfo.FileInfo,
	keyManager *keymanager.KeyManager,
	themeProvider ThemeColorProvider,
	debugPrint func(format string, args ...interface{}),
	matchers ...*search.Provider,
) *IncrementalSearchOverlay {
	overlay := &IncrementalSearchOverlay{
		searchTerm:    "",
		allFiles:      files,
		matchedFiles:  make([]fileinfo.FileInfo, 0),
		currentMatch:  -1,
		debugPrint:    debugPrint,
		keyManager:    keyManager,
		visible:       false,
		themeProvider: themeProvider,
	}
	if len(matchers) > 0 {
		overlay.matchers = matchers[0]
	}

	overlay.createWidgets()
	return overlay
}

// createWidgets creates the UI widgets for the overlay
func (iso *IncrementalSearchOverlay) createWidgets() {
	backgroundColor := iso.overlayColor(customtheme.ColorSearchOverlayBackground)
	textColor := iso.overlayColor(customtheme.ColorSearchOverlayForeground)
	background := canvas.NewRectangle(backgroundColor)

	// Create search text with explicit contrast so theme primary/cursor colors cannot leak in.
	iso.searchLabel = canvas.NewText("", textColor)
	iso.searchLabel.TextStyle.Bold = true
	iso.searchText = newShrinkingTextLabel(iso.searchLabel)

	// Add padding around the label
	paddedLabel := container.NewPadded(iso.searchText)

	// Stack background and label
	iso.container = container.NewMax(background, paddedLabel)
	iso.container.Hide() // Initially hidden
}

func (iso *IncrementalSearchOverlay) overlayColor(name string) color.RGBA {
	if iso.themeProvider == nil {
		return color.RGBA{}
	}
	return iso.themeProvider.GetCustomColor(name)
}

// GetContainer returns the container widget for the overlay
func (iso *IncrementalSearchOverlay) GetContainer() *fyne.Container {
	return iso.container
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

	// Update files list
	iso.updateSearch()

	// Show container
	iso.container.Show()

	// Refresh the parent window canvas to ensure layout is updated
	if parent != nil && parent.Canvas() != nil {
		parent.Canvas().Refresh(iso.container)
	}

	iso.debugPrint("IncrementalSearchOverlay: Showing overlay")
	iso.updateIMEAnchor()
}

// Hide hides the overlay and exits search mode
func (iso *IncrementalSearchOverlay) Hide() {
	iso.hide(true)
}

// HideAccepted hides the overlay after a successful search selection.
func (iso *IncrementalSearchOverlay) HideAccepted() {
	iso.hide(false)
}

func (iso *IncrementalSearchOverlay) hide(cancel bool) {
	if !iso.visible || iso.closed {
		return
	}

	iso.visible = false
	iso.closed = true
	iso.container.Hide()

	// Call cancel callback if this was an explicit cancellation.
	if cancel && iso.cancelCallback != nil {
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

	iso.searchTerm += string(char)
	iso.updateSearch()
	iso.debugPrint("IncrementalSearchOverlay: Added char '%c', term: '%s'", char, iso.searchTerm)
}

// RemoveLastCharacter removes the last character from the search term
func (iso *IncrementalSearchOverlay) RemoveLastCharacter() {
	if !iso.visible || len(iso.searchTerm) == 0 {
		return
	}

	iso.searchTerm = trimLastRune(iso.searchTerm)
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

	if iso.searchTerm == "" {
		// User deleted all characters - show all files
		iso.matchedFiles = append(iso.matchedFiles, iso.allFiles...)
		if len(iso.matchedFiles) > 0 {
			iso.currentMatch = 0
		} else {
			iso.currentMatch = -1
		}
	} else {
		// Find files that match the search term (case-insensitive matching)
		matcher := iso.matchers.Build(iso.searchTerm)
		for _, file := range iso.allFiles {
			if matcher.Match(file.Name) {
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
	if len(iso.matchedFiles) == 0 {
		if iso.searchTerm == "" {
			iso.setSearchText("🔍 Type to narrow down")
		} else {
			iso.setSearchText(fmt.Sprintf("🔍 Search: %s (no matches found)", iso.searchTerm))
		}
	} else {
		matchInfo := fmt.Sprintf("[%d/%d]", iso.currentMatch+1, len(iso.matchedFiles))
		if iso.searchTerm == "" {
			iso.setSearchText(fmt.Sprintf("🔍 Type to narrow down %s", matchInfo))
		} else {
			currentFileName := ""
			if iso.currentMatch >= 0 && iso.currentMatch < len(iso.matchedFiles) {
				currentFileName = fmt.Sprintf(" → %s", iso.matchedFiles[iso.currentMatch].Name)
			}
			iso.setSearchText(fmt.Sprintf("🔍 Search: %s %s%s", iso.searchTerm, matchInfo, currentFileName))
		}
	}
}

func (iso *IncrementalSearchOverlay) setSearchText(text string) {
	iso.searchText.SetText(text)
	iso.updateIMEAnchor()
}

func (iso *IncrementalSearchOverlay) updateIMEAnchor() {
	if !iso.visible || iso.parent == nil || iso.searchText == nil {
		return
	}
	setIMEAnchorAtTextEnd(iso.parent, iso.searchText, iso.searchText.fullText, iso.searchLabel.TextStyle)
}

type shrinkingTextLabel struct {
	widget.BaseWidget
	text     *canvas.Text
	fullText string
}

func newShrinkingTextLabel(text *canvas.Text) *shrinkingTextLabel {
	label := &shrinkingTextLabel{
		text:     text,
		fullText: text.Text,
	}
	label.ExtendBaseWidget(label)
	return label
}

func (l *shrinkingTextLabel) SetText(text string) {
	l.fullText = text
	l.text.Text = text
	l.Refresh()
}

func (l *shrinkingTextLabel) displayText(width float32) string {
	if width <= 0 {
		return l.fullText
	}
	if textWidth(l.fullText, l.text.TextSize, l.text.TextStyle) <= width {
		return l.fullText
	}

	const ellipsis = "..."
	if textWidth(ellipsis, l.text.TextSize, l.text.TextStyle) > width {
		return ""
	}

	runes := []rune(l.fullText)
	for keep := len(runes) - 1; keep > 0; keep-- {
		prefix := keep / 2
		suffix := keep - prefix
		candidate := string(runes[:prefix]) + ellipsis + string(runes[len(runes)-suffix:])
		if textWidth(candidate, l.text.TextSize, l.text.TextStyle) <= width {
			return candidate
		}
	}
	return ellipsis
}

func (l *shrinkingTextLabel) CreateRenderer() fyne.WidgetRenderer {
	return &shrinkingTextLabelRenderer{label: l}
}

type shrinkingTextLabelRenderer struct {
	label *shrinkingTextLabel
}

func (r *shrinkingTextLabelRenderer) Layout(size fyne.Size) {
	r.label.text.Text = r.label.displayText(size.Width)
	textSize := r.label.text.MinSize()
	r.label.text.Move(fyne.NewPos(0, (size.Height-textSize.Height)/2))
	r.label.text.Resize(fyne.NewSize(size.Width, textSize.Height))
}

func (r *shrinkingTextLabelRenderer) MinSize() fyne.Size {
	textSize := fyne.MeasureText("M", r.label.text.TextSize, r.label.text.TextStyle)
	return fyne.NewSize(0, textSize.Height)
}

func (r *shrinkingTextLabelRenderer) Refresh() {
	r.Layout(r.label.Size())
	canvas.Refresh(r.label)
}

func (r *shrinkingTextLabelRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.label.text}
}

func (r *shrinkingTextLabelRenderer) Destroy() {}
