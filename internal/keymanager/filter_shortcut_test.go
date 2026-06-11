package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
)

type fakeFilterSearchDialog struct {
	backspace int
	search    string
	right     int
	left      int
	open      int
	direct    int
	deleted   int
	unpinned  int
}

func (f *fakeFilterSearchDialog) MoveUp()                       {}
func (f *fakeFilterSearchDialog) MoveDown()                     {}
func (f *fakeFilterSearchDialog) MoveToTop()                    {}
func (f *fakeFilterSearchDialog) MoveToBottom()                 {}
func (f *fakeFilterSearchDialog) ClearSearch()                  {}
func (f *fakeFilterSearchDialog) AppendToSearch(char string)    { f.search += char }
func (f *fakeFilterSearchDialog) BackspaceSearch()              { f.backspace++ }
func (f *fakeFilterSearchDialog) GetSearchText() string         { return "" }
func (f *fakeFilterSearchDialog) IsSearchFocused() bool         { return false }
func (f *fakeFilterSearchDialog) FocusList()                    {}
func (f *fakeFilterSearchDialog) SelectCurrentItem()            {}
func (f *fakeFilterSearchDialog) AcceptSelection()              {}
func (f *fakeFilterSearchDialog) AcceptDirectInput()            { f.direct++ }
func (f *fakeFilterSearchDialog) DeleteSelectedEntry()          { f.deleted++ }
func (f *fakeFilterSearchDialog) UnpinSelectedPath()            { f.unpinned++ }
func (f *fakeFilterSearchDialog) AcceptDirectPathNavigation()   { f.direct++ }
func (f *fakeFilterSearchDialog) AcceptDirectPath()             {}
func (f *fakeFilterSearchDialog) OpenDestination()              { f.open++ }
func (f *fakeFilterSearchDialog) CancelDialog()                 {}
func (f *fakeFilterSearchDialog) CopySelectedPathToSearch()     {}
func (f *fakeFilterSearchDialog) CopySelectedShortcutToSearch() {}
func (f *fakeFilterSearchDialog) ScrollSelectedRight()          { f.right++ }
func (f *fakeFilterSearchDialog) ResetHorizontalScroll()        { f.left++ }

func TestFilteringDialogsTreatCtrlHAsBackspace(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*fakeFilterSearchDialog) KeyHandler
	}{
		{name: "filter", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewFilterDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
		{name: "history", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewHistoryDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
		{name: "directory jump", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewDirectoryJumpDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
		{name: "copy move", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewCopyMoveDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialog := &fakeFilterSearchDialog{}
			handler := tt.handler(dialog)

			if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyH}, ModifierState{CtrlPressed: true}) {
				t.Fatal("Ctrl+H activation should be handled")
			}
			if dialog.backspace != 1 {
				t.Fatalf("BackspaceSearch count = %d, want 1", dialog.backspace)
			}
		})
	}
}

func TestCopyMoveDialogCtrlNOpensDestination(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewCopyMoveDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyN}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+N activation should be handled")
	}
	if dialog.open != 1 {
		t.Fatalf("OpenDestination count = %d, want 1", dialog.open)
	}
}

func TestFilterDialogCtrlDDeletesSelectedEntry(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewFilterDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyD}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+D activation should be handled")
	}
	if dialog.deleted != 1 {
		t.Fatalf("DeleteSelectedEntry count = %d, want 1", dialog.deleted)
	}
}

func TestHistoryDialogCtrlDUnpinsSelectedPath(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyD}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+D activation should be handled")
	}
	if dialog.unpinned != 1 {
		t.Fatalf("UnpinSelectedPath count = %d, want 1", dialog.unpinned)
	}
}

func TestFilterDialogCtrlEnterAcceptsDirectInput(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewFilterDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+Enter activation should be handled")
	}
	if dialog.direct != 1 {
		t.Fatalf("AcceptDirectInput count = %d, want 1", dialog.direct)
	}
}

func TestHistoryDialogCtrlEnterAcceptsDirectPathNavigation(t *testing.T) {
	dialog := &fakeFilterSearchDialog{}
	handler := NewHistoryDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+Enter should be handled")
	}
	if dialog.direct != 1 {
		t.Fatalf("AcceptDirectPathNavigation count = %d, want 1", dialog.direct)
	}
}

func TestFilteringDialogsAcceptFirstTypedRune(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*fakeFilterSearchDialog) KeyHandler
	}{
		{name: "directory jump", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewDirectoryJumpDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
		{name: "copy move", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewCopyMoveDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialog := &fakeFilterSearchDialog{}
			handler := tt.handler(dialog)

			if !handler.OnTypedRune('w', ModifierState{}) {
				t.Fatal("first typed rune should be handled")
			}
			if dialog.search != "w" {
				t.Fatalf("search = %q, want %q", dialog.search, "w")
			}
		})
	}
}

func TestFilteringDialogsAcceptSpaceTypedRune(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*fakeFilterSearchDialog) KeyHandler
	}{
		{name: "filter", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewFilterDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
		{name: "history", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewHistoryDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
		{name: "copy move", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewCopyMoveDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialog := &fakeFilterSearchDialog{}
			handler := tt.handler(dialog)

			if !handler.OnTypedRune(' ', ModifierState{}) {
				t.Fatal("space typed rune should be handled")
			}
			if dialog.search != " " {
				t.Fatalf("search = %q, want single space", dialog.search)
			}
		})
	}
}

