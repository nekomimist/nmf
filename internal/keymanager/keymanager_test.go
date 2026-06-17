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

func (noopHandler) OnKeyActivated(_ *fyne.KeyEvent, _ ModifierState) bool { return false }
func (noopHandler) OnTypedRune(_ rune, _ ModifierState) bool              { return false }
func (noopHandler) GetName() string                                       { return "noop" }

type recordingHandler struct {
	typedKeys []fyne.KeyName
	runes     []rune
}

func (h *recordingHandler) OnKeyActivated(ev *fyne.KeyEvent, _ ModifierState) bool {
	h.typedKeys = append(h.typedKeys, ev.Name)
	return true
}
func (h *recordingHandler) OnTypedRune(r rune, _ ModifierState) bool {
	h.runes = append(h.runes, r)
	return true
}
func (h *recordingHandler) GetName() string { return "recording" }

// manualMainQueue stands in for the Fyne main-loop queue so tests can
// control when queued owner transitions run ("the next tick").
type manualMainQueue struct{ fns []func() }

func (q *manualMainQueue) queue(fn func()) { q.fns = append(q.fns, fn) }
func (q *manualMainQueue) runAll() {
	for len(q.fns) > 0 {
		fn := q.fns[0]
		q.fns = q.fns[1:]
		fn()
	}
}

func newGatedKeyManager() (*KeyManager, *manualMainQueue) {
	km := NewKeyManager(func(string, ...interface{}) {})
	q := &manualMainQueue{}
	km.queueOnMain = q.queue
	return km, q
}

type transitionPushOnActivationHandler struct {
	km        *KeyManager
	typedKeys []fyne.KeyName
}

func (h *transitionPushOnActivationHandler) OnKeyActivated(ev *fyne.KeyEvent, _ ModifierState) bool {
	if ev.Name == fyne.KeyL {
		h.km.BeginOwnerTransition("test.push", func() {
			h.km.PushHandler(&recordingHandler{})
		})
		return true
	}
	h.typedKeys = append(h.typedKeys, ev.Name)
	return false
}
func (h *transitionPushOnActivationHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *transitionPushOnActivationHandler) GetName() string {
	return "transitionPushOnActivation"
}

type transitionPopOnActivationHandler struct {
	km    *KeyManager
	token HandlerToken
}

func pushTransitionPopOnActivation(km *KeyManager) *transitionPopOnActivationHandler {
	h := &transitionPopOnActivationHandler{km: km}
	h.token = km.PushHandler(h)
	return h
}

func (h *transitionPopOnActivationHandler) OnKeyActivated(ev *fyne.KeyEvent, _ ModifierState) bool {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter || ev.Name == fyne.KeyEscape {
		h.km.BeginOwnerTransition("test.pop", func() {
			h.km.RemoveHandler(h.token)
		})
		return true
	}
	return false
}
func (h *transitionPopOnActivationHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *transitionPopOnActivationHandler) GetName() string {
	return "transitionPopOnActivation"
}

type transientStackChangeOnActivationHandler struct {
	km        *KeyManager
	typedKeys []fyne.KeyName
}

func (h *transientStackChangeOnActivationHandler) OnKeyActivated(ev *fyne.KeyEvent, _ ModifierState) bool {
	h.typedKeys = append(h.typedKeys, ev.Name)
	token := h.km.PushHandler(noopHandler{})
	h.km.RemoveHandler(token)
	return true
}
func (h *transientStackChangeOnActivationHandler) OnTypedRune(_ rune, _ ModifierState) bool {
	return false
}
func (h *transientStackChangeOnActivationHandler) GetName() string {
	return "transientStackChange"
}

