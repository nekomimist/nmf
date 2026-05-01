package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"nmf/internal/fileinfo"
)

type noopHandler struct{}

func (noopHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool  { return false }
func (noopHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool    { return false }
func (noopHandler) OnTypedKey(_ *fyne.KeyEvent, _ ModifierState) bool { return false }
func (noopHandler) OnTypedRune(_ rune, _ ModifierState) bool          { return false }
func (noopHandler) GetName() string                                   { return "noop" }

type recordingHandler struct {
	typedKeys []fyne.KeyName
}

func (h *recordingHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool { return false }
func (h *recordingHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool   { return false }
func (h *recordingHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	h.typedKeys = append(h.typedKeys, ev.Name)
	return true
}
func (h *recordingHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *recordingHandler) GetName() string                          { return "recording" }

type popOnKeyDownHandler struct {
	km *KeyManager
}

func (h *popOnKeyDownHandler) OnKeyDown(ev *fyne.KeyEvent, _ ModifierState) bool {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
		h.km.PopHandler()
		return true
	}
	return false
}
func (h *popOnKeyDownHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool    { return false }
func (h *popOnKeyDownHandler) OnTypedKey(_ *fyne.KeyEvent, _ ModifierState) bool { return false }
func (h *popOnKeyDownHandler) OnTypedRune(_ rune, _ ModifierState) bool          { return false }
func (h *popOnKeyDownHandler) GetName() string                                   { return "popOnKeyDown" }

type popOnTypedKeyHandler struct {
	km *KeyManager
}

func (h *popOnTypedKeyHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool { return false }
func (h *popOnTypedKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool   { return false }
func (h *popOnTypedKeyHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
		h.km.PopHandler()
		return true
	}
	return false
}
func (h *popOnTypedKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *popOnTypedKeyHandler) GetName() string                          { return "popOnTypedKey" }

type transientStackChangeOnTypedKeyHandler struct {
	km        *KeyManager
	typedKeys []fyne.KeyName
}

func (h *transientStackChangeOnTypedKeyHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *transientStackChangeOnTypedKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *transientStackChangeOnTypedKeyHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	h.typedKeys = append(h.typedKeys, ev.Name)
	h.km.PushHandler(noopHandler{})
	h.km.PopHandler()
	return true
}
func (h *transientStackChangeOnTypedKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool {
	return false
}
func (h *transientStackChangeOnTypedKeyHandler) GetName() string {
	return "transientStackChange"
}

func TestKeyManagerTracksAltModifier(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	km.PushHandler(noopHandler{})

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyAltLeft})
	if !km.GetModifierState().AltPressed {
		t.Fatal("AltPressed should be true after Alt key down")
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: desktop.KeyAltLeft})
	if km.GetModifierState().AltPressed {
		t.Fatal("AltPressed should be false after Alt key up")
	}
}

func TestKeyManagerDrainsTypedKeyAfterKeyDownPop(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	main := &recordingHandler{}
	km.PushHandler(main)
	km.PushHandler(&popOnKeyDownHandler{km: km})

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyEnter})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 0 {
		t.Fatalf("typed keys leaked to main while Enter was held: %v", main.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyReturn {
		t.Fatalf("typed keys after drain clear = %v, want [Return]", main.typedKeys)
	}
}

func TestKeyManagerDrainsRepeatedTypedKeyAfterTypedKeyPop(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	main := &recordingHandler{}
	km.PushHandler(main)
	km.PushHandler(&popOnTypedKeyHandler{km: km})

	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 0 {
		t.Fatalf("repeated typed key leaked to main while Enter was held: %v", main.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyReturn {
		t.Fatalf("typed keys after drain clear = %v, want [Return]", main.typedKeys)
	}
}

func TestKeyManagerDoesNotDrainAfterTransientStackChange(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	main := &transientStackChangeOnTypedKeyHandler{km: km}
	km.PushHandler(main)

	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyTab})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})

	if len(main.typedKeys) != 2 {
		t.Fatalf("typed keys = %v, want two Tab events", main.typedKeys)
	}
}

type mainScreenFakeFileManager struct {
	showJobsCount          int
	showDirectoryJumpCount int
	showRenameCount        int
	showDeleteCount        int
	showExplorerMenuCount  int
	deletePermanent        bool
}

func (f *mainScreenFakeFileManager) GetCurrentCursorIndex() int                 { return 0 }
func (f *mainScreenFakeFileManager) SetCursorByIndex(index int)                 {}
func (f *mainScreenFakeFileManager) RefreshCursor()                             {}
func (f *mainScreenFakeFileManager) LoadDirectory(path string)                  {}
func (f *mainScreenFakeFileManager) GetCurrentPath() string                     { return "" }
func (f *mainScreenFakeFileManager) GetFiles() []fileinfo.FileInfo              { return nil }
func (f *mainScreenFakeFileManager) GetSelectedFiles() map[string]bool          { return nil }
func (f *mainScreenFakeFileManager) SetFileSelected(path string, selected bool) {}
func (f *mainScreenFakeFileManager) RefreshFileList()                           {}
func (f *mainScreenFakeFileManager) SaveCursorPosition(dirPath string)          {}
func (f *mainScreenFakeFileManager) OpenNewWindow()                             {}
func (f *mainScreenFakeFileManager) ShowDirectoryTreeDialog()                   {}
func (f *mainScreenFakeFileManager) ShowNavigationHistoryDialog()               {}
func (f *mainScreenFakeFileManager) ShowDirectoryJumpDialog() {
	f.showDirectoryJumpCount++
}
func (f *mainScreenFakeFileManager) ShowFilterDialog()                {}
func (f *mainScreenFakeFileManager) ClearFilter()                     {}
func (f *mainScreenFakeFileManager) ToggleFilter()                    {}
func (f *mainScreenFakeFileManager) ShowIncrementalSearchDialog()     {}
func (f *mainScreenFakeFileManager) ShowSortDialog()                  {}
func (f *mainScreenFakeFileManager) ShowJobsDialog()                  { f.showJobsCount++ }
func (f *mainScreenFakeFileManager) FocusPathEntry()                  {}
func (f *mainScreenFakeFileManager) QuitApplication()                 {}
func (f *mainScreenFakeFileManager) OpenFile(file *fileinfo.FileInfo) {}
func (f *mainScreenFakeFileManager) ShowCopyDialog()                  {}
func (f *mainScreenFakeFileManager) ShowMoveDialog()                  {}
func (f *mainScreenFakeFileManager) ShowRenameDialog()                { f.showRenameCount++ }
func (f *mainScreenFakeFileManager) ShowDeleteDialog(permanent bool) {
	f.showDeleteCount++
	f.deletePermanent = permanent
}
func (f *mainScreenFakeFileManager) ShowExplorerContextMenu() { f.showExplorerMenuCount++ }

func TestMainScreenShiftJShowsDirectoryJumpDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyJ}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+J should be handled")
	}
	if fm.showDirectoryJumpCount != 1 {
		t.Fatalf("ShowDirectoryJumpDialog count = %d, want 1", fm.showDirectoryJumpCount)
	}
	if fm.showJobsCount != 0 {
		t.Fatalf("ShowJobsDialog count = %d, want 0", fm.showJobsCount)
	}
}