func TestPathDialogsHandleHorizontalScrollKeys(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*fakeFilterSearchDialog) KeyHandler
	}{
		{name: "history", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewHistoryDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
		{name: "copy move", handler: func(d *fakeFilterSearchDialog) KeyHandler {
			return NewCopyMoveDialogKeyHandler(d, func(string, ...interface{}) {})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialog := &fakeFilterSearchDialog{}
			handler := tt.handler(dialog)

			if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyRight}, ModifierState{}) {
				t.Fatal("Right activation should be handled")
			}
			if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyLeft}, ModifierState{}) {
				t.Fatal("Left activation should be handled")
			}
			if dialog.right != 1 || dialog.left != 1 {
				t.Fatalf("scroll calls right=%d left=%d, want 1/1", dialog.right, dialog.left)
			}
		})
	}
}

type fakeIncrementalSearch struct {
	removed      int
	hidden       int
	accepted     int
	cursorSet    int
	search       string
	currentMatch *fileinfo.FileInfo
}

func (f *fakeIncrementalSearch) ShowIncrementalSearchOverlay()             {}
func (f *fakeIncrementalSearch) HideIncrementalSearchOverlay()             { f.hidden++ }
func (f *fakeIncrementalSearch) AcceptIncrementalSearchOverlay()           { f.accepted++ }
func (f *fakeIncrementalSearch) IsIncrementalSearchVisible() bool          { return true }
func (f *fakeIncrementalSearch) AddSearchCharacter(char rune)              { f.search += string(char) }
func (f *fakeIncrementalSearch) RemoveLastSearchCharacter()                { f.removed++ }
func (f *fakeIncrementalSearch) NextSearchMatch()                          {}
func (f *fakeIncrementalSearch) PreviousSearchMatch()                      {}
func (f *fakeIncrementalSearch) GetCurrentSearchMatch() *fileinfo.FileInfo { return f.currentMatch }
func (f *fakeIncrementalSearch) SetCursorToFile(file *fileinfo.FileInfo)   { f.cursorSet++ }

func TestIncrementalSearchAcceptsSpaceTypedRune(t *testing.T) {
	search := &fakeIncrementalSearch{}
	handler := NewIncrementalSearchKeyHandler(search, func(string, ...interface{}) {})

	if !handler.OnTypedRune(' ', ModifierState{}) {
		t.Fatal("space typed rune should be handled")
	}
	if search.search != " " {
		t.Fatalf("search = %q, want single space", search.search)
	}
}

func TestIncrementalSearchTreatsCtrlHAsBackspace(t *testing.T) {
	search := &fakeIncrementalSearch{}
	handler := NewIncrementalSearchKeyHandler(search, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyH}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+H activation should be handled")
	}
	if search.removed != 1 {
		t.Fatalf("RemoveLastSearchCharacter count = %d, want 1", search.removed)
	}
}

func TestIncrementalSearchAcceptRunsOnNextTick(t *testing.T) {
	km, q := newGatedKeyManager()
	search := &fakeIncrementalSearch{}
	handler := NewIncrementalSearchKeyHandler(search, func(string, ...interface{}) {})
	handler.SetTransitionGate(km.BeginOwnerTransition)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if search.accepted != 0 {
		t.Fatalf("AcceptIncrementalSearchOverlay count = %d before next tick, want 0", search.accepted)
	}

	q.runAll()
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // held Enter repeat

	if search.accepted != 1 {
		t.Fatalf("AcceptIncrementalSearchOverlay count = %d after tick and held repeat, want 1", search.accepted)
	}
}

func TestIncrementalSearchDirectoryAcceptDoesNotLeakReturnToMain(t *testing.T) {
	km, q := newGatedKeyManager()
	search := &fakeIncrementalSearch{
		currentMatch: &fileinfo.FileInfo{Name: "dir", Path: "/tmp/dir", IsDir: true},
	}
	main := &recordingHandler{}
	handler := NewIncrementalSearchKeyHandler(search, func(string, ...interface{}) {})
	handler.SetTransitionGate(km.BeginOwnerTransition)
	km.PushHandler(main)
	searchToken := km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if search.cursorSet != 0 || search.accepted != 0 {
		t.Fatalf("search accepted before next tick: cursorSet=%d accepted=%d", search.cursorSet, search.accepted)
	}
	if len(main.typedKeys) != 0 {
		t.Fatalf("Return leaked to main while transition queued: %v", main.typedKeys)
	}

	q.runAll()
	km.RemoveHandler(searchToken)                           // accept callback removes the handler in production
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) // held Enter repeat

	if search.cursorSet != 1 || search.accepted != 1 || search.hidden != 0 {
		t.Fatalf("search accept after tick = cursorSet=%d accepted=%d hidden=%d, want 1/1/0", search.cursorSet, search.accepted, search.hidden)
	}
	if len(main.typedKeys) != 0 {
		t.Fatalf("late Return leaked to main after search exit: %v", main.typedKeys)
	}
}
