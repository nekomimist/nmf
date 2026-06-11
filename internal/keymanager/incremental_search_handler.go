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
	AcceptIncrementalSearchOverlay()
	IsIncrementalSearchVisible() bool

	// Search operations
	AddSearchCharacter(char rune)
	RemoveLastSearchCharacter()
	NextSearchMatch()
	PreviousSearchMatch()
	GetCurrentSearchMatch() *fileinfo.FileInfo

	// File operations
	SetCursorToFile(file *fileinfo.FileInfo)
}

// IncrementalSearchKeyHandler handles keyboard events during incremental search mode
type IncrementalSearchKeyHandler struct {
	searchInterface IncrementalSearchInterface
	debugPrint      func(format string, args ...interface{})
	deferTransition func(label string, action func())
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

// SetTransitionGate configures delayed execution for exiting search mode.
func (ish *IncrementalSearchKeyHandler) SetTransitionGate(deferTransition func(label string, action func())) {
	ish.deferTransition = deferTransition
}

func (ish *IncrementalSearchKeyHandler) deferUntilKeysReleased(label string, action func()) {
	if ish.deferTransition != nil {
		ish.deferTransition(label, action)
		return
	}
	action()
}

func (ish *IncrementalSearchKeyHandler) cancelSearch() {
	ish.deferUntilKeysReleased("search.cancel", func() {
		ish.searchInterface.HideIncrementalSearchOverlay()
	})
}

func (ish *IncrementalSearchKeyHandler) acceptCurrentMatch() {
	currentMatch := ish.searchInterface.GetCurrentSearchMatch()
	ish.deferUntilKeysReleased("search.accept", func() {
		if currentMatch != nil {
			ish.searchInterface.SetCursorToFile(currentMatch)
		}
		ish.searchInterface.AcceptIncrementalSearchOverlay()
	})
}

// OnKeyActivated handles key activations during incremental search
func (ish *IncrementalSearchKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	ish.debugPrint("IncrementalSearchKeyHandler: OnKeyActivated %v", ev.Name)

	switch ev.Name {
	case fyne.KeyH:
		if modifiers.CtrlPressed {
			ish.searchInterface.RemoveLastSearchCharacter()
			return true
		}

	case fyne.KeyEscape:
		// Exit search mode
		ish.cancelSearch()
		return true

	case fyne.KeyReturn, fyne.KeyEnter:
		// Select current match and exit search mode
		ish.acceptCurrentMatch()
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
	}

	// Let OnTypedRune handle character input
	return false
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
