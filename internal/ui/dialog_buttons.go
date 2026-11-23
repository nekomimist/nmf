package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	// dialogButtonMinWidth is the shared minimum width applied to every dialog
	// action button so mixed-length labels line up without feeling oversized.
	dialogButtonMinWidth float32 = 88

	// dialogButtonBarTopInset adds a subtle gap above the action bar so buttons
	// do not sit flush against the body content or list above them.
	dialogButtonBarTopInset float32 = 4
)

// minWidthLayout wraps a single child and raises its reported width to min
// when smaller. It is a floor, never a cap: wider children keep their own
// width and height always follows the child.
type minWidthLayout struct{ min float32 }

func (l minWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	objects[0].Resize(size)
	objects[0].Move(fyne.NewPos(0, 0))
}

func (l minWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	min := objects[0].MinSize()
	if min.Width < l.min {
		min.Width = l.min
	}
	return min
}

func dialogCancelButton(text string, tapped func()) *widget.Button {
	return widget.NewButton(text, tapped)
}

func dialogConfirmButton(text string, tapped func()) *widget.Button {
	button := widget.NewButton(text, tapped)
	button.Importance = widget.HighImportance
	return button
}

// dialogDangerButton marks a destructive affirmative action (e.g. Quit's
// "Quit Anyway" or Permanent Delete's "Delete").
func dialogDangerButton(text string, tapped func()) *widget.Button {
	button := widget.NewButton(text, tapped)
	button.Importance = widget.DangerImportance
	return button
}

// dialogAuxButton is a plain, standard-importance action (neither the
// dismiss nor the primary button in the bar).
func dialogAuxButton(text string, icon fyne.Resource, tapped func()) *widget.Button {
	if icon != nil {
		return widget.NewButtonWithIcon(text, icon, tapped)
	}
	return widget.NewButton(text, tapped)
}

func wrapDialogButtons(buttons []*widget.Button) []fyne.CanvasObject {
	wrapped := make([]fyne.CanvasObject, len(buttons))
	for i, b := range buttons {
		wrapped[i] = container.New(minWidthLayout{min: dialogButtonMinWidth}, b)
	}
	return wrapped
}

func padDialogButtonBar(row fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(dialogButtonBarTopInset, 0, 0, 0), row)
}

// dialogButtonBar right-aligns dialog action buttons with a uniform minimum
// width. Pass buttons in order: [auxiliary...] [cancel/dismiss] [primary].
func dialogButtonBar(buttons ...*widget.Button) fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, 0, len(buttons)+1)
	objects = append(objects, layout.NewSpacer())
	objects = append(objects, wrapDialogButtons(buttons)...)
	return padDialogButtonBar(container.NewHBox(objects...))
}

// dialogSplitButtonBar places auxiliary actions on the leading edge and
// dismiss/primary actions on the trailing edge.
func dialogSplitButtonBar(leading []*widget.Button, trailing []*widget.Button) fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, 0, len(leading)+len(trailing)+1)
	objects = append(objects, wrapDialogButtons(leading)...)
	objects = append(objects, layout.NewSpacer())
	objects = append(objects, wrapDialogButtons(trailing)...)
	return padDialogButtonBar(container.NewHBox(objects...))
}

func dialogButtonRow(cancelText string, cancelTapped func(), confirmText string, confirmTapped func()) fyne.CanvasObject {
	return dialogButtonBar(
		dialogCancelButton(cancelText, cancelTapped),
		dialogConfirmButton(confirmText, confirmTapped),
	)
}

func dialogOKButtonRow(tapped func()) fyne.CanvasObject {
	return dialogButtonBar(dialogConfirmButton("OK", tapped))
}

// DialogButtonBar exposes dialogButtonBar to package main (drop_ui.go).
func DialogButtonBar(buttons ...*widget.Button) fyne.CanvasObject {
	return dialogButtonBar(buttons...)
}

// DialogSplitButtonBar exposes dialogSplitButtonBar to package main.
func DialogSplitButtonBar(leading []*widget.Button, trailing []*widget.Button) fyne.CanvasObject {
	return dialogSplitButtonBar(leading, trailing)
}

// DialogCancelButton exposes dialogCancelButton to package main.
func DialogCancelButton(text string, tapped func()) *widget.Button {
	return dialogCancelButton(text, tapped)
}

// DialogConfirmButton exposes dialogConfirmButton to package main.
func DialogConfirmButton(text string, tapped func()) *widget.Button {
	return dialogConfirmButton(text, tapped)
}

// DialogAuxButton exposes dialogAuxButton to package main.
func DialogAuxButton(text string, icon fyne.Resource, tapped func()) *widget.Button {
	return dialogAuxButton(text, icon, tapped)
}
