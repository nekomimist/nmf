package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeFileViewer struct {
	closed int
	down   int
	up     int
	pgDown int
	pgUp   int
	home   int
	end    int
	next   int
	prev   int
	search int
	line   int
}

func (f *fakeFileViewer) CloseViewer()          { f.closed++ }
func (f *fakeFileViewer) ViewerLineDown()       { f.down++ }
func (f *fakeFileViewer) ViewerLineUp()         { f.up++ }
func (f *fakeFileViewer) ViewerPageDown()       { f.pgDown++ }
func (f *fakeFileViewer) ViewerPageUp()         { f.pgUp++ }
func (f *fakeFileViewer) ViewerHome()           { f.home++ }
func (f *fakeFileViewer) ViewerEnd()            { f.end++ }
func (f *fakeFileViewer) ViewerSearchNext()     { f.next++ }
func (f *fakeFileViewer) ViewerSearchPrevious() { f.prev++ }
func (f *fakeFileViewer) ViewerFocusSearch()    { f.search++ }
func (f *fakeFileViewer) ViewerFocusLine()      { f.line++ }

func TestFileViewerHandlerLessKeys(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer)

	for _, r := range []rune{'j', 'k', 'f', 'b', 'g', 'G', 'n', 'N', '/', ':', 'q'} {
		if !handler.OnTypedRune(r, ModifierState{}) {
			t.Fatalf("rune %q should be handled", r)
		}
	}

	if viewer.down != 1 || viewer.up != 1 || viewer.pgDown != 1 || viewer.pgUp != 1 ||
		viewer.home != 1 || viewer.end != 1 || viewer.next != 1 || viewer.prev != 1 ||
		viewer.search != 1 || viewer.line != 1 || viewer.closed != 1 {
		t.Fatalf("viewer actions = %+v, want each less action once", viewer)
	}
}

func TestFileViewerHandlerNavigationKeys(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer)

	for _, key := range []fyne.KeyName{
		fyne.KeyDown,
		fyne.KeyUp,
		fyne.KeyPageDown,
		fyne.KeyPageUp,
		fyne.KeyHome,
		fyne.KeyEnd,
	} {
		if !handler.OnKeyDown(&fyne.KeyEvent{Name: key}, ModifierState{}) {
			t.Fatalf("key %s should be handled", key)
		}
	}

	if viewer.down != 1 || viewer.up != 1 || viewer.pgDown != 1 ||
		viewer.pgUp != 1 || viewer.home != 1 || viewer.end != 1 {
		t.Fatalf("viewer actions = %+v, want each navigation action once", viewer)
	}
}