func TestDumpStateIncludesRoutingState(t *testing.T) {
	km, _ := newGatedKeyManager()
	km.PushHandler(noopHandler{})
	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
	km.BeginOwnerTransition("test.transition", func() {})

	dump := km.DumpState()
	for _, want := range []string{
		"handlers=[noop]",
		"modifiers=shift:false ctrl:true alt:false",
		"armed=true",
		"queuedTransitions=1",
		"lastKeyDown=A",
		"stackVersion=1",
	} {
		if !strings.Contains(dump, want) {
			t.Fatalf("DumpState() =\n%s\nwant substring %q", dump, want)
		}
	}
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

func TestKeyManagerActivationRequiresFreshPress(t *testing.T) {
	km, _ := newGatedKeyManager()
	main := &recordingHandler{}
	km.PushHandler(main)

	// Activations without a preceding fresh key down (e.g. repeats surviving
	// from before a focus change) are dropped.
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedRune('a')
	km.HandleShortcut(&fyne.ShortcutSelectAll{})
	if len(main.typedKeys) != 0 || len(main.runes) != 0 {
		t.Fatalf("events delivered while disarmed: keys=%v runes=%q", main.typedKeys, string(main.runes))
	}

	// The arming press itself is fully delivered.
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyReturn {
		t.Fatalf("typed keys = %v, want [Return]", main.typedKeys)
	}
}

func TestKeyManagerOpenTransitionDropsTrailingRuneAndRepeats(t *testing.T) {
	km, q := newGatedKeyManager()
	source := &transitionPushOnActivationHandler{km: km}
	km.PushHandler(source)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyL})  // arms the gate
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL}) // queues the push
	km.HandleTypedRune('l')                            // trailing rune of the same press

	q.runAll() // next main-loop iteration installs the new owner

	pushed, ok := km.GetCurrentHandler().(*recordingHandler)
	if !ok {
		t.Fatalf("current handler = %T, want pushed *recordingHandler", km.GetCurrentHandler())
	}
	if len(pushed.runes) != 0 {
		t.Fatalf("trailing rune leaked into the new owner: %q", string(pushed.runes))
	}

	// Held-key repeats produce no key down and stay gated.
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleTypedRune('l')
	if len(pushed.typedKeys) != 0 || len(pushed.runes) != 0 {
		t.Fatalf("held-key repeat leaked into the new owner: keys=%v runes=%q", pushed.typedKeys, string(pushed.runes))
	}

	// A fresh press is delivered from its key down onwards.
	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyX})
	km.HandleTypedRune('x')
	if len(pushed.runes) != 1 || pushed.runes[0] != 'x' {
		t.Fatalf("next rune = %q, want x", string(pushed.runes))
	}
}

func TestKeyManagerCloseTransitionGatesHeldEnter(t *testing.T) {
	km, q := newGatedKeyManager()
	main := &recordingHandler{}
	km.PushHandler(main)
	pushTransitionPopOnActivation(km)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})  // arms
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // dialog queues its close
	q.runAll()

	if got := km.GetCurrentHandler(); got != main {
		t.Fatalf("current handler = %T, want main handler", got)
	}

	// Enter is still held; its repeats must not fall through to main.
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if len(main.typedKeys) != 0 {
		t.Fatalf("held Enter repeats leaked to main: %v", main.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyReturn {
		t.Fatalf("next typed key = %v, want [Return]", main.typedKeys)
	}
}

func TestKeyManagerDropsFreshPressWhileTransitionQueued(t *testing.T) {
	km, q := newGatedKeyManager()
	source := &transitionPushOnActivationHandler{km: km}
	km.PushHandler(source)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL}) // queues the push
	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyL})

	// A rollover press lands between queue and run: dropped, not queued,
	// and it must not re-arm the gate.
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyF})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyF})
	if len(source.typedKeys) != 0 {
		t.Fatalf("rollover press executed on the old owner: %v", source.typedKeys)
	}

	q.runAll()
	pushed, ok := km.GetCurrentHandler().(*recordingHandler)
	if !ok {
		t.Fatalf("current handler = %T, want pushed *recordingHandler", km.GetCurrentHandler())
	}

	// The dropped press does not surface in the new owner either.
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyF}) // still-held F repeat
	if len(pushed.typedKeys) != 0 {
		t.Fatalf("dropped press leaked into the new owner: %v", pushed.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyF})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyA})
	if len(pushed.typedKeys) != 1 || pushed.typedKeys[0] != fyne.KeyA {
		t.Fatalf("typed keys = %v, want [A]", pushed.typedKeys)
	}
}

