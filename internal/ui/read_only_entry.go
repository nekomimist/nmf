package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// ReadOnlyEntry is a selectable multi-line Entry that suppresses editing while
// preserving normal cursor movement, selection, and copy shortcuts.
type ReadOnlyEntry struct {
	widget.Entry
	onCancel func()
}

func NewReadOnlyEntry(text string, onCancel func()) *ReadOnlyEntry {
	e := &ReadOnlyEntry{onCancel: onCancel}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapOff
	e.Scroll = fyne.ScrollBoth
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.ExtendBaseWidget(e)
	e.SetText(text)
	return e
}

func (e *ReadOnlyEntry) TypedRune(r rune) {
}

func (e *ReadOnlyEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev == nil {
		return
	}
	switch ev.Name {
	case fyne.KeyEscape:
		if e.onCancel != nil {
			e.onCancel()
		}
	case fyne.KeyBackspace, fyne.KeyDelete, fyne.KeyReturn, fyne.KeyEnter, fyne.KeyTab:
		return
	default:
		e.Entry.TypedKey(ev)
	}
}

func (e *ReadOnlyEntry) TypedShortcut(shortcut fyne.Shortcut) {
	switch shortcut.(type) {
	case *fyne.ShortcutCopy, *fyne.ShortcutSelectAll:
		e.Entry.TypedShortcut(shortcut)
	}
}
