package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowMessageDialog displays a simple OK dialog with a title and message.
// It returns immediately after showing.
func ShowMessageDialog(parent fyne.Window, title, message string) {
	d := dialog.NewInformation(title, message, parent)
	d.Show()
}

// ShowCompactMessageDialog displays a small acknowledgement dialog without the
// large information icon used by Fyne's default information dialog.
func ShowCompactMessageDialog(parent fyne.Window, title, message string) {
	var d *dialog.CustomDialog
	closeDialog := func() {
		if d != nil {
			d.Hide()
		}
	}

	label := widget.NewLabel(message)
	label.Alignment = fyne.TextAlignCenter
	label.Wrapping = fyne.TextWrapOff
	messageBox := container.NewGridWrap(fyne.NewSize(320, 48), container.NewCenter(label))
	content := container.NewVBox(
		messageBox,
		container.NewGridWithColumns(1, widget.NewButton("OK", closeDialog)),
	)
	sink := newCompactMessageSink(content, closeDialog)

	d = dialog.NewCustomWithoutButtons(title, sink, parent)
	d.Show()
	d.Resize(fyne.NewSize(360, 130))
	if parent != nil {
		parent.Canvas().Focus(sink)
	}
}

type compactMessageSink struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onClose func()
}

func newCompactMessageSink(content fyne.CanvasObject, onClose func()) *compactMessageSink {
	s := &compactMessageSink{content: content, onClose: onClose}
	s.ExtendBaseWidget(s)
	return s
}

func (s *compactMessageSink) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.content)
}

func (s *compactMessageSink) FocusGained() {}

func (s *compactMessageSink) FocusLost() {}

func (s *compactMessageSink) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyEscape, fyne.KeyReturn:
		if s.onClose != nil {
			s.onClose()
		}
	}
}

func (s *compactMessageSink) TypedRune(_ rune) {}