func TestKeyManagerResetTransientStateClearsModifiersAndDisarms(t *testing.T) {
	km, _ := newGatedKeyManager()
	main := &recordingHandler{}
	km.PushHandler(main)

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
	km.ResetTransientState("test.reset")

	if km.GetModifierState().CtrlPressed {
		t.Fatal("CtrlPressed should be false after reset")
	}

	// Keys still physically held after the reset (e.g. across a focus loss)
	// stay gated until a fresh press.
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyA})
	if len(main.typedKeys) != 0 {
		t.Fatalf("typed key delivered after reset without fresh press: %v", main.typedKeys)
	}

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyA})
	if len(main.typedKeys) != 1 || main.typedKeys[0] != fyne.KeyA {
		t.Fatalf("typed keys = %v, want [A]", main.typedKeys)
	}
}

func TestKeyManagerRemoveTopClearsModifiers(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	token := km.PushHandler(noopHandler{})

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	if !km.GetModifierState().CtrlPressed {
		t.Fatal("CtrlPressed should be true after Ctrl key down")
	}

	km.RemoveHandler(token)

	if km.GetModifierState().CtrlPressed {
		t.Fatal("CtrlPressed should be false after removing the top handler")
	}
}

func TestKeyManagerRemoveHandlerOutOfOrder(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	bottomToken := km.PushHandler(noopHandler{})
	dialog := &recordingHandler{}
	dialogToken := km.PushHandler(dialog)

	// Removing a non-top entry must not evict the current top handler.
	if got := km.RemoveHandler(bottomToken); got == nil {
		t.Fatal("RemoveHandler(bottomToken) should return the removed handler")
	}
	if got := km.GetCurrentHandler(); got != dialog {
		t.Fatalf("current handler = %T, want dialog handler", got)
	}

	// Removing the same token again is a warning no-op.
	if got := km.RemoveHandler(bottomToken); got != nil {
		t.Fatalf("duplicate remove returned %T, want nil", got)
	}
	if got := km.GetStackSize(); got != 1 {
		t.Fatalf("stack size = %d, want 1", got)
	}

	if got := km.RemoveHandler(dialogToken); got == nil {
		t.Fatal("RemoveHandler(dialogToken) should return the removed handler")
	}
	if got := km.GetStackSize(); got != 0 {
		t.Fatalf("stack size = %d, want 0", got)
	}
}

func TestKeyManagerRemoveNonTopKeepsModifiers(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	bottomToken := km.PushHandler(noopHandler{})
	km.PushHandler(noopHandler{})

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	km.RemoveHandler(bottomToken)

	if !km.GetModifierState().CtrlPressed {
		t.Fatal("CtrlPressed should survive removal of a non-top handler")
	}
}

func TestKeyManagerTransientStackChangeRequiresFreshPress(t *testing.T) {
	km, _ := newGatedKeyManager()
	main := &transientStackChangeOnActivationHandler{km: km}
	km.PushHandler(main)

	// First press: handled; the transient push/remove disarms the gate.
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyTab})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})

	// A held-key repeat stays gated after the stack change.
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if len(main.typedKeys) != 1 {
		t.Fatalf("typed keys = %v, want one Tab before re-press", main.typedKeys)
	}

	// A fresh press is delivered again.
	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyTab})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyTab})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})

	if len(main.typedKeys) != 2 {
		t.Fatalf("typed keys = %v, want two Tab events", main.typedKeys)
	}
}

type mainScreenFakeFileManager struct {
	showJobsCount            int
	showHistoryCount         int
	pinCurrentHistoryCount   int
	showSearchCount          int
	showDirectoryJumpCount   int
	reopenClosedCount        int
	focusWindowLeftCount     int
	focusWindowRightCount    int
	resetWindowSizeCount     int
	resetAllWindowSizesCount int
	showCreateDirCount       int
	createDirName            string
	createDirResult          bool
	showClipboardFileCount   int
	clipboardFileName        string
	clipboardFileResult      bool
	messageTitle             string
	messageText              string
	showMessageCount         int
	clipboardText            string
	clipboardResult          bool
	showRenameCount          int
	showDeleteCount          int
	showExplorerMenuCount    int
	showExternalMenuCount    int
	showViewerCount          int
	showMaintenanceCount     int
	showCompareCount         int
	showSortCount            int
	openFilePath             string
	openDefaultAppPath       string
	deletePermanent          bool
	focusPathCount           int
	currentPath              string
	loadDirectoryPath        string
	saveCursorPath           string
	cursorIndex              int
	setCursorIndex           int
	files                    []fileinfo.FileInfo
	selectedFiles            map[string]bool
	allSelectedFiles         []fileinfo.FileInfo
	refreshFileListCount     int
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
func (f *mainScreenFakeFileManager) ResetWindowSize()                  { f.resetWindowSizeCount++ }
func (f *mainScreenFakeFileManager) ResetAllWindowSizes()              { f.resetAllWindowSizesCount++ }
func (f *mainScreenFakeFileManager) ShowDirectoryTreeDialog()          {}
func (f *mainScreenFakeFileManager) ShowNavigationHistoryDialog()      { f.showHistoryCount++ }
func (f *mainScreenFakeFileManager) PinCurrentHistoryPath()            { f.pinCurrentHistoryCount++ }
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
func (f *mainScreenFakeFileManager) ShowCopyDialog()           {}
func (f *mainScreenFakeFileManager) ShowMoveDialog()           {}
func (f *mainScreenFakeFileManager) ShowExtractArchiveDialog() {}
func (f *mainScreenFakeFileManager) ShowCompareDialog()        { f.showCompareCount++ }
func (f *mainScreenFakeFileManager) ShowRenameDialog()         { f.showRenameCount++ }
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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{ShiftPressed: true})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyO}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyF12}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyJ}, ModifierState{})

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

