package keymanager

import (
	"unicode"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
)

// IncrementalSearchInterface defines the interface needed by IncrementalSearchKeyHandler
type IncrementalSearchInterface interface {
	// Search overlay management
	ShowIncrementalSearchOverlay()
	HideIncrementalSearchOverlay()
	IsIncrementalSearchVisible() bool

	// Search operations
	AddSearchCharacter(char rune)
	RemoveLastSearchCharacter()
	NextSearchMatch()
	PreviousSearchMatch()
	SelectCurrentSearchMatch()
	GetCurrentSearchMatch() *fileinfo.FileInfo

	// File operations
	OpenFile(file *fileinfo.FileInfo)
	SetCursorToFile(file *fileinfo.FileInfo)
}

// IncrementalSearchKeyHandler handles keyboard events during incremental search mode
type IncrementalSearchKeyHandler struct {
	searchInterface IncrementalSearchInterface
	debugPrint      func(format string, args ...interface{})
}

// NewIncrementalSearchKeyHandler creates a new incremental search key handler
func NewIncrementalSearchKeyHandler(
	si IncrementalSearchInterface,
	debugPrint func(format string, args ...interface{}),
) *IncrementalSearchKeyHandler {
	return &IncrementalSearchKeyHandler{
		searchInterface: si,
		debugPrint:      debugPrint,
	}
}

// GetName returns the name of this handler
func (ish *IncrementalSearchKeyHandler) GetName() string {
	return "IncrementalSearch"
}

// OnKeyDown handles key press events during incremental search
func (ish *IncrementalSearchKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	ish.debugPrint("IncrementalSearchKeyHandler: OnKeyDown %v", ev.Name)

	switch ev.Name {
	case fyne.KeyEscape:
		// Exit search mode
		ish.searchInterface.HideIncrementalSearchOverlay()
		return true

	case fyne.KeyReturn, fyne.KeyEnter:
		// Select current match and exit search mode
		currentMatch := ish.searchInterface.GetCurrentSearchMatch()
		if currentMatch != nil {
			// For directories, navigate into them
			// For files, just set cursor to them
			if currentMatch.IsDir {
				ish.searchInterface.OpenFile(currentMatch)
			} else {
				ish.searchInterface.SetCursorToFile(currentMatch)
			}
		}
		ish.searchInterface.HideIncrementalSearchOverlay()
		return true

	case fyne.KeyBackspace:
		// Remove last character from search term
		ish.searchInterface.RemoveLastSearchCharacter()
		return true

	case fyne.KeyUp:
		// Move to previous match
		if modifiers.ShiftPressed {
			// Shift+Up: Jump to first match
			// For now, just go to previous match multiple times
			for i := 0; i < 5; i++ {
				ish.searchInterface.PreviousSearchMatch()
			}
		} else {
			ish.searchInterface.PreviousSearchMatch()
		}
		return true

	case fyne.KeyDown:
		// Move to next match
		if modifiers.ShiftPressed {
			// Shift+Down: Jump to last match or skip several
			for i := 0; i < 5; i++ {
				ish.searchInterface.NextSearchMatch()
			}
		} else {
			ish.searchInterface.NextSearchMatch()
		}
		return true

	default:
		// Let OnTypedRune handle character input
		return false
	}
}

// OnKeyUp handles key release events during incremental search
func (ish *IncrementalSearchKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	// We don't need to handle key up events for incremental search
	return false
}

// OnTypedKey handles special key events during incremental search
func (ish *IncrementalSearchKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	ish.debugPrint("IncrementalSearchKeyHandler: OnTypedKey %v", ev.Name)

	switch ev.Name {
	case fyne.KeyEscape:
		// Exit search mode
		ish.searchInterface.HideIncrementalSearchOverlay()
		return true

	case fyne.KeyReturn, fyne.KeyEnter:
		// Select current match and exit search mode
		currentMatch := ish.searchInterface.GetCurrentSearchMatch()
		if currentMatch != nil {
			if currentMatch.IsDir {
				ish.searchInterface.OpenFile(currentMatch)
			} else {
				ish.searchInterface.SetCursorToFile(currentMatch)
			}
		}
		ish.searchInterface.HideIncrementalSearchOverlay()
		return true

	default:
		// Let other events pass through
		return false
	}
}

// OnTypedRune handles character input during incremental search
func (ish *IncrementalSearchKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	ish.debugPrint("IncrementalSearchKeyHandler: OnTypedRune '%c'", r)

	// Handle printable characters (letters, numbers, some symbols)
	if unicode.IsPrint(r) && !unicode.IsControl(r) {
		// Add character to search term
		ish.searchInterface.AddSearchCharacter(r)
		return true
	}

	// Don't handle non-printable characters
	return false
}
