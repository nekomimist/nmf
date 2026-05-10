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
	removed int
}

func (f *fakeIncrementalSearch) ShowIncrementalSearchOverlay()             {}
func (f *fakeIncrementalSearch) HideIncrementalSearchOverlay()             {}
func (f *fakeIncrementalSearch) IsIncrementalSearchVisible() bool          { return true }
func (f *fakeIncrementalSearch) AddSearchCharacter(char rune)              {}
func (f *fakeIncrementalSearch) RemoveLastSearchCharacter()                { f.removed++ }
func (f *fakeIncrementalSearch) NextSearchMatch()                          {}
func (f *fakeIncrementalSearch) PreviousSearchMatch()                      {}
func (f *fakeIncrementalSearch) SelectCurrentSearchMatch()                 {}
func (f *fakeIncrementalSearch) GetCurrentSearchMatch() *fileinfo.FileInfo { return nil }
func (f *fakeIncrementalSearch) OpenFile(file *fileinfo.FileInfo)          {}
func (f *fakeIncrementalSearch) SetCursorToFile(file *fileinfo.FileInfo)   {}

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