func TestMainScreenShiftBPinsCurrentHistoryPath(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyB}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+B should be handled")
	}
	if fm.pinCurrentHistoryCount != 1 {
		t.Fatalf("PinCurrentHistoryPath count = %d, want 1", fm.pinCurrentHistoryCount)
	}
}

func TestMainScreenLeftFocusesLeftWindow(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyLeft}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyRight}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyJ}, ModifierState{ShiftPressed: true})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyK}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyP}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyF2}, ModifierState{})

	if !handled {
		t.Fatal("F2 should be handled")
	}
	if fm.showRenameCount != 1 {
		t.Fatalf("ShowRenameDialog count = %d, want 1", fm.showRenameCount)
	}
}

func TestMainScreenRShowsRenameDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{})

	if !handled {
		t.Fatal("R should be handled")
	}
	if fm.showRenameCount != 1 {
		t.Fatalf("ShowRenameDialog count = %d, want 1", fm.showRenameCount)
	}
}

func TestMainScreenModifiedRenameKeysDoNotShowRenameDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyF2}, ModifierState{CtrlPressed: true})
	handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{ShiftPressed: true})

	if fm.showRenameCount != 0 {
		t.Fatalf("ShowRenameDialog count = %d, want 0", fm.showRenameCount)
	}
}

func TestMainScreenTabShowsExplorerContextMenu(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyPeriod}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{CtrlPressed: true})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{ShiftPressed: true})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyV}, ModifierState{})

	if !handled {
		t.Fatal("V should be handled")
	}
	if fm.showViewerCount != 1 {
		t.Fatalf("ShowFileViewer count = %d, want 1", fm.showViewerCount)
	}
}

func TestMainScreenShiftCShowsCompareDialog(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyC}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+C should be handled")
	}
	if fm.showCompareCount != 1 {
		t.Fatalf("ShowCompareDialog count = %d, want 1", fm.showCompareCount)
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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{CtrlPressed: true})

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

func TestMainScreenIInvertsFileMarksOnly(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		files: []fileinfo.FileInfo{
			{Name: "..", Path: "/parent"},
			{Name: "a.txt", Path: "/dir/a.txt"},
			{Name: "b.txt", Path: "/dir/b.txt"},
			{Name: "gone.txt", Path: "/dir/gone.txt", Status: fileinfo.StatusDeleted},
			{Name: "sub", Path: "/dir/sub", IsDir: true},
		},
		selectedFiles: map[string]bool{
			"/dir/a.txt":    true,
			"/dir/gone.txt": true,
			"/dir/sub":      true,
		},
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyI}, ModifierState{})

	if !handled {
		t.Fatal("I should be handled")
	}
	if fm.selectedFiles["/dir/a.txt"] {
		t.Fatalf("selected files = %+v, want a.txt unmarked", fm.selectedFiles)
	}
	if !fm.selectedFiles["/dir/b.txt"] {
		t.Fatalf("selected files = %+v, want b.txt marked", fm.selectedFiles)
	}
	if fm.selectedFiles["/dir/sub"] {
		t.Fatalf("selected files = %+v, want directory unmarked by file-only invert", fm.selectedFiles)
	}
	if !fm.selectedFiles["/dir/gone.txt"] {
		t.Fatalf("selected files = %+v, deleted existing mark should be untouched", fm.selectedFiles)
	}
	if fm.refreshFileListCount != 1 {
		t.Fatalf("RefreshFileList count = %d, want 1", fm.refreshFileListCount)
	}
}

