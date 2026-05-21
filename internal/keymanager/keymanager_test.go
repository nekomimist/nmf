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
	runes     []rune
}

func (h *recordingHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool { return false }
func (h *recordingHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool   { return false }
func (h *recordingHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	h.typedKeys = append(h.typedKeys, ev.Name)
	return true
}
func (h *recordingHandler) OnTypedRune(r rune, _ ModifierState) bool {
	h.runes = append(h.runes, r)
	return true
}
func (h *recordingHandler) GetName() string { return "recording" }

type deferPopOnKeyDownHandler struct {
	km *KeyManager
}

func (h *deferPopOnKeyDownHandler) OnKeyDown(ev *fyne.KeyEvent, _ ModifierState) bool {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
		h.km.DeferUntilKeysReleased("test.pop", func() {
			h.km.PopHandler()
		})
		return true
	}
	return false
}
func (h *deferPopOnKeyDownHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *deferPopOnKeyDownHandler) OnTypedKey(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *deferPopOnKeyDownHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *deferPopOnKeyDownHandler) GetName() string                          { return "deferPopOnKeyDown" }

type deferPushOnTypedKeyHandler struct {
	km *KeyManager
}

func (h *deferPushOnTypedKeyHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *deferPushOnTypedKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *deferPushOnTypedKeyHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	if ev.Name == fyne.KeyL {
		h.km.DeferUntilKeysReleased("test.push", func() {
			h.km.PushHandler(&recordingHandler{})
		})
		return true
	}
	return false
}
func (h *deferPushOnTypedKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *deferPushOnTypedKeyHandler) GetName() string                          { return "deferPushOnTypedKey" }

type deferPopOnTypedKeyHandler struct {
	km *KeyManager
}

func (h *deferPopOnTypedKeyHandler) OnKeyDown(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *deferPopOnTypedKeyHandler) OnKeyUp(_ *fyne.KeyEvent, _ ModifierState) bool {
	return false
}
func (h *deferPopOnTypedKeyHandler) OnTypedKey(ev *fyne.KeyEvent, _ ModifierState) bool {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEscape {
		h.km.DeferUntilKeysReleased("test.typedPop", func() {
			h.km.PopHandler()
		})
		return true
	}
	return false
}
func (h *deferPopOnTypedKeyHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *deferPopOnTypedKeyHandler) GetName() string                          { return "deferPopOnTypedKey" }

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

func TestKeyManagerDefersTransitionUntilKeysReleased(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	main := &recordingHandler{}
	km.PushHandler(main)
	km.PushHandler(&deferPopOnKeyDownHandler{km: km})

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyEnter})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 0 {
		t.Fatalf("typed keys leaked to main while transition was pending: %v", main.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if got := km.GetCurrentHandler(); got != main {
		t.Fatalf("current handler = %T, want main handler", got)
	}
}

func TestKeyManagerSuppressesLateTypedKeyAfterDeferredTransition(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	main := &recordingHandler{}
	km.PushHandler(main)
	km.PushHandler(&deferPopOnKeyDownHandler{km: km})

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyEnter})
	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 0 {
		t.Fatalf("typed keys leaked to main after Enter key up: %v", main.typedKeys)
	}

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyReturn {
		t.Fatalf("next typed key = %v, want [Return]", main.typedKeys)
	}
}

