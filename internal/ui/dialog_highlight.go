package ui

import (
	"fyne.io/fyne/v2"
)

type dialogHighlightState struct {
	frame       *HighlightFrame
	highlighted bool
}

func (s *dialogHighlightState) setHighlighted(highlighted bool) {
	s.highlighted = highlighted
	if s.frame != nil {
		s.frame.SetHighlighted(highlighted)
	}
}

func (s *dialogHighlightState) wrap(content fyne.CanvasObject) fyne.CanvasObject {
	s.frame = NewHighlightFrame(nil)
	s.frame.SetHighlighted(s.highlighted)
	return s.frame.WrapContent(content)
}