func TestMainScreenShiftIInvertsMarksIncludingDirectories(t *testing.T) {
	fm := &mainScreenFakeFileManager{
		files: []fileinfo.FileInfo{
			{Name: "..", Path: "/parent"},
			{Name: "a.txt", Path: "/dir/a.txt"},
			{Name: "b.txt", Path: "/dir/b.txt"},
			{Name: "gone.txt", Path: "/dir/gone.txt", Status: fileinfo.StatusDeleted},
			{Name: "sub", Path: "/dir/sub", IsDir: true},
		},
		selectedFiles: map[string]bool{
			"/dir/a.txt":    true,
			"/dir/gone.txt": true,
		},
	}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyI}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+I should be handled")
	}
	if fm.selectedFiles["/dir/a.txt"] {
		t.Fatalf("selected files = %+v, want a.txt unmarked", fm.selectedFiles)
	}
	if !fm.selectedFiles["/dir/b.txt"] || !fm.selectedFiles["/dir/sub"] {
		t.Fatalf("selected files = %+v, want b.txt and sub marked", fm.selectedFiles)
	}
	if !fm.selectedFiles["/dir/gone.txt"] {
		t.Fatalf("selected files = %+v, deleted existing mark should be untouched", fm.selectedFiles)
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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

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

func TestMainScreenIgnoresNonMainTargetBindings(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Target: KeyBindingTargetFileViewer, Key: "V", Command: CommandNoop},
	})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyV}, ModifierState{})

	if !handled {
		t.Fatal("default V should still be handled")
	}
	if fm.showViewerCount != 1 {
		t.Fatalf("ShowFileViewer count = %d, want 1", fm.showViewerCount)
	}
}

func TestMainScreenNoopCommandOverridesDefault(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "S-S", Command: CommandNoop},
	})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyS}, ModifierState{ShiftPressed: true})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true})

	if !handled {
		t.Fatal("configured window.reopen should be handled")
	}
	if fm.reopenClosedCount != 1 {
		t.Fatalf("ReopenClosedWindow count = %d, want 1", fm.reopenClosedCount)
	}
}

func TestMainScreenShiftQResetsCurrentWindowSize(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyQ}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("Shift+Q should be handled")
	}
	if fm.resetWindowSizeCount != 1 {
		t.Fatalf("ResetWindowSize count = %d, want 1", fm.resetWindowSizeCount)
	}
	if fm.resetAllWindowSizesCount != 0 {
		t.Fatalf("ResetAllWindowSizes count = %d, want 0", fm.resetAllWindowSizesCount)
	}
}

func TestMainScreenCtrlShiftQResetsAllWindowSizes(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyQ}, ModifierState{CtrlPressed: true, ShiftPressed: true})

	if !handled {
		t.Fatal("Ctrl+Shift+Q should be handled")
	}
	if fm.resetAllWindowSizesCount != 1 {
		t.Fatalf("ResetAllWindowSizes count = %d, want 1", fm.resetAllWindowSizesCount)
	}
	if fm.resetWindowSizeCount != 0 {
		t.Fatalf("ResetWindowSize count = %d, want 0", fm.resetWindowSizeCount)
	}
}

func TestMainScreenNoopCommandOverridesResetSize(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "S-Q", Command: CommandNoop},
	})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyQ}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("configured noop should be handled")
	}
	if fm.resetWindowSizeCount != 0 {
		t.Fatalf("ResetWindowSize count = %d, want 0", fm.resetWindowSizeCount)
	}
}

func TestMainScreenNoopCommandOverridesResetAllSizes(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "C-S-Q", Command: CommandNoop},
	})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyQ}, ModifierState{CtrlPressed: true, ShiftPressed: true})

	if !handled {
		t.Fatal("configured noop should be handled")
	}
	if fm.resetAllWindowSizesCount != 0 {
		t.Fatalf("ResetAllWindowSizes count = %d, want 0", fm.resetAllWindowSizesCount)
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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{ShiftPressed: true})

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