func TestKeyManagerGatesRepeatedTypedKeyDuringDeferredTransition(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	main := &recordingHandler{}
	km.PushHandler(main)
	km.PushHandler(&deferPopOnKeyDownHandler{km: km})

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 0 {
		t.Fatalf("repeated typed key leaked to main while Enter was held: %v", main.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyReturn {
		t.Fatalf("typed keys after transition = %v, want [Return]", main.typedKeys)
	}
}

func TestKeyManagerDefersTypedKeyPopUntilKeyRelease(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	main := &recordingHandler{}
	dialog := &deferPopOnTypedKeyHandler{km: km}
	km.PushHandler(main)
	km.PushHandler(dialog)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if got := km.GetCurrentHandler(); got != dialog {
		t.Fatalf("current handler before key up = %T, want dialog handler", got)
	}
	if len(main.typedKeys) != 0 {
		t.Fatalf("typed keys leaked to main while dialog close was pending: %v", main.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if got := km.GetCurrentHandler(); got != main {
		t.Fatalf("current handler after key up = %T, want main handler", got)
	}
	if len(main.typedKeys) != 0 {
		t.Fatalf("late typed key leaked to main after dialog close: %v", main.typedKeys)
	}

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyReturn {
		t.Fatalf("next typed key = %v, want [Return]", main.typedKeys)
	}
}

func TestKeyManagerGatesTypedRuneBeforeDeferredTransition(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	km.PushHandler(&deferPushOnTypedKeyHandler{km: km})

	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleTypedRune('L')

	current := km.GetCurrentHandler()
	if _, ok := current.(*deferPushOnTypedKeyHandler); !ok {
		t.Fatalf("current handler = %T, want deferred source handler", current)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyL})
	current = km.GetCurrentHandler()
	main, ok := current.(*recordingHandler)
	if !ok {
		t.Fatalf("current handler = %T, want *recordingHandler", current)
	}

	if len(main.runes) != 0 {
		t.Fatalf("typed rune leaked to pushed handler: %q", string(main.runes))
	}

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyX})
	km.HandleTypedRune('x')

	if len(main.runes) != 1 || main.runes[0] != 'x' {
		t.Fatalf("next rune = %q, want x", string(main.runes))
	}
}

func TestKeyManagerSuppressesLateTypedRuneAfterDeferredTransition(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	km.PushHandler(&deferPushOnTypedKeyHandler{km: km})

	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleTypedRune('L')

	current := km.GetCurrentHandler()
	main, ok := current.(*recordingHandler)
	if !ok {
		t.Fatalf("current handler = %T, want *recordingHandler", current)
	}

	if len(main.runes) != 0 {
		t.Fatalf("late typed rune leaked to pushed handler: %q", string(main.runes))
	}

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyX})
	km.HandleTypedRune('x')

	if len(main.runes) != 1 || main.runes[0] != 'x' {
		t.Fatalf("runes after suppress clear = %q, want x", string(main.runes))
	}
}

