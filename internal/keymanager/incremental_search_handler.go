package keymanager

import (
	"unicode"

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
	*dialogKeyHandler
	searchInterface IncrementalSearchInterface
	deferTransition func(label string, action func())
}

// NewIncrementalSearchKeyHandler creates a new incremental search key handler
func NewIncrementalSearchKeyHandler(
	si IncrementalSearchInterface,
	debugPrint func(format string, args ...interface{}),
) *IncrementalSearchKeyHandler {
	ish := &IncrementalSearchKeyHandler{searchInterface: si}

	ish.dialogKeyHandler = newDialogKeyHandler("IncrementalSearch", debugPrint, []dialogBinding{
		{"C-H", si.RemoveLastSearchCharacter},

		// Exit search mode.
		{"Escape", ish.cancelSearch},
		// Select current match and exit search mode.
		{"Return", ish.acceptCurrentMatch},

		// Remove last character from search term.
		{"Backspace", si.RemoveLastSearchCharacter},

		// Move to previous/next match; Shift jumps several at once.
		{"Up", si.PreviousSearchMatch},
		{"S-Up", func() {
			for i := 0; i < 5; i++ {
				si.PreviousSearchMatch()
			}
		}},
		{"Down", si.NextSearchMatch},
		{"S-Down", func() {
			for i := 0; i < 5; i++ {
				si.NextSearchMatch()
			}
		}},
	}).withRune(func(r rune, modifiers ModifierState) bool {
		// Handle printable characters (letters, numbers, some symbols).
		if unicode.IsPrint(r) && !unicode.IsControl(r) {
			si.AddSearchCharacter(r)
			return true
		}
		// Don't handle non-printable characters.
		return false
	})
	return ish
}

// SetTransitionGate configures delayed execution for exiting search mode.
func (ish *IncrementalSearchKeyHandler) SetTransitionGate(deferTransition func(label string, action func())) {
	ish.deferTransition = deferTransition
}

func (ish *IncrementalSearchKeyHandler) beginTransition(label string, action func()) {
	if ish.deferTransition != nil {
		ish.deferTransition(label, action)
		return
	}
	action()
}

func (ish *IncrementalSearchKeyHandler) cancelSearch() {
	ish.beginTransition("search.cancel", func() {
		ish.searchInterface.HideIncrementalSearchOverlay()
	})
}

func (ish *IncrementalSearchKeyHandler) acceptCurrentMatch() {
	currentMatch := ish.searchInterface.GetCurrentSearchMatch()
	ish.beginTransition("search.accept", func() {
		if currentMatch != nil {
			ish.searchInterface.SetCursorToFile(currentMatch)
		}
		ish.searchInterface.AcceptIncrementalSearchOverlay()
	})
}