func TestMainScreenTransitionCommandRunsOnNextTick(t *testing.T) {
	km, q := newGatedKeyManager()
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "S-L", Command: CommandHistoryShow},
	})
	handler.SetTransitionGate(km.BeginOwnerTransition)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL})
	km.HandleTypedRune('L')

	if fm.showHistoryCount != 0 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d before next tick, want 0", fm.showHistoryCount)
	}

	q.runAll()
	if fm.showHistoryCount != 1 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d after next tick, want 1", fm.showHistoryCount)
	}

	// Held-key repeats after the transition must not re-trigger the command.
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyL})
	q.runAll()
	if fm.showHistoryCount != 1 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d after held repeat, want 1", fm.showHistoryCount)
	}
}

func TestMainScreenMaintenanceDialogTransitionRunsOnNextTick(t *testing.T) {
	km, q := newGatedKeyManager()
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {}, []config.KeyBindingEntry{
		{Key: "F12", Command: CommandMaintenanceShow},
	})
	handler.SetTransitionGate(km.BeginOwnerTransition)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyF12})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyF12})

	if fm.showMaintenanceCount != 0 {
		t.Fatalf("ShowMaintenanceDialog count = %d before next tick, want 0", fm.showMaintenanceCount)
	}

	q.runAll()
	if fm.showMaintenanceCount != 1 {
		t.Fatalf("ShowMaintenanceDialog count = %d after next tick, want 1", fm.showMaintenanceCount)
	}
}

func TestMainScreenDefersRunCommandTransition(t *testing.T) {
	km, q := newGatedKeyManager()
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
	handler.SetTransitionGate(km.BeginOwnerTransition)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyX})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyX})

	if fm.showHistoryCount != 0 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d before next tick, want 0", fm.showHistoryCount)
	}

	q.runAll()
	if fm.showHistoryCount != 1 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d after next tick, want 1", fm.showHistoryCount)
	}
}

func TestMainScreenProvidesTransitionGateToExtraCommand(t *testing.T) {
	km, q := newGatedKeyManager()
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
	handler.SetTransitionGate(km.BeginOwnerTransition)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyX})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyX})

	if fm.showHistoryCount != 0 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d before next tick, want 0", fm.showHistoryCount)
	}

	q.runAll()
	if fm.showHistoryCount != 1 {
		t.Fatalf("ShowNavigationHistoryDialog count = %d after next tick, want 1", fm.showHistoryCount)
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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

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
	handler.SetTransitionGate(km.BeginOwnerTransition)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyControlLeft})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyA})
	km.HandleShortcut(&fyne.ShortcutSelectAll{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyX}, ModifierState{})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyUp}, ModifierState{ShiftPressed: true})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDown}, ModifierState{ShiftPressed: true})

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

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyName("Bogus")}, ModifierState{})

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

type modsRecordingHandler struct {
	keys []fyne.KeyName
	mods []ModifierState
}

func (h *modsRecordingHandler) OnKeyActivated(ev *fyne.KeyEvent, mods ModifierState) bool {
	h.keys = append(h.keys, ev.Name)
	h.mods = append(h.mods, mods)
	return true
}
func (h *modsRecordingHandler) OnTypedRune(_ rune, _ ModifierState) bool { return false }
func (h *modsRecordingHandler) GetName() string                          { return "modsRecording" }

type fakeUnknownShortcut struct{}

func (fakeUnknownShortcut) ShortcutName() string { return "FakeUnknown" }