func TestKeyManagerPopClearsModifiers(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	km.PushHandler(noopHandler{})

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	if !km.GetModifierState().CtrlPressed {
		t.Fatal("CtrlPressed should be true after Ctrl key down")
	}

	km.PopHandler()

	if km.GetModifierState().CtrlPressed {
		t.Fatal("CtrlPressed should be false after PopHandler")
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
	showHistoryCount       int
	showSearchCount        int
	showDirectoryJumpCount int
	reopenClosedCount      int
	focusWindowLeftCount   int
	focusWindowRightCount  int
	showCreateDirCount     int
	createDirName          string
	createDirResult        bool
	showClipboardFileCount int
	clipboardFileName      string
	clipboardFileResult    bool
	messageTitle           string
	messageText            string
	showMessageCount       int
	clipboardText          string
	clipboardResult        bool
	showRenameCount        int
	showDeleteCount        int
	showExplorerMenuCount  int
	showExternalMenuCount  int
	showViewerCount        int
	showMaintenanceCount   int
	showSortCount          int
	openFilePath           string
	openDefaultAppPath     string
	deletePermanent        bool
	focusPathCount         int
	currentPath            string
	loadDirectoryPath      string
	saveCursorPath         string
	cursorIndex            int
	setCursorIndex         int
	files                  []fileinfo.FileInfo
	selectedFiles          map[string]bool
	allSelectedFiles       []fileinfo.FileInfo
	refreshFileListCount   int
}

func (f *mainScreenFakeFileManager) GetCurrentCursorIndex() int    { return f.cursorIndex }
func (f *mainScreenFakeFileManager) SetCursorByIndex(index int)    { f.setCursorIndex = index }
func (f *mainScreenFakeFileManager) RefreshCursor()                {}
func (f *mainScreenFakeFileManager) LoadDirectory(path string)     { f.loadDirectoryPath = path }
func (f *mainScreenFakeFileManager) GetCurrentPath() string        { return f.currentPath }
func (f *mainScreenFakeFileManager) GetFiles() []fileinfo.FileInfo { return f.files }
func (f *mainScreenFakeFileManager) CurrentSort() config.SortConfig {
	return config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
}
func (f *mainScreenFakeFileManager) ApplyTemporarySort(sortConfig config.SortConfig) {
}
func (f *mainScreenFakeFileManager) GetSelectedFiles() map[string]bool { return f.selectedFiles }
func (f *mainScreenFakeFileManager) GetAllSelectedFiles() []fileinfo.FileInfo {
	if f.allSelectedFiles != nil {
		return f.allSelectedFiles
	}
	files := f.GetFiles()
	selected := f.GetSelectedFiles()
	targets := make([]fileinfo.FileInfo, 0, len(selected))
	for _, fi := range files {
		if selected[fi.Path] {
			targets = append(targets, fi)
		}
	}
	return targets
}
func (f *mainScreenFakeFileManager) SetFileSelected(path string, selected bool) {
	if f.selectedFiles == nil {
		f.selectedFiles = make(map[string]bool)
	}
	f.selectedFiles[path] = selected
}
func (f *mainScreenFakeFileManager) RefreshFileList()                  { f.refreshFileListCount++ }
func (f *mainScreenFakeFileManager) SaveCursorPosition(dirPath string) { f.saveCursorPath = dirPath }
func (f *mainScreenFakeFileManager) OpenNewWindow()                    {}
func (f *mainScreenFakeFileManager) ReopenClosedWindow()               { f.reopenClosedCount++ }
func (f *mainScreenFakeFileManager) FocusWindowLeft()                  { f.focusWindowLeftCount++ }
func (f *mainScreenFakeFileManager) FocusWindowRight()                 { f.focusWindowRightCount++ }
func (f *mainScreenFakeFileManager) ShowDirectoryTreeDialog()          {}
func (f *mainScreenFakeFileManager) ShowNavigationHistoryDialog()      { f.showHistoryCount++ }
func (f *mainScreenFakeFileManager) ShowDirectoryJumpDialog() {
	f.showDirectoryJumpCount++
}
func (f *mainScreenFakeFileManager) ShowFilterDialog()            {}
func (f *mainScreenFakeFileManager) ClearFilter()                 {}
func (f *mainScreenFakeFileManager) ToggleFilter()                {}
func (f *mainScreenFakeFileManager) ShowIncrementalSearchDialog() { f.showSearchCount++ }
func (f *mainScreenFakeFileManager) ShowSortDialog()              { f.showSortCount++ }
func (f *mainScreenFakeFileManager) ShowJobsDialog()              { f.showJobsCount++ }
func (f *mainScreenFakeFileManager) ShowPathEditDialog()          { f.focusPathCount++ }
func (f *mainScreenFakeFileManager) ShowCreateDirectoryDialog()   { f.showCreateDirCount++ }
func (f *mainScreenFakeFileManager) CreateDirectory(name string) bool {
	f.createDirName = name
	return f.createDirResult
}
func (f *mainScreenFakeFileManager) ShowClipboardTextFileDialog() { f.showClipboardFileCount++ }
func (f *mainScreenFakeFileManager) CreateClipboardTextFile(name string) bool {
	f.clipboardFileName = name
	return f.clipboardFileResult
}
func (f *mainScreenFakeFileManager) ShowMessageDialog(title string, message string) {
	f.messageTitle = title
	f.messageText = message
	f.showMessageCount++
}
func (f *mainScreenFakeFileManager) SetClipboardText(text string) bool {
	f.clipboardText = text
	return f.clipboardResult
}
func (f *mainScreenFakeFileManager) QuitApplication() {}
func (f *mainScreenFakeFileManager) OpenFile(file *fileinfo.FileInfo) {
	if file != nil {
		f.openFilePath = file.Path
	}
}
func (f *mainScreenFakeFileManager) OpenFileDefaultApp(file *fileinfo.FileInfo) {
	if file != nil {
		f.openDefaultAppPath = file.Path
	}
}
func (f *mainScreenFakeFileManager) ShowCopyDialog()   {}
func (f *mainScreenFakeFileManager) ShowMoveDialog()   {}
func (f *mainScreenFakeFileManager) ShowRenameDialog() { f.showRenameCount++ }
func (f *mainScreenFakeFileManager) ShowDeleteDialog(permanent bool) {
	f.showDeleteCount++
	f.deletePermanent = permanent
}
func (f *mainScreenFakeFileManager) ShowExplorerContextMenu() { f.showExplorerMenuCount++ }
func (f *mainScreenFakeFileManager) ShowExternalCommandMenu() { f.showExternalMenuCount++ }
func (f *mainScreenFakeFileManager) ShowFileViewer()          { f.showViewerCount++ }
func (f *mainScreenFakeFileManager) ShowMaintenanceDialog()   { f.showMaintenanceCount++ }
func (f *mainScreenFakeFileManager) ShowCommandMenu(title string, items []CommandMenuItem) {
}

func TestMainScreenReturnRunsOpen(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		files: []fileinfo.FileInfo{{Name: "book.xlsx", Path: "/dir/book.xlsx"}},
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{})

	if !handled {
		t.Fatal("Return should be handled")
	}
	if fm.openFilePath != "/dir/book.xlsx" {
		t.Fatalf("OpenFile path = %q, want /dir/book.xlsx", fm.openFilePath)
	}
	if fm.openDefaultAppPath != "" {
		t.Fatalf("OpenFileDefaultApp path = %q, want empty", fm.openDefaultAppPath)
	}
}

