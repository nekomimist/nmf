package keymanager

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"nmf/internal/fileinfo"
)

// FileManagerInterface defines the interface needed by MainScreenKeyHandler
type FileManagerInterface interface {
	// Cursor management
	GetCurrentCursorIndex() int
	SetCursorByIndex(index int)
	RefreshCursor()

	// Directory navigation
	LoadDirectory(path string)
	GetCurrentPath() string
	GetFiles() []fileinfo.FileInfo // Returns file list

	// Selection management
	GetSelectedFiles() map[string]bool
	SetFileSelected(path string, selected bool)
	RefreshFileList()

	// State management
	SaveCursorPosition(dirPath string)

	// Window management
	OpenNewWindow()
	ShowDirectoryTreeDialog()
	ShowNavigationHistoryDialog()
}

// MainScreenKeyHandler handles keyboard events for the main file list screen
type MainScreenKeyHandler struct {
	fileManager  FileManagerInterface
	shiftPressed bool
	ctrlPressed  bool
	debugPrint   func(format string, args ...interface{})
}

// NewMainScreenKeyHandler creates a new main screen key handler
func NewMainScreenKeyHandler(fm FileManagerInterface, debugPrint func(format string, args ...interface{})) *MainScreenKeyHandler {
	return &MainScreenKeyHandler{
		fileManager: fm,
		debugPrint:  debugPrint,
	}
}

// GetName returns the name of this handler
func (mh *MainScreenKeyHandler) GetName() string {
	return "MainScreen"
}

// OnKeyDown handles key press events
func (mh *MainScreenKeyHandler) OnKeyDown(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		mh.shiftPressed = true
		mh.debugPrint("MainScreen: Shift key pressed (state: %t)", mh.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		mh.ctrlPressed = true
		mh.debugPrint("MainScreen: Ctrl key pressed (state: %t)", mh.ctrlPressed)
		return true

	case fyne.KeyN:
		// Ctrl+N - Open new window
		if mh.ctrlPressed {
			mh.fileManager.OpenNewWindow()
			return true
		}

	case fyne.KeyT:
		// Ctrl+T - Show directory tree dialog
		if mh.ctrlPressed {
			mh.fileManager.ShowDirectoryTreeDialog()
			return true
		}

	case fyne.KeyH:
		// Ctrl+H - Show navigation history dialog
		if mh.ctrlPressed {
			mh.fileManager.ShowNavigationHistoryDialog()
			return true
		}
	}

	return false
}

// OnKeyUp handles key release events
func (mh *MainScreenKeyHandler) OnKeyUp(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		mh.shiftPressed = false
		mh.debugPrint("MainScreen: Shift key released (state: %t)", mh.shiftPressed)
		return true

	case desktop.KeyControlLeft, desktop.KeyControlRight:
		mh.ctrlPressed = false
		mh.debugPrint("MainScreen: Ctrl key released (state: %t)", mh.ctrlPressed)
		return true
	}

	return false
}

// OnTypedKey handles typed key events
func (mh *MainScreenKeyHandler) OnTypedKey(ev *fyne.KeyEvent) bool {
	switch ev.Name {
	case fyne.KeyUp:
		currentIdx := mh.fileManager.GetCurrentCursorIndex()
		if mh.shiftPressed {
			// Move up 20 items or to the beginning
			mh.debugPrint("MainScreen: Shift+Up detected!")
			newIdx := currentIdx - 20
			if newIdx < 0 {
				newIdx = 0
			}
			mh.fileManager.SetCursorByIndex(newIdx)
			mh.fileManager.RefreshCursor()
		} else {
			if currentIdx > 0 {
				mh.fileManager.SetCursorByIndex(currentIdx - 1)
				mh.fileManager.RefreshCursor()
			}
		}
		return true

	case fyne.KeyDown:
		currentIdx := mh.fileManager.GetCurrentCursorIndex()
		files := mh.fileManager.GetFiles()
		if mh.shiftPressed {
			// Move down 20 items or to the end
			mh.debugPrint("MainScreen: Shift+Down detected!")
			newIdx := currentIdx + 20
			if newIdx >= len(files) {
				newIdx = len(files) - 1
			}
			mh.fileManager.SetCursorByIndex(newIdx)
			mh.fileManager.RefreshCursor()
		} else {
			if currentIdx < len(files)-1 {
				mh.fileManager.SetCursorByIndex(currentIdx + 1)
				mh.fileManager.RefreshCursor()
			}
		}
		return true

	case fyne.KeyReturn:
		currentIdx := mh.fileManager.GetCurrentCursorIndex()
		files := mh.fileManager.GetFiles()
		if currentIdx >= 0 && currentIdx < len(files) {
			fileInfo := files[currentIdx]
			if fileInfo.IsDir {
				mh.fileManager.LoadDirectory(fileInfo.Path)
			}
		}
		return true

	case fyne.KeySpace:
		currentIdx := mh.fileManager.GetCurrentCursorIndex()
		files := mh.fileManager.GetFiles()
		if currentIdx >= 0 && currentIdx < len(files) {
			fileInfo := files[currentIdx]
			// Don't allow selection of parent directory entry or deleted files
			if fileInfo.Name != ".." && fileInfo.Status != fileinfo.StatusDeleted {
				// Toggle selection state of current cursor item
				selectedFiles := mh.fileManager.GetSelectedFiles()
				mh.fileManager.SetFileSelected(fileInfo.Path, !selectedFiles[fileInfo.Path])
				mh.fileManager.RefreshFileList()

				// Move cursor to next file (same as Down key without Shift)
				if currentIdx < len(files)-1 {
					mh.fileManager.SetCursorByIndex(currentIdx + 1)
					mh.fileManager.RefreshCursor()
				}
			}
		}
		return true

	case fyne.KeyBackspace:
		parent := filepath.Dir(mh.fileManager.GetCurrentPath())
		if parent != mh.fileManager.GetCurrentPath() {
			mh.fileManager.LoadDirectory(parent)
		}
		return true

	case fyne.KeyComma:
		// Shift+Comma = '<' - Move to first item
		if mh.shiftPressed {
			files := mh.fileManager.GetFiles()
			if len(files) > 0 {
				mh.fileManager.SetCursorByIndex(0)
				mh.fileManager.RefreshCursor()
			}
		}
		return true

	case fyne.KeyPeriod:
		if mh.shiftPressed {
			// Shift+Period = '>' - Move to last item
			files := mh.fileManager.GetFiles()
			if len(files) > 0 {
				mh.fileManager.SetCursorByIndex(len(files) - 1)
				mh.fileManager.RefreshCursor()
			}
		} else {
			// Period key - Refresh current directory
			// Save current cursor position before refresh
			mh.fileManager.SaveCursorPosition(mh.fileManager.GetCurrentPath())
			mh.fileManager.LoadDirectory(mh.fileManager.GetCurrentPath())
		}
		return true

	case fyne.KeyBackTick:
		// Shift+` - Navigate to home directory
		if mh.shiftPressed {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				mh.debugPrint("MainScreen: Failed to get home directory: %v", err)
			} else {
				mh.fileManager.LoadDirectory(homeDir)
			}
		}
		return true
	}

	return false
}
