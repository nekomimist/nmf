package keymanager

import "fyne.io/fyne/v2"

// CommandErrorDialogInterface contains the actions available while reviewing
// a failed script. ScrollDetails uses signed line counts.
type CommandErrorDialogInterface interface {
	Close()
	CopyDetails()
	ScrollDetails(lines int)
}

func NewCommandErrorDialogHandler(d CommandErrorDialogInterface) KeyHandler {
	return newDialogKeyHandler("CommandErrorDialog", nil, []dialogBinding{
		{"Return", d.Close},
		{"Escape", d.Close},
		{"C-C", d.CopyDetails},
		{"Up", func() { d.ScrollDetails(-1) }},
		{"Down", func() { d.ScrollDetails(1) }},
		{"PageUp", func() { d.ScrollDetails(-10) }},
		{"PageDown", func() { d.ScrollDetails(10) }},
	}).withFallback(func(*fyne.KeyEvent, ModifierState) bool {
		return true
	}).withRune(func(rune, ModifierState) bool {
		return true
	})
}