func TestMainScreenShiftReturnRunsOpenDefaultApp(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		files: []fileinfo.FileInfo{{Name: "book.xlsx", Path: "/dir/book.xlsx"}},
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+Return should be handled")
	}
	if fm.openDefaultAppPath != "/dir/book.xlsx" {
		t.Fatalf("OpenFileDefaultApp path = %q, want /dir/book.xlsx", fm.openDefaultAppPath)
	}
	if fm.openFilePath != "" {
		t.Fatalf("OpenFile path = %q, want empty", fm.openFilePath)
	}
}

func TestMainScreenConfiguredBindingCanRunOpenDefaultApp(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		files: []fileinfo.FileInfo{{Name: "book.xlsx", Path: "/dir/book.xlsx"}},
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "O", Command: CommandOpenDefaultApp},
	})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyO}, ModifierState{})

	if !handled {
		t.Fatal("configured open.defaultApp should be handled")
	}
	if fm.openDefaultAppPath != "/dir/book.xlsx" {
		t.Fatalf("OpenFileDefaultApp path = %q, want /dir/book.xlsx", fm.openDefaultAppPath)
	}
}

func TestMainScreenConfiguredBindingCanShowMaintenanceDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "F12", Command: CommandMaintenanceShow},
	})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyF12}, ModifierState{})

	if !handled {
		t.Fatal("configured maintenance.show should be handled")
	}
	if fm.showMaintenanceCount != 1 {
		t.Fatalf("ShowMaintenanceDialog count = %d, want 1", fm.showMaintenanceCount)
	}
}

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

func TestMainScreenLeftFocusesLeftWindow(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyLeft}, ModifierState{})

	if !handled {
		t.Fatal("Left should be handled")
	}
	if fm.focusWindowLeftCount != 1 {
		t.Fatalf("FocusWindowLeft count = %d, want 1", fm.focusWindowLeftCount)
	}
	if fm.focusWindowRightCount != 0 {
		t.Fatalf("FocusWindowRight count = %d, want 0", fm.focusWindowRightCount)
	}
}

func TestMainScreenRightFocusesRightWindow(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyRight}, ModifierState{})

	if !handled {
		t.Fatal("Right should be handled")
	}
	if fm.focusWindowRightCount != 1 {
		t.Fatalf("FocusWindowRight count = %d, want 1", fm.focusWindowRightCount)
	}
	if fm.focusWindowLeftCount != 0 {
		t.Fatalf("FocusWindowLeft count = %d, want 0", fm.focusWindowLeftCount)
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

func TestMainScreenKShowsCreateDirectoryDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyK}, ModifierState{})

	if !handled {
		t.Fatal("K should be handled")
	}
	if fm.showCreateDirCount != 1 {
		t.Fatalf("ShowCreateDirectoryDialog count = %d, want 1", fm.showCreateDirCount)
	}
}

func TestMainScreenPShowsClipboardTextFileDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyP}, ModifierState{})

	if !handled {
		t.Fatal("P should be handled")
	}
	if fm.showClipboardFileCount != 1 {
		t.Fatalf("ShowClipboardTextFileDialog count = %d, want 1", fm.showClipboardFileCount)
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

func TestMainScreenPeriodRefreshesCurrentDirectory(t *testing.T) {
	fm := &mainScreenFakeFileManager{currentPath: "/tmp/nmf"}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyPeriod}, ModifierState{})

	if !handled {
		t.Fatal("Period should be handled")
	}
	if fm.saveCursorPath != "/tmp/nmf" {
		t.Fatalf("SaveCursorPosition path = %q, want /tmp/nmf", fm.saveCursorPath)
	}
	if fm.loadDirectoryPath != "/tmp/nmf" {
		t.Fatalf("LoadDirectory path = %q, want /tmp/nmf", fm.loadDirectoryPath)
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

func TestMainScreenVShowsFileViewer(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyV}, ModifierState{})

	if !handled {
		t.Fatal("V should be handled")
	}
	if fm.showViewerCount != 1 {
		t.Fatalf("ShowFileViewer count = %d, want 1", fm.showViewerCount)
	}
}

func TestMainScreenCtrlAMarksAllSelectableFiles(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		files: []fileinfo.FileInfo{
			{Name: "..", Path: "/parent"},
			{Name: "a.txt", Path: "/dir/a.txt"},
			{Name: "gone.txt", Path: "/dir/gone.txt", Status: fileinfo.StatusDeleted},
			{Name: "sub", Path: "/dir/sub", IsDir: true},
		},
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{CtrlPressed: true})

	if !handled {
		t.Fatal("Ctrl+A should be handled")
	}
	if got := fm.selectedFiles; !got["/dir/a.txt"] || !got["/dir/sub"] {
		t.Fatalf("selected files = %+v, want selectable files marked", got)
	}
	if fm.selectedFiles["/parent"] || fm.selectedFiles["/dir/gone.txt"] {
		t.Fatalf("selected files = %+v, should skip parent and deleted entries", fm.selectedFiles)
	}
	if fm.refreshFileListCount != 1 {
		t.Fatalf("RefreshFileList count = %d, want 1", fm.refreshFileListCount)
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

func TestMainScreenNoopCommandOverridesDefault(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "S-S", Command: CommandNoop},
	})

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyS}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("configured noop should be handled")
	}
	if fm.showSortCount != 0 {
		t.Fatalf("ShowSortDialog count = %d, want 0", fm.showSortCount)
	}
}

func TestMainScreenConfiguredBindingCanReopenWindow(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "C-R", Command: CommandWindowReopen},
	})

	handled := handler.OnKeyDown(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true})

	if !handled {
		t.Fatal("configured window.reopen should be handled")
	}
	if fm.reopenClosedCount != 1 {
		t.Fatalf("ReopenClosedWindow count = %d, want 1", fm.reopenClosedCount)
	}
}

func TestMainScreenConfiguredBindingCanUseExtraCommand(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	var got CommandContext
	handler := NewMainScreenKeyHandlerWithCommands(
		fm,
		func(string, ...interface{}) {},
		[]config.KeyBindingEntry{{Key: "S-X", Command: "user.test"}},
		CommandRegistry{
			"user.test": func(ctx CommandContext) {
				got = ctx
			},
		},
	)

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("configured extra command should be handled")
	}
	if got.Key != fyne.KeyX || got.Event != keyEventTyped || !got.Modifiers.ShiftPressed {
		t.Fatalf("context = %+v, want key=X event=typed shift=true", got)
	}
	if got.FileManager != fm {
		t.Fatal("context should carry file manager")
	}
}

func TestMainScreenDefersInputTransitionUntilKeysReleased(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "S-L", Command: CommandHistoryShow, Event: keyEventTyped},
	})
	handler.SetTransitionGate(km.DeferUntilKeysReleased)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleTypedRune('L')

	if fm.showHistoryCount != 0 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d before key release, want 0", fm.showHistoryCount)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyL})
	if fm.showHistoryCount != 0 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d before all keys release, want 0", fm.showHistoryCount)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	if fm.showHistoryCount != 1 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d after release, want 1", fm.showHistoryCount)
	}
}

