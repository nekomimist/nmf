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
	onKey    func(*fyne.KeyEvent) bool
	onRune   func(rune) bool
	onScroll func(float32) bool
}

func NewReadOnlyEntry(text string, onCancel func(), onKey func(*fyne.KeyEvent) bool, onRune ...func(rune) bool) *ReadOnlyEntry {
	e := &ReadOnlyEntry{onCancel: onCancel}
	e.onKey = onKey
	if len(onRune) > 0 {
		e.onRune = onRune[0]
	}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapOff
	e.Scroll = fyne.ScrollBoth
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.ExtendBaseWidget(e)
	e.SetText(text)
	return e
}

func (e *ReadOnlyEntry) SetScrollHandler(onScroll func(float32) bool) {
	e.onScroll = onScroll
}

func (e *ReadOnlyEntry) Scrolled(ev *fyne.ScrollEvent) {
	if e.onScroll != nil && e.onScroll(ev.Scrolled.DY) {
		return
	}
}

func (e *ReadOnlyEntry) TypedRune(r rune) {
	if e.onRune != nil {
		e.onRune(r)
	}
}

func (e *ReadOnlyEntry) TypedKey(ev *fyne.KeyEvent) {
	if ev == nil {
		return
	}
	if e.onKey != nil && e.onKey(ev) {
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
