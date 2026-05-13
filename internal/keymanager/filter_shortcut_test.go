package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
)

type fakeFilterSearchDialog struct {
	backspace int
}

func (f *fakeFilterSearchDialog) MoveUp()                       {}
func (f *fakeFilterSearchDialog) MoveDown()                     {}
func (f *fakeFilterSearchDialog) MoveToTop()                    {}
func (f *fakeFilterSearchDialog) MoveToBottom()                 {}
func (f *fakeFilterSearchDialog) ClearSearch()                  {}
func (f *fakeFilterSearchDialog) AppendToSearch(char string)    {}
func (f *fakeFilterSearchDialog) BackspaceSearch()              { f.backspace++ }
func (f *fakeFilterSearchDialog) GetSearchText() string         { return "" }
func (f *fakeFilterSearchDialog) IsSearchFocused() bool         { return false }
func (f *fakeFilterSearchDialog) FocusList()                    {}
func (f *fakeFilterSearchDialog) SelectCurrentItem()            {}
func (f *fakeFilterSearchDialog) AcceptSelection()              {}
func (f *fakeFilterSearchDialog) AcceptDirectPathNavigation()   {}
func (f *fakeFilterSearchDialog) AcceptDirectPath()             {}
func (f *fakeFilterSearchDialog) CancelDialog()                 {}
func (f *fakeFilterSearchDialog) CopySelectedPathToSearch()     {}
func (f *fakeFilterSearchDialog) CopySelectedShortcutToSearch() {}

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

			if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyH}, ModifierState{CtrlPressed: true}) {
				t.Fatal("Ctrl+H typed key should be handled")
			}
			if dialog.backspace != 1 {
				t.Fatalf("BackspaceSearch count = %d, want 1", dialog.backspace)
			}
		})
	}
}

type fakeIncrementalSearch struct {
	removed      int
	hidden       int
	accepted     int
	cursorSet    int
	currentMatch *fileinfo.FileInfo
}

func (f *fakeIncrementalSearch) ShowIncrementalSearchOverlay()             {}
func (f *fakeIncrementalSearch) HideIncrementalSearchOverlay()             { f.hidden++ }
func (f *fakeIncrementalSearch) AcceptIncrementalSearchOverlay()           { f.accepted++ }
func (f *fakeIncrementalSearch) IsIncrementalSearchVisible() bool          { return true }
func (f *fakeIncrementalSearch) AddSearchCharacter(char rune)              {}
func (f *fakeIncrementalSearch) RemoveLastSearchCharacter()                { f.removed++ }
func (f *fakeIncrementalSearch) NextSearchMatch()                          {}
func (f *fakeIncrementalSearch) PreviousSearchMatch()                      {}
func (f *fakeIncrementalSearch) GetCurrentSearchMatch() *fileinfo.FileInfo { return f.currentMatch }
func (f *fakeIncrementalSearch) SetCursorToFile(file *fileinfo.FileInfo)   { f.cursorSet++ }

func TestIncrementalSearchTreatsCtrlHAsBackspace(t *testing.T) {
	search := &fakeIncrementalSearch{}
	handler := NewIncrementalSearchKeyHandler(search, func(string, ...interface{}) {})

	if !handler.OnTypedKey(&fyne.KeyEvent{Name: fyne.KeyH}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+H typed key should be handled")
	}
	if search.removed != 1 {
		t.Fatalf("RemoveLastSearchCharacter count = %d, want 1", search.removed)
	}
}

func TestIncrementalSearchDefersAcceptUntilKeysReleased(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	search := &fakeIncrementalSearch{}
	handler := NewIncrementalSearchKeyHandler(search, func(string, ...interface{}) {})
	handler.SetTransitionGate(km.DeferUntilKeysReleased)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if search.accepted != 0 {
		t.Fatalf("AcceptIncrementalSearchOverlay count = %d before key release, want 0", search.accepted)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if search.accepted != 1 {
		t.Fatalf("AcceptIncrementalSearchOverlay count = %d after release and late typed key, want 1", search.accepted)
	}
}

func TestIncrementalSearchDirectoryAcceptDoesNotLeakReturnToMain(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	search := &fakeIncrementalSearch{
		currentMatch: &fileinfo.FileInfo{Name: "dir", Path: "/tmp/dir", IsDir: true},
	}
	main := &recordingHandler{}
	handler := NewIncrementalSearchKeyHandler(search, func(string, ...interface{}) {})
	handler.SetTransitionGate(km.DeferUntilKeysReleased)
	km.PushHandler(main)
	km.PushHandler(handler)

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if search.cursorSet != 0 || search.accepted != 0 {
		t.Fatalf("search accepted before key release: cursorSet=%d accepted=%d", search.cursorSet, search.accepted)
	}
	if len(main.typedKeys) != 0 {
		t.Fatalf("Return leaked to main while search pending: %v", main.typedKeys)
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if search.cursorSet != 1 || search.accepted != 1 || search.hidden != 0 {
		t.Fatalf("search accept after release = cursorSet=%d accepted=%d hidden=%d, want 1/1/0", search.cursorSet, search.accepted, search.hidden)
	}
	if len(main.typedKeys) != 0 {
		t.Fatalf("late Return leaked to main after search exit: %v", main.typedKeys)
	}
}
