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
	ViewerColumnLeft()
	ViewerColumnRight()
	ViewerToggleWrap()
	ViewerSearchNext()
	ViewerSearchPrevious()
	ViewerFocusSearch()
	ViewerFocusLine()
	ViewerCopySelection()
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
	if modifiers.CtrlPressed && ev.Name == fyne.KeyC {
		h.viewer.ViewerCopySelection()
		return true
	}
	switch ev.Name {
	case fyne.KeyUp:
		h.viewer.ViewerLineUp()
	case fyne.KeyDown:
		h.viewer.ViewerLineDown()
	case fyne.KeyLeft:
		h.viewer.ViewerColumnLeft()
	case fyne.KeyRight:
		h.viewer.ViewerColumnRight()
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
	case 'h':
		h.viewer.ViewerColumnLeft()
	case 'l':
		h.viewer.ViewerColumnRight()
	case 'f':
		h.viewer.ViewerPageDown()
	case 'b':
		h.viewer.ViewerPageUp()
	case 'g':
		h.viewer.ViewerHome()
	case 'G':
		h.viewer.ViewerEnd()
	case 'w':
		h.viewer.ViewerToggleWrap()
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
