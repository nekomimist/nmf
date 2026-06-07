package ui

import (
	"math"
	"net/url"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ShowCompactMessageDialog displays a small acknowledgement dialog without the
// large information icon used by Fyne's default information dialog.
func ShowCompactMessageDialog(parent fyne.Window, title, message string) {
	ShowCompactMessageDialogWithOnClose(parent, title, message, nil)
}

// ShowCompactMessageDialogWithOnClose displays a compact acknowledgement dialog
// and runs onClose after the user dismisses it.
func ShowCompactMessageDialogWithOnClose(parent fyne.Window, title, message string, onClose func()) {
	fyne.Do(func() {
		var d *dialog.CustomDialog
		closed := false
		closeDialog := func() {
			if closed {
				return
			}
			closed = true
			if d != nil {
				d.Hide()
			}
			if onClose != nil {
				onClose()
			}
		}

		label := widget.NewLabel(message)
		label.Alignment = fyne.TextAlignLeading
		label.Wrapping = fyne.TextWrapBreak
		messageSize, dialogSize := compactMessageDialogSizes(message)
		messageBox := container.NewGridWrap(messageSize, container.NewPadded(label))
		content := container.NewVBox(
			messageBox,
			dialogOKButtonRow(closeDialog),
		)
		sink := newCompactMessageSink(content, closeDialog)

		d = dialog.NewCustomWithoutButtons(title, sink, parent)
		d.Show()
		d.Resize(dialogSize)
		if parent != nil {
			parent.Canvas().Focus(sink)
		}
	})
}

// ShowCompactVersionDialog displays app metadata with a clickable repository URL.
func ShowCompactVersionDialog(parent fyne.Window, software, repository, version string) {
	fyne.Do(func() {
		var d *dialog.CustomDialog
		closed := false
		closeDialog := func() {
			if closed {
				return
			}
			closed = true
			if d != nil {
				d.Hide()
			}
		}

		rows := versionDialogRows(software, repository, version)
		messageSize, dialogSize := versionDialogSizes(software, repository, version)
		messageBox := container.NewGridWrap(messageSize, container.NewPadded(rows))
		content := container.NewVBox(
			messageBox,
			dialogOKButtonRow(closeDialog),
		)
		sink := newCompactMessageSink(content, closeDialog)

		d = dialog.NewCustomWithoutButtons("Version", sink, parent)
		d.Show()
		d.Resize(dialogSize)
		if parent != nil {
			parent.Canvas().Focus(sink)
		}
	})
}

func versionDialogRepositoryValue(repository string) fyne.CanvasObject {
	parsed, err := url.Parse(repository)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return versionDialogValueLabel(repository)
	}
	link := widget.NewHyperlink(repository, parsed)
	link.Wrapping = fyne.TextWrapBreak
	return link
}

func versionDialogRows(software, repository, version string) fyne.CanvasObject {
	return container.NewVBox(
		versionDialogRow("Software:", versionDialogValueLabel(software), versionDialogValueHeight(software)),
		versionDialogRow("Repository:", versionDialogRepositoryValue(repository), versionDialogValueHeight(repository)),
		versionDialogRow("Version:", versionDialogValueLabel(version), versionDialogValueHeight(version)),
	)
}

func versionDialogRow(label string, value fyne.CanvasObject, height float32) fyne.CanvasObject {
	labelWidget := widget.NewLabel(label)
	return container.NewBorder(
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(versionDialogLabelWidth, height), labelWidget),
		nil,
		container.NewGridWrap(fyne.NewSize(versionDialogValueWidth, height), value),
	)
}

func versionDialogValueLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapBreak
	return label
}

func versionDialogSizes(software, repository, version string) (fyne.Size, fyne.Size) {
	contentHeight := versionDialogValueHeight(software) +
		versionDialogValueHeight(repository) +
		versionDialogValueHeight(version) +
		compactMessageVPadding
	messageHeight := maxFloat32(
		compactMessageMinHeight,
		contentHeight,
	)
	messageSize := metricsSize(compactMessageWidth, messageHeight)
	return messageSize, metricsSize(compactMessageWidth+compactDialogExtraWidth, messageHeight+compactDialogExtraHeight)
}

func versionDialogValueHeight(text string) float32 {
	lines := compactMessageEstimatedLineCount(text, versionDialogValueCharsPerLine)
	return maxFloat32(versionDialogRowHeight, float32(lines)*versionDialogWrappedLineHeight)
}

func versionDialogMessageText(software, repository, version string) string {
	return "Software: " + software +
		"\nRepository: " + repository +
		"\nVersion: " + version
}

func compactMessageDialogSizes(message string) (fyne.Size, fyne.Size) {
	lines := compactMessageEstimatedLineCount(message, compactMessageCharsPerLine)
	messageHeight := maxFloat32(compactMessageMinHeight, float32(lines)*compactMessageLineHeight+compactMessageVPadding)
	messageSize := metricsSize(compactMessageWidth, messageHeight)
	return messageSize, metricsSize(compactMessageWidth+compactDialogExtraWidth, messageHeight+compactDialogExtraHeight)
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
