package ui

import "fyne.io/fyne/v2/widget"

// TabEntry is an Entry that accepts Tab (prevents focus traversal).
// It embeds widget.Entry and implements fyne.Tabbable.
type TabEntry struct {
	widget.Entry
	acceptTab bool
}

// NewTabEntry creates a new TabEntry with Tab capture enabled.
func NewTabEntry() *TabEntry {
	e := &TabEntry{acceptTab: true}
	e.ExtendBaseWidget(e)
	return e
}

// AcceptsTab indicates this entry consumes Tab so focus will not move.
func (e *TabEntry) AcceptsTab() bool { return e.acceptTab }

// SetTabCapture toggles Tab capture behavior.
func (e *TabEntry) SetTabCapture(on bool) { e.acceptTab = on }