func TestMainScreenCtrlJStillShowsJobsDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyJ}, ModifierState{CtrlPressed: true, ShiftPressed: true})

	if !handled {
		t.Fatal("Ctrl+J should be handled")
	}
	if fm.showJobsCount != 1 {
		t.Fatalf("ShowJobsDialog count = %d, want 1", fm.showJobsCount)
	}
	if fm.showDirectoryJumpCount != 0 {
		t.Fatalf("ShowDirectoryJumpDialog count = %d, want 0", fm.showDirectoryJumpCount)
	}
}

func TestMainScreenF2ShowsRenameDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyF2}, ModifierState{})

	if !handled {
		t.Fatal("F2 should be handled")
	}
	if fm.showRenameCount != 1 {
		t.Fatalf("ShowRenameDialog count = %d, want 1", fm.showRenameCount)
	}
}

func TestMainScreenRKeyUpShowsRenameDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyUp(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{})

	if !handled {
		t.Fatal("R key up should be handled")
	}
	if fm.showRenameCount != 1 {
		t.Fatalf("ShowRenameDialog count = %d, want 1", fm.showRenameCount)
	}
}

func TestMainScreenModifiedRenameKeysDoNotShowRenameDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyF2}, ModifierState{CtrlPressed: true})
	handler.OnKeyUp(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{ShiftPressed: true})

	if fm.showRenameCount != 0 {
		t.Fatalf("ShowRenameDialog count = %d, want 0", fm.showRenameCount)
	}
}

func TestMainScreenTabShowsExplorerContextMenu(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{})

	if !handled {
		t.Fatal("Tab should be handled")
	}
	if fm.showExplorerMenuCount != 1 {
		t.Fatalf("ShowExplorerContextMenu count = %d, want 1", fm.showExplorerMenuCount)
	}
}

func TestMainScreenModifiedTabDoesNotShowExplorerContextMenu(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{CtrlPressed: true})

	if !handled {
		t.Fatal("modified Tab should still be handled to keep focus traversal suppressed")
	}
	if fm.showExplorerMenuCount != 0 {
		t.Fatalf("ShowExplorerContextMenu count = %d, want 0", fm.showExplorerMenuCount)
	}
}

func TestMainScreenDeleteShowsTrashDeleteDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{})

	if !handled {
		t.Fatal("Delete should be handled")
	}
	if fm.showDeleteCount != 1 {
		t.Fatalf("ShowDeleteDialog count = %d, want 1", fm.showDeleteCount)
	}
	if fm.deletePermanent {
		t.Fatal("Delete should request trash, not permanent delete")
	}
}

func TestMainScreenShiftDeleteShowsPermanentDeleteDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+Delete should be handled")
	}
	if fm.showDeleteCount != 1 {
		t.Fatalf("ShowDeleteDialog count = %d, want 1", fm.showDeleteCount)
	}
	if !fm.deletePermanent {
		t.Fatal("Shift+Delete should request permanent delete")
	}
}
