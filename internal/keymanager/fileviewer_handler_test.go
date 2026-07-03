package keymanager

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"nmf/internal/config"
)

type fakeFileViewer struct {
	closed int
	down   int
	up     int
	pgDown int
	pgUp   int
	home   int
	end    int
	left   int
	right  int
	wrap   int
	text   int
	md     int
	hex    int
	next   int
	prev   int
	search int
	line   int
	copy   int
	all    int
}

func (f *fakeFileViewer) CloseViewer()          { f.closed++ }
func (f *fakeFileViewer) ViewerLineDown()       { f.down++ }
func (f *fakeFileViewer) ViewerLineUp()         { f.up++ }
func (f *fakeFileViewer) ViewerPageDown()       { f.pgDown++ }
func (f *fakeFileViewer) ViewerPageUp()         { f.pgUp++ }
func (f *fakeFileViewer) ViewerHome()           { f.home++ }
func (f *fakeFileViewer) ViewerEnd()            { f.end++ }
func (f *fakeFileViewer) ViewerColumnLeft()     { f.left++ }
func (f *fakeFileViewer) ViewerColumnRight()    { f.right++ }
func (f *fakeFileViewer) ViewerToggleWrap()     { f.wrap++ }
func (f *fakeFileViewer) ViewerShowText()       { f.text++ }
func (f *fakeFileViewer) ViewerShowMarkdown()   { f.md++ }
func (f *fakeFileViewer) ViewerShowHex()        { f.hex++ }
func (f *fakeFileViewer) ViewerSearchNext()     { f.next++ }
func (f *fakeFileViewer) ViewerSearchPrevious() { f.prev++ }
func (f *fakeFileViewer) ViewerFocusSearch()    { f.search++ }
func (f *fakeFileViewer) ViewerFocusLine()      { f.line++ }
func (f *fakeFileViewer) ViewerCopySelection()  { f.copy++ }
func (f *fakeFileViewer) ViewerSelectAll()      { f.all++ }

func TestFileViewerHandlerLessKeys(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer, nil)

	for _, r := range []rune{'j', 'k', 'h', 'l', 'f', 'b', 'g', 'G', 'w', 't', 'm', 'x', 'n', 'N', '/', ':', 'q'} {
		if !handler.OnTypedRune(r, ModifierState{}) {
			t.Fatalf("rune %q should be handled", r)
		}
	}

	if viewer.down != 1 || viewer.up != 1 || viewer.pgDown != 1 || viewer.pgUp != 1 ||
		viewer.home != 1 || viewer.end != 1 || viewer.left != 1 || viewer.right != 1 ||
		viewer.wrap != 1 || viewer.text != 1 || viewer.md != 1 || viewer.hex != 1 ||
		viewer.next != 1 || viewer.prev != 1 || viewer.search != 1 ||
		viewer.line != 1 || viewer.closed != 1 {
		t.Fatalf("viewer actions = %+v, want each less action once", viewer)
	}
}

func TestFileViewerHandlerCtrlCCopiesSelection(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer, nil)

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyC}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+C should be handled")
	}
	if viewer.copy != 1 {
		t.Fatalf("copy calls = %d, want 1", viewer.copy)
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyC}, ModifierState{}) {
		t.Fatal("plain C should not be handled as an activation")
	}
}

func TestFileViewerHandlerCtrlASelectsAll(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer, nil)

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{CtrlPressed: true}) {
		t.Fatal("Ctrl+A should be handled")
	}
	if viewer.all != 1 {
		t.Fatalf("select all calls = %d, want 1", viewer.all)
	}
	if handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{}) {
		t.Fatal("plain A should not be handled as an activation")
	}
}

func TestFileViewerHandlerNavigationKeys(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer, nil)

	for _, key := range []fyne.KeyName{
		fyne.KeyDown,
		fyne.KeyUp,
		fyne.KeyLeft,
		fyne.KeyRight,
		fyne.KeyPageDown,
		fyne.KeyPageUp,
		fyne.KeyHome,
		fyne.KeyEnd,
	} {
		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: key}, ModifierState{}) {
			t.Fatalf("key %s should be handled", key)
		}
	}

	if viewer.down != 1 || viewer.up != 1 || viewer.pgDown != 1 ||
		viewer.pgUp != 1 || viewer.home != 1 || viewer.end != 1 ||
		viewer.left != 1 || viewer.right != 1 {
		t.Fatalf("viewer actions = %+v, want each navigation action once", viewer)
	}
}

