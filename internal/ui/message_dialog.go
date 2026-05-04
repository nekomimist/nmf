package ui

import (
	"math"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
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
	label.Alignment = fyne.TextAlignLeading
	label.Wrapping = fyne.TextWrapBreak
	messageSize, dialogSize := compactMessageDialogSizes(message)
	messageBox := container.NewGridWrap(messageSize, container.NewPadded(label))
	content := container.NewVBox(
		messageBox,
		container.NewGridWithColumns(1, widget.NewButton("OK", closeDialog)),
	)
	sink := newCompactMessageSink(content, closeDialog)

	d = dialog.NewCustomWithoutButtons(title, sink, parent)
	d.Show()
	d.Resize(dialogSize)
	if parent != nil {
		parent.Canvas().Focus(sink)
	}
}

func compactMessageDialogSizes(message string) (fyne.Size, fyne.Size) {
	const (
		messageWidth      = float32(520)
		minMessageHeight  = float32(72)
		lineHeight        = float32(28)
		verticalPadding   = float32(24)
		dialogExtraWidth  = float32(40)
		dialogExtraHeight = float32(92)
	)

	lines := compactMessageEstimatedLineCount(message, 52)
	messageHeight := maxFloat32(minMessageHeight, float32(lines)*lineHeight+verticalPadding)
	messageSize := fyne.NewSize(messageWidth, messageHeight)
	return messageSize, fyne.NewSize(messageWidth+dialogExtraWidth, messageHeight+dialogExtraHeight)
}

func compactMessageEstimatedLineCount(message string, charsPerLine int) int {
	if charsPerLine <= 0 {
		return 1
	}
	lines := 0
	for _, line := range strings.Split(message, "\n") {
		length := utf8.RuneCountInString(line)
		if length == 0 {
			lines++
			continue
		}
		lines += int(math.Ceil(float64(length) / float64(charsPerLine)))
	}
	if lines < 1 {
		return 1
	}
	return lines
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

type compactMessageSink struct {
	widget.BaseWidget
	content           fyne.CanvasObject
	onClose           func()
	pressedDismissKey fyne.KeyName
}

var _ desktop.Keyable = (*compactMessageSink)(nil)

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

func (s *compactMessageSink) KeyDown(ev *fyne.KeyEvent) {
	if isCompactMessageDismissKey(ev.Name) {
		s.pressedDismissKey = normalizeCompactMessageDismissKey(ev.Name)
	}
}

func (s *compactMessageSink) KeyUp(ev *fyne.KeyEvent) {
	if !isCompactMessageDismissKey(ev.Name) {
		return
	}

	pressedKey := s.pressedDismissKey
	s.pressedDismissKey = ""
	if pressedKey != normalizeCompactMessageDismissKey(ev.Name) {
		return
	}
	if s.onClose != nil {
		s.onClose()
	}
}

func (s *compactMessageSink) TypedKey(ev *fyne.KeyEvent) {
	if isCompactMessageDismissKey(ev.Name) {
		if s.pressedDismissKey == normalizeCompactMessageDismissKey(ev.Name) {
			return
		}
		if s.onClose != nil {
			s.onClose()
		}
	}
}

func (s *compactMessageSink) TypedRune(_ rune) {}

func isCompactMessageDismissKey(name fyne.KeyName) bool {
	switch name {
	case fyne.KeyEscape, fyne.KeyReturn, fyne.KeyEnter:
		return true
	default:
		return false
	}
}

func normalizeCompactMessageDismissKey(name fyne.KeyName) fyne.KeyName {
	if name == fyne.KeyEnter {
		return fyne.KeyReturn
	}
	return name
}