func TestMainScreenDefersMaintenanceDialogTransition(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "F12", Command: CommandMaintenanceShow},
	})
	handler.SetTransitionGate(km.DeferUntilKeysReleased)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyF12})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyF12})

	if fm.showMaintenanceCount != 0 {
		t.Fatalf("ShowMaintenanceDialog count = %d before key release, want 0", fm.showMaintenanceCount)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyF12})
	if fm.showMaintenanceCount != 1 {
		t.Fatalf("ShowMaintenanceDialog count = %d after release, want 1", fm.showMaintenanceCount)
	}
}

func TestMainScreenDefersRunCommandTransition(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandlerWithCommands(
		fm,
		func(string, ...interface{}) {},
		[]config.KeyBindingEntry{{Key: "X", Command: "user.history"}},
		CommandRegistry{
			"user.history": func(ctx CommandContext) {
				ctx.RunCommand(CommandHistoryShow)
			},
		},
	)
	handler.SetTransitionGate(km.DeferUntilKeysReleased)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyX})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyX})

	if fm.showHistoryCount != 0 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d before key release, want 0", fm.showHistoryCount)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyX})
	if fm.showHistoryCount != 1 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d after release, want 1", fm.showHistoryCount)
	}
}

func TestMainScreenProvidesTransitionGateToExtraCommand(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandlerWithCommands(
		fm,
		func(string, ...interface{}) {},
		[]config.KeyBindingEntry{{Key: "X", Command: "user.dialog"}},
		CommandRegistry{
			"user.dialog": func(ctx CommandContext) {
				ctx.DeferTransition("user.dialog", func() {
					ctx.FileManager.ShowNavigationHistoryDialog()
				})
			},
		},
	)
	handler.SetTransitionGate(km.DeferUntilKeysReleased)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyX})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyX})

	if fm.showHistoryCount != 0 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d before key release, want 0", fm.showHistoryCount)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyX})
	if fm.showHistoryCount != 1 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d after release, want 1", fm.showHistoryCount)
	}
}

func TestMainScreenProvidesClipboardWriterToExtraCommand(t *testing.T) {
	fm := &mainScreenFakeFileManager{clipboardResult: true}
	handler := NewMainScreenKeyHandlerWithCommands(
		fm,
		func(string, ...interface{}) {},
		[]config.KeyBindingEntry{{Key: "X", Command: "user.copy"}},
		CommandRegistry{
			"user.copy": func(ctx CommandContext) {
				if ctx.SetClipboard != nil {
					ctx.SetClipboard("hello")
				}
			},
		},
	)

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

	if !handled {
		t.Fatal("configured extra command should be handled")
	}
	if fm.clipboardText != "hello" {
		t.Fatalf("clipboard text = %q, want hello", fm.clipboardText)
	}
}

func TestMainScreenDoesNotDeferNonTransitionCommand(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	fm := &mainScreenFakeFileManager{
		files: []fileinfo.FileInfo{
			{Name: "a.txt", Path: "/dir/a.txt"},
		},
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})
	handler.SetTransitionGate(km.DeferUntilKeysReleased)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyA})

	if fm.refreshFileListCount != 1 {
		t.Fatalf("RefreshFileList count = %d, want immediate select-all", fm.refreshFileListCount)
	}
}

func TestMainScreenExtraCommandCanRunInternalCommand(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandlerWithCommands(
		fm,
		func(string, ...interface{}) {},
		[]config.KeyBindingEntry{{Key: "X", Command: "user.jobs"}},
		CommandRegistry{
			"user.jobs": func(ctx CommandContext) {
				ctx.RunCommand(CommandJobsShow)
			},
		},
	)

	handled := handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

	if !handled {
		t.Fatal("configured extra command should be handled")
	}
	if fm.showJobsCount != 1 {
		t.Fatalf("ShowJobsDialog count = %d, want 1", fm.showJobsCount)
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