func TestFileViewerConfiguredBindingOverridesDefaultRune(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer, nil, []config.KeyBindingEntry{
		{Target: KeyBindingTargetFileViewer, Key: "J", Command: CommandFileViewerPageDown},
		{Target: KeyBindingTargetFileViewer, Key: "S-Semicolon", Command: CommandNoop},
	})

	if !handler.OnTypedRune('j', ModifierState{}) {
		t.Fatal("configured j should be handled")
	}
	if viewer.pgDown != 1 || viewer.down != 0 {
		t.Fatalf("viewer actions = %+v, want page down only", viewer)
	}
	if !handler.OnTypedRune(':', ModifierState{}) {
		t.Fatal("configured : noop should be handled")
	}
	if viewer.line != 0 {
		t.Fatalf("line focus calls = %d, want 0", viewer.line)
	}
}

// TestFileViewerHandlerLetterFiresOnceThroughKeyManager is a regression test
// for a bug where letter bindings fired twice per physical press. Fyne's
// GLFW driver delivers both a TypedKey and a TypedRune for one physical
// printable-letter press, and KeyManager's gate forwards both events of the
// same press to the current handler. Before the fix, FileViewerKeyHandler
// matched the same binding list from both OnKeyActivated (raw key) and
// OnTypedRune (via fileViewerRuneKey's reverse mapping), so a single press
// executed the bound command twice. Each case below drives the KeyManager
// through the real event sequence a press produces.
func TestFileViewerHandlerLetterFiresOnceThroughKeyManager(t *testing.T) {
	t.Run("j line down", func(t *testing.T) {
		viewer := &fakeFileViewer{}
		km := NewKeyManager(func(string, ...interface{}) {})
		km.PushHandler(NewFileViewerKeyHandler(viewer, nil))

		km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyJ})
		km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyJ})
		km.HandleTypedRune('j')

		if viewer.down != 1 {
			t.Fatalf("line-down calls = %d, want 1", viewer.down)
		}
	})

	t.Run("w toggle wrap", func(t *testing.T) {
		viewer := &fakeFileViewer{}
		km := NewKeyManager(func(string, ...interface{}) {})
		km.PushHandler(NewFileViewerKeyHandler(viewer, nil))

		km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyW})
		km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyW})
		km.HandleTypedRune('w')

		if viewer.wrap != 1 {
			t.Fatalf("wrap toggle calls = %d, want 1", viewer.wrap)
		}
	})

	t.Run("shift+g end", func(t *testing.T) {
		viewer := &fakeFileViewer{}
		km := NewKeyManager(func(string, ...interface{}) {})
		km.PushHandler(NewFileViewerKeyHandler(viewer, nil))

		km.HandleKeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
		km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyG})
		km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyG})
		km.HandleTypedRune('G')

		if viewer.end != 1 {
			t.Fatalf("end calls = %d, want 1", viewer.end)
		}
	})

	t.Run("down arrow has no rune", func(t *testing.T) {
		viewer := &fakeFileViewer{}
		km := NewKeyManager(func(string, ...interface{}) {})
		km.PushHandler(NewFileViewerKeyHandler(viewer, nil))

		km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyDown})
		km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})

		if viewer.down != 1 {
			t.Fatalf("line-down calls = %d, want 1", viewer.down)
		}
	})
}

func TestFileViewerConfiguredBindingOverridesDefaultCtrlA(t *testing.T) {
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer, nil, []config.KeyBindingEntry{
		{Target: KeyBindingTargetFileViewer, Key: "C-A", Command: CommandNoop},
	})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyA}, ModifierState{CtrlPressed: true}) {
		t.Fatal("configured Ctrl+A noop should be handled")
	}
	if viewer.all != 0 {
		t.Fatalf("select all calls = %d, want 0", viewer.all)
	}
}

// TestNewFileViewerKeyHandlerSurfacesConstructionWarnings is a regression
// test for a bug where NewFileViewerKeyHandler hardcoded a no-op debugPrint,
// so buildTargetKeyBindings warnings (invalid key spec, unknown command) were
// silently dropped instead of reaching the caller-supplied debugPrint.
func TestNewFileViewerKeyHandlerSurfacesConstructionWarnings(t *testing.T) {
	var messages []string
	debugPrint := func(format string, args ...interface{}) {
		messages = append(messages, fmt.Sprintf(format, args...))
	}
	viewer := &fakeFileViewer{}
	handler := NewFileViewerKeyHandler(viewer, debugPrint, []config.KeyBindingEntry{
		{Target: KeyBindingTargetFileViewer, Key: "not-a-real-key", Command: CommandFileViewerClose},
		{Target: KeyBindingTargetFileViewer, Key: "J", Command: "not.a.real.command"},
	})
	if handler == nil {
		t.Fatal("handler should still be constructed despite invalid bindings")
	}
	if len(messages) != 2 {
		t.Fatalf("construction warnings = %#v, want 2 (invalid key spec + unknown command)", messages)
	}
}
