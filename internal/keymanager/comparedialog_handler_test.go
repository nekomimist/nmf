package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeCompareDialog struct {
	missingOrNewer int
	missing        int
	newer          int
	size           int
	sizeTime       int
	sizeContent    int
}

func (f *fakeCompareDialog) MoveUp()                   {}
func (f *fakeCompareDialog) MoveDown()                 {}
func (f *fakeCompareDialog) MoveToTop()                {}
func (f *fakeCompareDialog) MoveToBottom()             {}
func (f *fakeCompareDialog) ClearSearch()              {}
func (f *fakeCompareDialog) AppendToSearch(string)     {}
func (f *fakeCompareDialog) BackspaceSearch()          {}
func (f *fakeCompareDialog) CopySelectedPathToSearch() {}
func (f *fakeCompareDialog) ScrollSelectedRight()      {}
func (f *fakeCompareDialog) ResetHorizontalScroll()    {}
func (f *fakeCompareDialog) SelectCurrentItem()        {}
func (f *fakeCompareDialog) AcceptSelection()          {}
func (f *fakeCompareDialog) AcceptDirectPath()         {}
func (f *fakeCompareDialog) CancelDialog()             {}
func (f *fakeCompareDialog) NextMethod()               {}
func (f *fakeCompareDialog) PreviousMethod()           {}
func (f *fakeCompareDialog) SelectMissingOrNewer()     { f.missingOrNewer++ }
func (f *fakeCompareDialog) SelectMissing()            { f.missing++ }
func (f *fakeCompareDialog) SelectNewer()              { f.newer++ }
func (f *fakeCompareDialog) SelectSizeEqual()          { f.size++ }
func (f *fakeCompareDialog) SelectSizeTimeEqual()      { f.sizeTime++ }
func (f *fakeCompareDialog) SelectSizeContentEqual()   { f.sizeContent++ }

func TestCompareDialogAltShortcutsSelectMethods(t *testing.T) {
	dialog := &fakeCompareDialog{}
	handler := NewCompareDialogKeyHandler(dialog, func(string, ...interface{}) {})
	modifiers := ModifierState{AltPressed: true}

	tests := []struct {
		name string
		key  fyne.KeyName
		want func() int
	}{
		{name: "missing or newer", key: fyne.KeyU, want: func() int { return dialog.missingOrNewer }},
		{name: "missing", key: fyne.KeyM, want: func() int { return dialog.missing }},
		{name: "newer", key: fyne.KeyN, want: func() int { return dialog.newer }},
		{name: "size", key: fyne.KeyS, want: func() int { return dialog.size }},
		{name: "size time", key: fyne.KeyT, want: func() int { return dialog.sizeTime }},
		{name: "size content", key: fyne.KeyC, want: func() int { return dialog.sizeContent }},
	}
	for _, tt := range tests {
		before := tt.want()
		if !handler.OnKeyDown(&fyne.KeyEvent{Name: tt.key}, modifiers) {
			t.Fatalf("%s shortcut was not handled", tt.name)
		}
		if got := tt.want(); got != before+1 {
			t.Fatalf("%s calls: got %d want %d", tt.name, got, before+1)
		}
	}
}
