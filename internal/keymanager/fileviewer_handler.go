package keymanager

import "fyne.io/fyne/v2"

// FileViewerInterface defines keyboard actions for the built-in viewer.
type FileViewerInterface interface {
	CloseViewer()
	ViewerLineDown()
	ViewerLineUp()
	ViewerPageDown()
	ViewerPageUp()
	ViewerHome()
	ViewerEnd()
	ViewerSearchNext()
	ViewerSearchPrevious()
	ViewerFocusSearch()
	ViewerFocusLine()
}

// FileViewerKeyHandler handles less-like keys for the built-in viewer.
type FileViewerKeyHandler struct {
	viewer FileViewerInterface
}

func NewFileViewerKeyHandler(viewer FileViewerInterface) *FileViewerKeyHandler {
	return &FileViewerKeyHandler{viewer: viewer}
}

func (h *FileViewerKeyHandler) GetName() string { return "FileViewer" }

func (h *FileViewerKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil {
		return false
	}
	switch ev.Name {
	case fyne.KeyUp:
		h.viewer.ViewerLineUp()
	case fyne.KeyDown:
		h.viewer.ViewerLineDown()
	case fyne.KeyPageUp:
		h.viewer.ViewerPageUp()
	case fyne.KeyPageDown:
		h.viewer.ViewerPageDown()
	case fyne.KeyHome:
		h.viewer.ViewerHome()
	case fyne.KeyEnd:
		h.viewer.ViewerEnd()
	default:
		return false
	}
	return true
}

func (h *FileViewerKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}

func (h *FileViewerKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if ev == nil {
		return false
	}
	switch ev.Name {
	case fyne.KeyEscape:
		h.viewer.CloseViewer()
	case fyne.KeySpace:
		h.viewer.ViewerPageDown()
	case fyne.KeySlash:
		h.viewer.ViewerFocusSearch()
	default:
		return false
	}
	return true
}

func (h *FileViewerKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	switch r {
	case 'q':
		h.viewer.CloseViewer()
	case 'j':
		h.viewer.ViewerLineDown()
	case 'k':
		h.viewer.ViewerLineUp()
	case 'f':
		h.viewer.ViewerPageDown()
	case 'b':
		h.viewer.ViewerPageUp()
	case 'g':
		h.viewer.ViewerHome()
	case 'G':
		h.viewer.ViewerEnd()
	case 'n':
		h.viewer.ViewerSearchNext()
	case 'N':
		h.viewer.ViewerSearchPrevious()
	case '/':
		h.viewer.ViewerFocusSearch()
	case ':':
		h.viewer.ViewerFocusLine()
	default:
		return false
	}
	return true
}
