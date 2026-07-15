package ui

import (
	"image/color"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

var _ fyne.Focusable = (*CommandMenu)(nil)

type commandMenuItem struct {
	label     string
	key       string
	separator bool
	action    func()
}

// CommandMenu is a focusable popup menu with single-key accelerators.
type CommandMenu struct {
	widget.BaseWidget

	items               []commandMenuItem
	selected            int
	overlay             *commandMenuOverlay
	canvas              fyne.Canvas
	onDismiss           func()
	resetTransientState func(string)
	dismissed           bool
}

// NewCommandMenu creates a command menu from UI-agnostic menu entries.
func NewCommandMenu(items []keymanager.CommandMenuItem, onDismiss func()) *CommandMenu {
	menu := &CommandMenu{
		items:     make([]commandMenuItem, 0, len(items)),
		selected:  -1,
		onDismiss: onDismiss,
	}
	usedKeys := make(map[string]struct{})
	for _, item := range items {
		key := normalizeMenuAccelerator(item.Key)
		normalizedKey := strings.ToUpper(key)
		if key != "" {
			if _, exists := usedKeys[normalizedKey]; exists {
				key = ""
			} else {
				usedKeys[normalizedKey] = struct{}{}
			}
		}
		menu.items = append(menu.items, commandMenuItem{
			label:     item.Label,
			key:       key,
			separator: item.Separator,
			action:    item.Action,
		})
	}
	menu.ExtendBaseWidget(menu)
	menu.selectFirst()
	return menu
}

// SetTransientStateReset configures the callback used to clear stale input
// state when the menu takes or releases focus.
func (m *CommandMenu) SetTransientStateReset(callback func(string)) {
	m.resetTransientState = callback
}

func normalizeMenuAccelerator(key string) string {
	if key == "" {
		return ""
	}
	runes := []rune(key)
	if len(runes) != 1 || unicode.IsSpace(runes[0]) || !unicode.IsPrint(runes[0]) {
		return ""
	}
	return key
}

// ShowAtPosition displays the menu as an overlay and focuses it for keyboard input.
func (m *CommandMenu) ShowAtPosition(c fyne.Canvas, pos fyne.Position) {
	if c == nil {
		return
	}
	m.canvas = c
	m.overlay = newCommandMenuOverlay(m, c, pos)
	m.overlay.Show()
	c.Focus(m)
}

func (m *CommandMenu) FocusGained() {
	m.resetInputState("command-menu-focus-gained")
}

// FocusLost dismisses the menu. Dismiss itself resets transient input state,
// so no separate reset happens here.
func (m *CommandMenu) FocusLost() {
	m.Dismiss()
}

func (m *CommandMenu) TypedKey(ev *fyne.KeyEvent) {
	if ev == nil {
		return
	}
	switch ev.Name {
	case fyne.KeyDown:
		m.selectNext()
	case fyne.KeyUp:
		m.selectPrevious()
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		m.executeSelected()
	case fyne.KeyEscape:
		m.Dismiss()
	default:
		if len([]rune(string(ev.Name))) == 1 {
			m.executeKey(string(ev.Name))
		}
	}
}

func (m *CommandMenu) TypedRune(r rune) {
	m.executeKey(string(r))
}

// Dismiss closes the menu and runs the optional dismissal callback once.
func (m *CommandMenu) Dismiss() {
	if m.dismissed {
		return
	}
	m.dismissed = true
	m.resetInputState("command-menu-dismiss")
	if m.overlay != nil {
		m.overlay.Hide()
	}
	if m.onDismiss != nil {
		m.onDismiss()
	}
}

func (m *CommandMenu) resetInputState(label string) {
	if m.resetTransientState != nil {
		m.resetTransientState(label)
	}
}

// commandMenuOverlay is a full-canvas overlay root that hosts the command
// menu box at a fixed position on the canvas.
//
// Fyne v2.8.0's widget.PopUp overlay dismisses an outside tap through PopUp.Hide
// (see widget/popup.go and internal/widget/overlay_container.go), bypassing
// CommandMenu.Dismiss and therefore skipping resetInputState and onDismiss.
// PopUpMenu avoids that problem by routing its OverlayContainer dismissal
// through Dismiss. That type lives in an internal package we cannot import, so
// this replicates the same small amount of logic: a Tappable/SecondaryTappable
// root sized to the canvas, hosting the menu box (background rectangle +
// menuThemeOverride content) at a clamped position, that calls
// CommandMenu.Dismiss() whenever the tap lands outside the box.
type commandMenuOverlay struct {
	widget.BaseWidget

	menu       *CommandMenu
	canvas     fyne.Canvas
	content    fyne.CanvasObject
	background *canvas.Rectangle
	requested  fyne.Position
	boxSize    fyne.Size
	shown      bool
}

var (
	_ fyne.Widget            = (*commandMenuOverlay)(nil)
	_ fyne.Tappable          = (*commandMenuOverlay)(nil)
	_ fyne.SecondaryTappable = (*commandMenuOverlay)(nil)
)

func newCommandMenuOverlay(menu *CommandMenu, c fyne.Canvas, pos fyne.Position) *commandMenuOverlay {
	content := menuThemeOverride(menu)
	o := &commandMenuOverlay{
		menu:       menu,
		canvas:     c,
		content:    content,
		background: canvas.NewRectangle(color.Transparent),
		requested:  pos,
	}
	o.ExtendBaseWidget(o)
	o.boxSize = content.MinSize()
	return o
}

// Show adds the overlay to the canvas's overlay stack, sized to the full
// canvas, and reveals it. The explicit Refresh mirrors widget.PopUp.Show:
// BaseWidget.Show is a no-op when Hidden's zero value (false) already reads
// as visible, so without it the background rectangle would never pick up its
// themed fill color.
func (o *commandMenuOverlay) Show() {
	if !o.shown {
		o.canvas.Overlays().Add(o)
		o.shown = true
	}
	o.BaseWidget.Resize(o.canvas.Size())
	o.Refresh()
	o.BaseWidget.Show()
}

// Hide removes the overlay from the canvas's overlay stack.
func (o *commandMenuOverlay) Hide() {
	if o.shown {
		o.canvas.Overlays().Remove(o)
		o.shown = false
	}
	o.BaseWidget.Hide()
}

// Tapped dismisses the menu when the tap lands outside the menu box.
func (o *commandMenuOverlay) Tapped(ev *fyne.PointEvent) {
	if ev != nil && o.isInsideBox(ev.Position) {
		return
	}
	if o.menu != nil {
		o.menu.Dismiss()
	}
}

// TappedSecondary dismisses the menu the same way Tapped does; the overlay
// has no right-click behavior of its own.
func (o *commandMenuOverlay) TappedSecondary(ev *fyne.PointEvent) {
	o.Tapped(ev)
}

func (o *commandMenuOverlay) isInsideBox(pos fyne.Position) bool {
	boxPos := o.clampedBoxPosition()
	return pos.X >= boxPos.X && pos.Y >= boxPos.Y &&
		pos.X <= boxPos.X+o.boxSize.Width && pos.Y <= boxPos.Y+o.boxSize.Height
}

// clampedBoxPosition keeps the menu box within the canvas's interactive
// area, mirroring widget.PopUpMenu.adjustedPosition.
func (o *commandMenuOverlay) clampedBoxPosition() fyne.Position {
	x, y := o.requested.X, o.requested.Y
	_, areaSize := o.canvas.InteractiveArea()
	if x+o.boxSize.Width > areaSize.Width {
		x = areaSize.Width - o.boxSize.Width
		if x < 0 {
			x = 0
		}
	}
	if y+o.boxSize.Height > areaSize.Height {
		y = areaSize.Height - o.boxSize.Height
		if y < 0 {
			y = 0
		}
	}
	return fyne.NewPos(x, y)
}

func (o *commandMenuOverlay) CreateRenderer() fyne.WidgetRenderer {
	return &commandMenuOverlayRenderer{overlay: o, objects: []fyne.CanvasObject{o.background, o.content}}
}

type commandMenuOverlayRenderer struct {
	overlay *commandMenuOverlay
	objects []fyne.CanvasObject
}

func (r *commandMenuOverlayRenderer) Destroy() {}

func (r *commandMenuOverlayRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *commandMenuOverlayRenderer) Layout(fyne.Size) {
	boxPos := r.overlay.clampedBoxPosition()
	r.overlay.background.Move(boxPos)
	r.overlay.background.Resize(r.overlay.boxSize)
	r.overlay.content.Move(boxPos)
	r.overlay.content.Resize(r.overlay.boxSize)
}

func (r *commandMenuOverlayRenderer) MinSize() fyne.Size {
	return r.overlay.canvas.Size()
}

func (r *commandMenuOverlayRenderer) Refresh() {
	th := r.overlay.Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	r.overlay.background.FillColor = th.Color(theme.ColorNameOverlayBackground, variant)
	r.Layout(fyne.Size{})
	r.overlay.background.Refresh()
	r.overlay.content.Refresh()
}

func (m *CommandMenu) CreateRenderer() fyne.WidgetRenderer {
	rows := make([]fyne.CanvasObject, 0, len(m.items))
	for i := range m.items {
		rows = append(rows, newCommandMenuRow(m, i))
	}
	box := container.New(layout.NewVBoxLayout(), rows...)
	return widget.NewSimpleRenderer(box)
}

func (m *CommandMenu) selectFirst() {
	for i := range m.items {
		if !m.items[i].separator {
			m.selected = i
			return
		}
	}
	m.selected = -1
}

func (m *CommandMenu) selectNext() {
	if len(m.items) == 0 {
		return
	}
	start := m.selected
	for i := 1; i <= len(m.items); i++ {
		next := (start + i + len(m.items)) % len(m.items)
		if !m.items[next].separator {
			m.selected = next
			m.Refresh()
			return
		}
	}
}

func (m *CommandMenu) selectPrevious() {
	if len(m.items) == 0 {
		return
	}
	start := m.selected
	for i := 1; i <= len(m.items); i++ {
		next := (start - i + len(m.items)) % len(m.items)
		if !m.items[next].separator {
			m.selected = next
			m.Refresh()
			return
		}
	}
}

func (m *CommandMenu) executeSelected() {
	if m.selected < 0 || m.selected >= len(m.items) || m.items[m.selected].separator {
		return
	}
	m.execute(m.selected)
}

func (m *CommandMenu) executeKey(key string) {
	if key == "" {
		return
	}
	needle := strings.ToUpper(key)
	for i, item := range m.items {
		if item.separator || item.key == "" {
			continue
		}
		if strings.ToUpper(item.key) == needle {
			m.execute(i)
			return
		}
	}
}

func (m *CommandMenu) execute(index int) {
	if index < 0 || index >= len(m.items) || m.items[index].separator {
		return
	}
	action := m.items[index].action
	m.Dismiss()
	if action != nil {
		action()
	}
}

func (m *CommandMenu) selectIndex(index int) {
	if index < 0 || index >= len(m.items) || m.items[index].separator {
		return
	}
	m.selected = index
	m.Refresh()
}

type commandMenuRow struct {
	widget.BaseWidget
	menu  *CommandMenu
	index int
}

var (
	_ fyne.Tappable     = (*commandMenuRow)(nil)
	_ desktop.Hoverable = (*commandMenuRow)(nil)
)

func newCommandMenuRow(menu *CommandMenu, index int) *commandMenuRow {
	row := &commandMenuRow{menu: menu, index: index}
	row.ExtendBaseWidget(row)
	return row
}

func (r *commandMenuRow) Tapped(*fyne.PointEvent) {
	r.menu.execute(r.index)
}

func (r *commandMenuRow) MouseIn(*desktop.MouseEvent) {
	r.menu.selectIndex(r.index)
}

func (r *commandMenuRow) MouseMoved(*desktop.MouseEvent) {}
func (r *commandMenuRow) MouseOut()                      {}

func (r *commandMenuRow) CreateRenderer() fyne.WidgetRenderer {
	item := r.menu.items[r.index]
	if item.separator {
		sep := widget.NewSeparator()
		return widget.NewSimpleRenderer(sep)
	}
	th := r.Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	background := canvas.NewRectangle(color.Transparent)
	label := canvas.NewText(item.label, th.Color(theme.ColorNameForeground, variant))
	key := canvas.NewText(item.key, shortcutTextColor(th, variant))
	key.Alignment = fyne.TextAlignTrailing
	return &commandMenuRowRenderer{
		objects:    []fyne.CanvasObject{background, label, key},
		row:        r,
		background: background,
		label:      label,
		key:        key,
	}
}

type commandMenuRowRenderer struct {
	objects    []fyne.CanvasObject
	row        *commandMenuRow
	background *canvas.Rectangle
	label      *canvas.Text
	key        *canvas.Text
}

func (r *commandMenuRowRenderer) Destroy() {}

func (r *commandMenuRowRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *commandMenuRowRenderer) Layout(size fyne.Size) {
	padding := r.row.Theme().Size(theme.SizeNamePadding)
	keySize := r.key.MinSize()
	labelSize := r.label.MinSize()
	r.background.Resize(size)
	y := (size.Height - labelSize.Height) / 2
	r.label.Move(fyne.NewPos(padding, y))
	r.label.Resize(fyne.NewSize(size.Width-keySize.Width-(padding*3), labelSize.Height))
	r.key.Move(fyne.NewPos(size.Width-keySize.Width-padding, y))
	r.key.Resize(fyne.NewSize(keySize.Width, labelSize.Height))
}

func (r *commandMenuRowRenderer) MinSize() fyne.Size {
	padding := r.row.Theme().Size(theme.SizeNamePadding)
	labelSize := r.label.MinSize()
	keySize := r.key.MinSize()
	return fyne.NewSize(labelSize.Width+keySize.Width+(padding*4), labelSize.Height+(padding*2))
}

func (r *commandMenuRowRenderer) Refresh() {
	th := r.row.Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	if r.row.menu.selected == r.row.index {
		r.background.FillColor = th.Color(theme.ColorNameFocus, variant)
	} else {
		r.background.FillColor = color.Transparent
	}
	r.label.Color = th.Color(theme.ColorNameForeground, variant)
	r.key.Color = shortcutTextColor(th, variant)
	r.background.Refresh()
	r.label.Refresh()
	r.key.Refresh()
	canvas.Refresh(r.row)
}

func shortcutTextColor(th fyne.Theme, variant fyne.ThemeVariant) color.Color {
	r, g, b, a := th.Color(theme.ColorNameForeground, variant).RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(float32(a>>8) * 0.72)}
}
