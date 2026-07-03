package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"nmf/internal/keymanager"
)

func TestCommandMenuTypedRuneExecutesAcceleratorCaseInsensitive(t *testing.T) {
	called := 0
	dismissed := 0
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "Open Explorer Here", Key: "E", Action: func() { called++ }},
	}, func() { dismissed++ })

	menu.TypedRune('e')

	if called != 1 {
		t.Fatalf("action count = %d, want 1", called)
	}
	if dismissed != 1 {
		t.Fatalf("dismiss count = %d, want 1", dismissed)
	}
}

func TestCommandMenuUnmatchedKeyKeepsMenuOpen(t *testing.T) {
	called := 0
	dismissed := 0
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "Open Explorer Here", Key: "E", Action: func() { called++ }},
	}, func() { dismissed++ })

	menu.TypedRune('x')

	if called != 0 {
		t.Fatalf("action count = %d, want 0", called)
	}
	if dismissed != 0 {
		t.Fatalf("dismiss count = %d, want 0", dismissed)
	}
}

func TestCommandMenuDuplicateAcceleratorsFirstWins(t *testing.T) {
	var called []string
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "One", Key: "E", Action: func() { called = append(called, "one") }},
		{Label: "Two", Key: "e", Action: func() { called = append(called, "two") }},
	}, nil)

	if menu.items[0].key != "E" {
		t.Fatalf("first key = %q, want E", menu.items[0].key)
	}
	if menu.items[1].key != "" {
		t.Fatalf("duplicate key = %q, want empty", menu.items[1].key)
	}

	menu.TypedRune('e')

	if len(called) != 1 || called[0] != "one" {
		t.Fatalf("called = %#v, want first item only", called)
	}
}

func TestCommandMenuInvalidAcceleratorsAreIgnored(t *testing.T) {
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "Multi", Key: "EX"},
		{Label: "Space", Key: " "},
	}, nil)

	if menu.items[0].key != "" || menu.items[1].key != "" {
		t.Fatalf("keys = %q %q, want invalid keys ignored", menu.items[0].key, menu.items[1].key)
	}
}

func TestCommandMenuNavigationSkipsSeparators(t *testing.T) {
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Separator: true},
		{Label: "One"},
		{Separator: true},
		{Label: "Two"},
	}, nil)

	if menu.selected != 1 {
		t.Fatalf("initial selected = %d, want 1", menu.selected)
	}

	menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if menu.selected != 3 {
		t.Fatalf("selected after Down = %d, want 3", menu.selected)
	}

	menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	if menu.selected != 1 {
		t.Fatalf("selected after Up = %d, want 1", menu.selected)
	}
}

func TestCommandMenuEnterExecutesSelectedAndEscapeDismisses(t *testing.T) {
	called := 0
	dismissed := 0
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "One", Action: func() { called++ }},
	}, func() { dismissed++ })

	menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if called != 1 {
		t.Fatalf("action count = %d, want 1", called)
	}
	if dismissed != 1 {
		t.Fatalf("dismiss count = %d, want 1", dismissed)
	}

	menu = NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "One", Action: func() { called++ }},
	}, func() { dismissed++ })
	menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

	if called != 1 {
		t.Fatalf("action count after Escape = %d, want 1", called)
	}
	if dismissed != 2 {
		t.Fatalf("dismiss count after Escape = %d, want 2", dismissed)
	}
}

func TestCommandMenuResetsTransientStateOnFocusAndDismiss(t *testing.T) {
	var labels []string
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "One"},
	}, nil)
	menu.SetTransientStateReset(func(label string) {
		labels = append(labels, label)
	})

	menu.FocusGained()
	menu.Dismiss()

	want := []string{"command-menu-focus-gained", "command-menu-dismiss"}
	if len(labels) != len(want) {
		t.Fatalf("reset labels = %#v, want %#v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("reset labels = %#v, want %#v", labels, want)
		}
	}
}

func TestCommandMenuResetsTransientStateOnFocusLost(t *testing.T) {
	var labels []string
	dismissed := 0
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "One"},
	}, func() { dismissed++ })
	menu.SetTransientStateReset(func(label string) {
		labels = append(labels, label)
	})

	menu.FocusLost()

	// FocusLost only calls Dismiss (which itself resets transient state), so
	// exactly one reset happens, not a separate focus-lost reset plus dismiss.
	want := []string{"command-menu-dismiss"}
	if len(labels) != len(want) {
		t.Fatalf("reset labels = %#v, want %#v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("reset labels = %#v, want %#v", labels, want)
		}
	}
	if dismissed != 1 {
		t.Fatalf("dismiss count = %d, want 1", dismissed)
	}
}

// TestCommandMenuOutsideTapDismissesViaOverlay is a regression test for a bug
// where widget.PopUp's own Tapped hid the popup directly on an outside tap,
// bypassing CommandMenu.Dismiss and therefore skipping resetInputState
// (KeyManager.ResetTransientState) and onDismiss (FocusFileList). The menu
// now shows itself in a dedicated overlay whose outside tap always routes
// through Dismiss.
func TestCommandMenuOutsideTapDismissesViaOverlay(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	c := test.NewCanvas()

	dismissed := 0
	var labels []string
	menu := NewCommandMenu([]keymanager.CommandMenuItem{
		{Label: "One", Action: func() {}},
	}, func() { dismissed++ })
	menu.SetTransientStateReset(func(label string) {
		labels = append(labels, label)
	})

	menu.ShowAtPosition(c, fyne.NewPos(10, 10))

	if got := len(c.Overlays().List()); got != 1 {
		t.Fatalf("overlay count after show = %d, want 1", got)
	}
	overlay := menu.overlay
	if overlay == nil {
		t.Fatal("menu overlay was not created")
	}

	outside := fyne.NewPos(overlay.boxSize.Width+500, overlay.boxSize.Height+500)
	overlay.Tapped(&fyne.PointEvent{Position: outside})

	if dismissed != 1 {
		t.Fatalf("onDismiss calls = %d, want 1", dismissed)
	}
	if len(labels) == 0 || labels[len(labels)-1] != "command-menu-dismiss" {
		t.Fatalf("reset labels = %#v, want the last entry to be command-menu-dismiss", labels)
	}
	if got := len(c.Overlays().List()); got != 0 {
		t.Fatalf("overlay count after outside tap = %d, want 0", got)
	}
}
