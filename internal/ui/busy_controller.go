package ui

import (
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/keymanager"
)

// BusyController owns the delayed overlay and input guard for one window. It is
// confined to the Fyne UI thread and is not safe for concurrent use. Only the
// delay timer crosses goroutines, and it marshals state and visual work back to
// the UI thread before calling showIfCurrent.
type BusyController struct {
	window     fyne.Window
	overlay    *BusyOverlay
	keyManager *keymanager.KeyManager
	delay      time.Duration
	debugPrint func(format string, args ...interface{})
	runOnMain  func(func())

	active     bool
	text       string
	timer      *time.Timer
	token      keymanager.HandlerToken
	generation uint64
}

func NewBusyController(
	window fyne.Window,
	keyManager *keymanager.KeyManager,
	themeProvider ThemeColorProvider,
	delay time.Duration,
	debugPrint func(format string, args ...interface{}),
) *BusyController {
	return &BusyController{
		window:     window,
		overlay:    NewBusyOverlay(themeProvider),
		keyManager: keyManager,
		delay:      max(delay, 0),
		debugPrint: debugPrint,
		runOnMain:  fyne.Do,
	}
}

func (c *BusyController) GetContainer() *fyne.Container {
	if c == nil || c.overlay == nil {
		return nil
	}
	return c.overlay.GetContainer()
}

func (c *BusyController) Active() bool {
	if c == nil {
		return false
	}
	return c.active
}

// Begin blocks keyboard input immediately and shows the overlay after the
// configured delay. Beginning again while active only updates the text,
// preserving the existing input owner.
func (c *BusyController) Begin(text string, onCancel func()) {
	if c == nil {
		return
	}
	if c.active {
		c.text = text
		if c.overlay != nil {
			c.overlay.Show(c.window, text)
		}
		c.debugf("BusyController: update text=%q", text)
		return
	}

	c.active = true
	c.text = text
	c.generation++
	generation := c.generation
	c.debugf("BusyController: begin text=%q", text)

	if c.keyManager != nil {
		c.token = c.keyManager.PushHandler(keymanager.NewBusyKeyHandler(onCancel))
	}
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(c.delay, func() {
		c.runOnMain(func() {
			c.showIfCurrent(generation)
		})
	})
}

func (c *BusyController) showIfCurrent(generation uint64) {
	if !c.active || c.generation != generation || c.overlay == nil {
		return
	}
	c.overlay.Show(c.window, c.text)
	c.debugf("BusyController: show text=%q", c.text)
}

// UpdateText updates an active overlay without changing the input handler.
func (c *BusyController) UpdateText(text string) {
	if c == nil {
		return
	}
	if !c.active {
		return
	}
	c.text = text
	if c.overlay != nil {
		c.overlay.Show(c.window, text)
	}
}

// End hides the overlay and releases exactly the input-handler token owned by
// this controller. It is safe to call repeatedly.
func (c *BusyController) End() {
	if c == nil {
		return
	}
	if !c.active {
		return
	}
	c.active = false
	c.generation++
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	token := c.token
	c.token = 0

	if c.overlay != nil {
		c.overlay.Hide()
	}
	if c.keyManager != nil && token != 0 {
		c.keyManager.RemoveHandler(token)
	}
	c.debugf("BusyController: end")
}

func (c *BusyController) debugf(format string, args ...interface{}) {
	if c.debugPrint != nil {
		c.debugPrint(format, args...)
	}
}