func TestHandleShortcutReconstructsFoldedShortcuts(t *testing.T) {
	tests := []struct {
		name        string
		lastKeyDown fyne.KeyName
		shortcut    fyne.Shortcut
		wantKey     fyne.KeyName
		wantMods    ModifierState
	}{
		{name: "copy from C", lastKeyDown: fyne.KeyC, shortcut: &fyne.ShortcutCopy{}, wantKey: fyne.KeyC, wantMods: ModifierState{CtrlPressed: true}},
		{name: "copy from Insert", lastKeyDown: fyne.KeyInsert, shortcut: &fyne.ShortcutCopy{}, wantKey: fyne.KeyInsert, wantMods: ModifierState{CtrlPressed: true}},
		{name: "cut from X", lastKeyDown: fyne.KeyX, shortcut: &fyne.ShortcutCut{}, wantKey: fyne.KeyX, wantMods: ModifierState{CtrlPressed: true}},
		{name: "cut from Shift+Delete", lastKeyDown: fyne.KeyDelete, shortcut: &fyne.ShortcutCut{}, wantKey: fyne.KeyDelete, wantMods: ModifierState{ShiftPressed: true}},
		{name: "paste from V", lastKeyDown: fyne.KeyV, shortcut: &fyne.ShortcutPaste{}, wantKey: fyne.KeyV, wantMods: ModifierState{CtrlPressed: true}},
		{name: "paste from Shift+Insert", lastKeyDown: fyne.KeyInsert, shortcut: &fyne.ShortcutPaste{}, wantKey: fyne.KeyInsert, wantMods: ModifierState{ShiftPressed: true}},
		{name: "select all", lastKeyDown: fyne.KeyA, shortcut: &fyne.ShortcutSelectAll{}, wantKey: fyne.KeyA, wantMods: ModifierState{CtrlPressed: true}},
		{name: "undo", lastKeyDown: fyne.KeyZ, shortcut: &fyne.ShortcutUndo{}, wantKey: fyne.KeyZ, wantMods: ModifierState{CtrlPressed: true}},
		{name: "redo", lastKeyDown: fyne.KeyY, shortcut: &fyne.ShortcutRedo{}, wantKey: fyne.KeyY, wantMods: ModifierState{CtrlPressed: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := NewKeyManager(func(string, ...interface{}) {})
			h := &modsRecordingHandler{}
			km.PushHandler(h)

			km.HandleKeyDown(&fyne.KeyEvent{Name: tt.lastKeyDown})
			km.HandleShortcut(tt.shortcut)

			if len(h.keys) != 1 || h.keys[0] != tt.wantKey {
				t.Fatalf("keys = %v, want [%s]", h.keys, tt.wantKey)
			}
			if h.mods[0] != tt.wantMods {
				t.Fatalf("mods = %+v, want %+v", h.mods[0], tt.wantMods)
			}
		})
	}
}

func TestHandleShortcutPassesCustomShortcutModifiers(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	h := &modsRecordingHandler{}
	km.PushHandler(h)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyQ})
	km.HandleShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyQ,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	})

	if len(h.keys) != 1 || h.keys[0] != fyne.KeyQ {
		t.Fatalf("keys = %v, want [Q]", h.keys)
	}
	want := ModifierState{CtrlPressed: true, ShiftPressed: true}
	if h.mods[0] != want {
		t.Fatalf("mods = %+v, want %+v", h.mods[0], want)
	}
}

func TestHandleShortcutIgnoresUnknownShortcutTypes(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	h := &modsRecordingHandler{}
	km.PushHandler(h)

	km.HandleShortcut(fakeUnknownShortcut{})

	if len(h.keys) != 0 {
		t.Fatalf("keys = %v, want none", h.keys)
	}
}

func TestMainScreenShiftDeleteViaFoldedCutShortcut(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	fm := &mainScreenFakeFileManager{}
	handler := NewMainScreenKeyHandler(fm, func(string, ...interface{}) {})
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyDelete})
	km.HandleShortcut(&fyne.ShortcutCut{})

	if fm.showDeleteCount != 1 {
		t.Fatalf("ShowDeleteDialog count = %d, want 1", fm.showDeleteCount)
	}
	if !fm.deletePermanent {
		t.Fatal("Shift+Delete should request permanent delete")
	}
}

func TestMainScreenDeprecatedEventBindingStillFires(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	var logs []string
	handler := NewMainScreenKeyHandler(fm, func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}, []config.KeyBindingEntry{
		{Key: "S-L", Command: CommandWindowResetSize, Event: "down"},
	})

	handled := handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyL}, ModifierState{ShiftPressed: true})

	if !handled {
		t.Fatal("binding with deprecated event should still fire on activation")
	}
	if fm.resetWindowSizeCount != 1 {
		t.Fatalf("ResetWindowSize count = %d, want 1", fm.resetWindowSizeCount)
	}
	deprecated := false
	for _, log := range logs {
		if strings.Contains(log, "deprecated") {
			deprecated = true
		}
	}
	if !deprecated {
		t.Fatalf("logs = %#v, want deprecation warning", logs)
	}
}
