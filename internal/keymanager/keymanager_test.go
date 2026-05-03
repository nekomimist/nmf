package keymanager

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"nmf/internal/config"
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
	showExternalMenuCount  int
	deletePermanent        bool
	cursorIndex            int
	setCursorIndex         int
	files                  []fileinfo.FileInfo
}

func (f *mainScreenFakeFileManager) GetCurrentCursorIndex() int                 { return f.cursorIndex }
func (f *mainScreenFakeFileManager) SetCursorByIndex(index int)                 { f.setCursorIndex = index }
func (f *mainScreenFakeFileManager) RefreshCursor()                             {}
func (f *mainScreenFakeFileManager) LoadDirectory(path string)                  {}
func (f *mainScreenFakeFileManager) GetCurrentPath() string                     { return "" }
func (f *mainScreenFakeFileManager) GetFiles() []fileinfo.FileInfo              { return f.files }
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
func (f *mainScreenFakeFileManager) ShowExternalCommandMenu() { f.showExternalMenuCount++ }

func TestMainScreenJShowsDirectoryJumpDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyJ}, ModifierState{})

	if !handled {
		t.Fatal("J should be handled")
	}
	if fm.showDirectoryJumpCount != 1 {
		t.Fatalf("ShowDirectoryJumpDialog count = %d, want 1", fm.showDirectoryJumpCount)
	}
	if fm.showJobsCount != 0 {
		t.Fatalf("ShowJobsDialog count = %d, want 0", fm.showJobsCount)
	}
}

func TestMainScreenShiftJShowsJobsDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyJ}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+J should be handled")
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

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{})

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

func TestMainScreenXShowsExternalCommandMenu(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

	if !handled {
		t.Fatal("X should be handled")
	}
	if fm.showExternalMenuCount != 1 {
		t.Fatalf("ShowExternalCommandMenu count = %d, want 1", fm.showExternalMenuCount)
	}
}

func TestMainScreenConfiguredBindingOverridesDefault(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "X", Command: CommandJobsShow},
	})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

	if !handled {
		t.Fatal("configured X should be handled")
	}
	if fm.showJobsCount != 1 {
		t.Fatalf("ShowJobsDialog count = %d, want 1", fm.showJobsCount)
	}
	if fm.showExternalMenuCount != 0 {
		t.Fatalf("ShowExternalCommandMenu count = %d, want 0", fm.showExternalMenuCount)
	}
}

func TestMainScreenShiftUpUsesPageUpCommand(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		cursorIndex: 30,
		files:       make([]fileinfo.FileInfo, 40),
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+Up should be handled")
	}
	if fm.setCursorIndex != 10 {
		t.Fatalf("SetCursorByIndex = %d, want 10", fm.setCursorIndex)
	}
}

func TestMainScreenShiftDownUsesPageDownCommand(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		cursorIndex: 5,
		files:       make([]fileinfo.FileInfo, 30),
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyDown}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+Down should be handled")
	}
	if fm.setCursorIndex != 25 {
		t.Fatalf("SetCursorByIndex = %d, want 25", fm.setCursorIndex)
	}
}

func TestParseKeySpecRejectsCaretSyntax(t *testing.T) {
	if _, err := parseKeySpec("^N"); err == nil {
		t.Fatal("parseKeySpec should reject caret syntax")
	}
}

func TestParseKeySpecRejectsLongCtrlSyntax(t *testing.T) {
	if _, err := parseKeySpec("Ctrl-N"); err == nil {
		t.Fatal("parseKeySpec should reject long Ctrl syntax")
	}
}

func TestParseKeySpecAcceptsMultipleModifiers(t *testing.T) {
	spec, err := parseKeySpec("S-A-C-F2")
	if err != nil {
		t.Fatalf("parseKeySpec returned error: %v", err)
	}
	if spec.key != fyne.KeyF2 {
		t.Fatalf("key = %q, want %q", spec.key, fyne.KeyF2)
	}
	if !spec.mod.ShiftPressed || !spec.mod.AltPressed || !spec.mod.CtrlPressed {
		t.Fatalf("modifiers = %+v, want shift/alt/ctrl", spec.mod)
	}
}

func TestParseKeySpecRejectsUnknownKeyName(t *testing.T) {
	if _, err := parseKeySpec("C-NotAKey"); err == nil {
		t.Fatal("parseKeySpec should reject unknown key names")
	}
}

func TestParseKeySpecRejectsUnknownModifierName(t *testing.T) {
	if _, err := parseKeySpec("M-A"); err == nil {
		t.Fatal("parseKeySpec should reject modifiers outside S/A/C")
	}
}

func TestParseKeySpecAcceptsFyneKeyNameValues(t *testing.T) {
	tests := []struct {
		input string
		want  fyne.KeyName
	}{
		{input: "BackSpace", want: fyne.KeyBackspace},
		{input: "Prior", want: fyne.KeyPageUp},
		{input: "Next", want: fyne.KeyPageDown},
		{input: "KP_Enter", want: fyne.KeyEnter},
		{input: "C--", want: fyne.KeyMinus},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			spec, err := parseKeySpec(tt.input)
			if err != nil {
				t.Fatalf("parseKeySpec returned error: %v", err)
			}
			if spec.key != tt.want {
				t.Fatalf("key = %q, want %q", spec.key, tt.want)
			}
		})
	}
}

func TestInvalidConfiguredBindingIsIgnored(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	var logs []string
	handler := NewMainScreenKeyHandler(fm, func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}, []config.KeyBindingEntry{
		{Key: "Bogus", Command: CommandJobsShow},
	})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyName("Bogus")}, ModifierState{})

	if handled {
		t.Fatal("invalid configured binding should be ignored")
	}
	if fm.showJobsCount != 0 {
		t.Fatalf("ShowJobsDialog count = %d, want 0", fm.showJobsCount)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "WARNING invalid key binding") {
		t.Fatalf("logs = %#v, want warning", logs)
	}
}
