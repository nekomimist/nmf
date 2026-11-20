package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeCompareDialog struct {
	order          int
	missingOrNewer int
	missing        int
	newer          int
	size           int
	sizeTime       int
	sizeContent    int
	subdirectory   int
}

func (f *fakeCompareDialog) MoveUp()                    {}
func (f *fakeCompareDialog) MoveDown()                  {}
func (f *fakeCompareDialog) MoveToTop()                 {}
func (f *fakeCompareDialog) MoveToBottom()              {}
func (f *fakeCompareDialog) ClearSearch()               {}
func (f *fakeCompareDialog) AppendToSearch(string)      {}
func (f *fakeCompareDialog) BackspaceSearch()           {}
func (f *fakeCompareDialog) CopySelectedPathToSearch()  {}
func (f *fakeCompareDialog) ScrollSelectedRight()       {}
func (f *fakeCompareDialog) ResetHorizontalScroll()     {}
func (f *fakeCompareDialog) ToggleHistoryOrder()        { f.order++ }
func (f *fakeCompareDialog) SelectCurrentItem()         {}
func (f *fakeCompareDialog) AcceptSelection()           {}
func (f *fakeCompareDialog) AcceptDirectPath()          {}
func (f *fakeCompareDialog) CancelDialog()              {}
func (f *fakeCompareDialog) NextMethod()                {}
func (f *fakeCompareDialog) PreviousMethod()            {}
func (f *fakeCompareDialog) SelectMissingOrNewer()      { f.missingOrNewer++ }
func (f *fakeCompareDialog) SelectMissing()             { f.missing++ }
func (f *fakeCompareDialog) SelectNewer()               { f.newer++ }
func (f *fakeCompareDialog) SelectSizeEqual()           { f.size++ }
func (f *fakeCompareDialog) SelectSizeTimeEqual()       { f.sizeTime++ }
func (f *fakeCompareDialog) SelectSizeContentEqual()    { f.sizeContent++ }
func (f *fakeCompareDialog) SelectMissingSubdirectory() { f.subdirectory++ }

func TestCompareDialogHandlerCtrlRTogglesHistoryOrder(t *testing.T) {
	dialog := &fakeCompareDialog{}
	handler := NewCompareDialogKeyHandler(dialog, nil)
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+R should be handled")
	}
	if dialog.order != 1 {
		t.Fatalf("order toggles = %d, want 1", dialog.order)
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyR}, ModifierState{CtrlPressed: true, ShiftPressed: true}) {
		t.Fatal("Ctrl+Shift+R should not match Ctrl+R")
	}
}

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
		{name: "missing subdirectory", key: fyne.KeyD, want: func() int { return dialog.subdirectory }},
	}
	for _, tt := range tests {
		before := tt.want()
		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: tt.key}, modifiers) {
			t.Fatalf("%s shortcut was not handled", tt.name)
		}
		if got := tt.want(); got != before+1 {
			t.Fatalf("%s calls: got %d want %d", tt.name, got, before+1)
		}
	}
}
