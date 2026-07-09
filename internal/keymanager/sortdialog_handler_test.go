package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeSortDialog struct {
	prevField   int
	nextField   int
	toggled     int
	accepted    int
	cancelled   int
	byName      int
	bySize      int
	byModified  int
	byExt       int
	orderToggle int
	dirsToggle  int
}

func (f *fakeSortDialog) MoveToPreviousField()    { f.prevField++ }
func (f *fakeSortDialog) MoveToNextField()        { f.nextField++ }
func (f *fakeSortDialog) ToggleCurrentField()     { f.toggled++ }
func (f *fakeSortDialog) AcceptSettings()         { f.accepted++ }
func (f *fakeSortDialog) CancelDialog()           { f.cancelled++ }
func (f *fakeSortDialog) SetSortByName()          { f.byName++ }
func (f *fakeSortDialog) SetSortBySize()          { f.bySize++ }
func (f *fakeSortDialog) SetSortByModified()      { f.byModified++ }
func (f *fakeSortDialog) SetSortByExtension()     { f.byExt++ }
func (f *fakeSortDialog) ToggleSortOrder()        { f.orderToggle++ }
func (f *fakeSortDialog) ToggleDirectoriesFirst() { f.dirsToggle++ }

func TestSortDialogHandlerTabNavigation(t *testing.T) {
	dialog := &fakeSortDialog{}
	handler := NewSortDialogHandler(dialog, func(string, ...interface{}) {})

	if handler.GetName() != "SortDialog" {
		t.Fatalf("GetName() = %q, want %q", handler.GetName(), "SortDialog")
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{}) {
		t.Fatal("Tab should be handled")
	}
	if dialog.nextField != 1 {
		t.Fatalf("nextField = %d, want 1", dialog.nextField)
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyTab}, ModifierState{ShiftPressed: true}) {
		t.Fatal("Shift+Tab should be handled")
	}
	if dialog.prevField != 1 {
		t.Fatalf("prevField = %d, want 1", dialog.prevField)
	}
}

func TestSortDialogHandlerAcceptAndCancel(t *testing.T) {
	dialog := &fakeSortDialog{}
	handler := NewSortDialogHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyReturn}, ModifierState{}) {
		t.Fatal("Return should be handled")
	}
	if dialog.accepted != 1 {
		t.Fatalf("accepted = %d, want 1", dialog.accepted)
	}

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, ModifierState{}) {
		t.Fatal("Escape should be handled")
	}
	if dialog.cancelled != 1 {
		t.Fatalf("cancelled = %d, want 1", dialog.cancelled)
	}
}

func TestSortDialogHandlerNumberShortcuts(t *testing.T) {
	dialog := &fakeSortDialog{}
	handler := NewSortDialogHandler(dialog, func(string, ...interface{}) {})

	tests := []struct {
		key  fyne.KeyName
		want func() int
	}{
		{fyne.Key1, func() int { return dialog.byName }},
		{fyne.Key2, func() int { return dialog.bySize }},
		{fyne.Key3, func() int { return dialog.byModified }},
		{fyne.Key4, func() int { return dialog.byExt }},
	}
	for _, tt := range tests {
		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: tt.key}, ModifierState{}) {
			t.Fatalf("%s should be handled", tt.key)
		}
		if tt.want() != 1 {
			t.Fatalf("%s: count = %d, want 1", tt.key, tt.want())
		}
	}
}

func TestSortDialogHandlerRuneShortcuts(t *testing.T) {
	dialog := &fakeSortDialog{}
	handler := NewSortDialogHandler(dialog, func(string, ...interface{}) {})

	for _, r := range []rune{'o', 'O'} {
		if !handler.OnTypedRune(r, ModifierState{}) {
			t.Fatalf("rune %q should be handled", r)
		}
	}
	if dialog.orderToggle != 2 {
		t.Fatalf("orderToggle = %d, want 2", dialog.orderToggle)
	}

	for _, r := range []rune{'d', 'D'} {
		if !handler.OnTypedRune(r, ModifierState{}) {
			t.Fatalf("rune %q should be handled", r)
		}
	}
	if dialog.dirsToggle != 2 {
		t.Fatalf("dirsToggle = %d, want 2", dialog.dirsToggle)
	}

	if handler.OnTypedRune('z', ModifierState{}) {
		t.Fatal("unrelated rune should not be handled")
	}
}
