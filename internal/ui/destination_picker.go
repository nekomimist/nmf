package ui

import (
	"image/color"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/search"
	customtheme "nmf/internal/theme"
)

// DestinationCandidate describes a destination and where it came from.
type DestinationCandidate struct {
	Path         string
	OpenInWindow bool
	LastUsed     time.Time // Navigation-history timestamp, not filesystem access time.
}

// destinationPicker owns filtering, selection, and scrolling shared by the
// transfer and compare dialogs. Each dialog retains its own close and
// key-handler lifecycle.
type destinationPicker struct {
	searchEntry           *CustomSearchEntry
	orderRadio            *widget.RadioGroup
	orderMode             string
	destList              *widget.List
	filteredDest, allDest []DestinationCandidate
	recentDest            []DestinationCandidate
	openDest              map[string]bool
	selectedPath          string
	selectedIdx           int
	matchers              *search.Provider
	destScroll            *dialogListScroller
	destEmpty             *widget.Label
	scrollRight           bool
	onPathChanged         func(string)
	widthCache            *DestinationWidthCache
}

// DestinationWidthCache keeps measurements for one window's most recent
// destination list. Only the Fyne thread accesses it.
type DestinationWidthCache struct {
	font   fyne.Resource
	size   float32
	widths map[string]float32
}

func (c *DestinationWidthCache) contentWidth(candidates []DestinationCandidate, minimum float32) float32 {
	if c == nil {
		c = &DestinationWidthCache{}
	}
	style := fyne.TextStyle{Monospace: true}
	font, size := fynetheme.Font(style), fynetheme.TextSize()
	if c.font != font || c.size != size {
		c.widths = nil
	}
	next := make(map[string]float32, len(candidates))
	padding := 4 * fynetheme.Padding()
	widest := minimum
	for _, candidate := range candidates {
		width, ok := c.widths[candidate.Path]
		if !ok {
			width = fyne.MeasureText(candidate.Path, size, style).Width
		}
		next[candidate.Path] = width
		widest = max(widest, width+padding)
	}
	c.font, c.size, c.widths = font, size, next
	return widest
}

// SetDestinationWidthCache shares path measurements across dialogs in the same window.
func (d *destinationPicker) SetDestinationWidthCache(cache *DestinationWidthCache) {
	d.widthCache = cache
}

func (d *destinationPicker) destinationArea(width, height float32) fyne.CanvasObject {
	size := metricsSize(width, height)
	d.destScroll = newDialogListScroller(d.destList, d.widthCache.contentWidth(d.allDest, width), width, height)
	d.destEmpty = widget.NewLabel("No matching destinations")
	d.destEmpty.Alignment = fyne.TextAlignCenter
	fixed := container.NewWithoutLayout(d.destScroll, d.destEmpty)
	fixed.Resize(size)
	d.destScroll.Resize(size)
	d.destEmpty.Resize(size)
	d.updateDestinationEmptyState()
	return fixed
}

func (d *destinationPicker) createWidgets(focus func()) {
	d.searchEntry = NewCustomSearchEntry()
	d.searchEntry.SetPlaceHolder("Type to filter destination...")
	d.searchEntry.OnChanged = func(q string) { d.updateFiltered(q) }
	d.destList = widget.NewList(
		func() int { return len(d.filteredDest) },
		func() fyne.CanvasObject {
			text := canvas.NewText("", currentAppThemeColor(fynetheme.ColorNameForeground))
			text.TextStyle = fyne.TextStyle{Monospace: true}
			text.TextSize = fynetheme.TextSize()
			return text
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(d.filteredDest) {
				return
			}
			path := d.filteredDest[id].Path
			if text, ok := obj.(*canvas.Text); ok {
				text.Text = path
				text.TextSize = fynetheme.TextSize()
				text.Color = d.destinationTextColor(path)
				text.Refresh()
			}
		},
	)
	d.destList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && int(id) < len(d.filteredDest) {
			d.selectedIdx = int(id)
			d.selectedPath = d.filteredDest[id].Path
			d.notifySelectedPathChanged()
			d.applyHorizontalScroll()
			if focus != nil {
				focus()
			}
		}
	}
	d.orderRadio = newHistoryOrderRadio(func(mode string) {
		if d.orderMode == mode {
			return
		}
		d.orderMode = mode
		d.updateFiltered(d.GetSearchText())
		if focus != nil {
			focus()
		}
	})
}

// updateFiltered updates destination list
func (d *destinationPicker) updateFiltered(q string) {
	ordered := d.allDest
	if d.orderMode == navigationHistoryOrderLastUsed {
		if d.recentDest == nil {
			d.recentDest = append([]DestinationCandidate(nil), d.allDest...)
			sort.SliceStable(d.recentDest, func(i, j int) bool { return d.recentDest[i].LastUsed.After(d.recentDest[j].LastUsed) })
		}
		ordered = d.recentDest
	}
	if q == "" {
		d.filteredDest = ordered
	} else {
		matcher := d.matchers.Build(q)
		d.filteredDest = nil
		for _, p := range ordered {
			if matcher.Match(p.Path) {
				d.filteredDest = append(d.filteredDest, p)
			}
		}
	}
	if len(d.filteredDest) > 0 {
		d.selectedIdx = 0
		d.selectedPath = d.filteredDest[0].Path
		d.destList.Select(0)
		d.notifySelectedPathChanged()
		d.applyHorizontalScroll()
	} else {
		d.selectedIdx = -1
		d.selectedPath = ""
		d.notifySelectedPathChanged()
	}
	d.destList.Refresh()
	d.updateDestinationEmptyState()
}

// ToggleHistoryOrder switches between frecency and last-access ordering.
func (d *destinationPicker) ToggleHistoryOrder() {
	toggleHistoryOrder(d.orderRadio)
}

// SetOnSelectedPathChanged sets a callback for destination selection changes.
func (d *destinationPicker) SetOnSelectedPathChanged(callback func(string)) {
	d.onPathChanged = callback
	d.notifySelectedPathChanged()
}

func (d *destinationPicker) notifySelectedPathChanged() {
	if d.onPathChanged != nil {
		d.onPathChanged(d.selectedPath)
	}
}

// SetDestinations replaces destination candidates while preserving order mode and search.
func (d *destinationPicker) SetDestinations(candidates []DestinationCandidate, preferredPath string) {
	previousPath := d.selectedPath
	query := d.GetSearchText()
	d.allDest = append([]DestinationCandidate(nil), candidates...)
	d.recentDest = nil
	d.openDest = destinationOpenMap(d.allDest)
	d.updateFiltered(query)

	if preferredPath != "" && d.selectFilteredPath(preferredPath) {
		return
	}
	if previousPath != "" {
		d.selectFilteredPath(previousPath)
	}
}

// Interface methods used by key handler
func (d *destinationPicker) MoveUp() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		i := d.selectedIdx - 1
		if i < 0 {
			i = 0
		}
		if i != d.selectedIdx {
			d.destList.Select(widget.ListItemID(i))
		}
	}
}

func (d *destinationPicker) MoveDown() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		i := d.selectedIdx + 1
		m := len(d.filteredDest) - 1
		if i > m {
			i = m
		}
		if i != d.selectedIdx {
			d.destList.Select(widget.ListItemID(i))
		}
	}
}

func (d *destinationPicker) MoveToTop() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		d.destList.Select(0)
	}
}

func (d *destinationPicker) MoveToBottom() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		d.destList.Select(len(d.filteredDest) - 1)
	}
}

func (d *destinationPicker) ClearSearch() {
	if d.searchEntry != nil {
		d.searchEntry.SetText("")
	}
}

func (d *destinationPicker) AppendToSearch(c string) {
	if d.searchEntry != nil {
		d.searchEntry.SetText(d.searchEntry.Text + c)
	}
}

func (d *destinationPicker) BackspaceSearch() {
	if d.searchEntry != nil {
		t := d.searchEntry.Text
		if len(t) > 0 {
			d.searchEntry.SetText(trimLastRune(t))
		}
	}
}

func (d *destinationPicker) GetSearchText() string {
	if d.searchEntry != nil {
		return d.searchEntry.Text
	}
	return ""
}

func (d *destinationPicker) CopySelectedPathToSearch() {
	if d.searchEntry != nil && d.selectedPath != "" {
		d.searchEntry.SetText(d.selectedPath)
	}
}

func (d *destinationPicker) ScrollSelectedRight() {
	d.scrollRight = true
	d.applyHorizontalScroll()
}

func (d *destinationPicker) ResetHorizontalScroll() {
	d.scrollRight = false
	if d.destScroll != nil {
		d.destScroll.ResetHorizontalScroll()
	}
}

func (d *destinationPicker) applyHorizontalScroll() {
	if !d.scrollRight || d.destScroll == nil || d.selectedPath == "" {
		return
	}
	d.destScroll.ScrollPathRight(d.selectedPath)
}

func (d *destinationPicker) destinationTextColor(path string) color.Color {
	if d.openDest[path] {
		themeProvider := currentThemeColorProvider()
		if themeProvider != nil {
			return themeProvider.GetCustomColor(customtheme.ColorCopyMoveOpenDestination)
		}
	}
	return currentAppThemeColor(fynetheme.ColorNameForeground)
}

func (d *destinationPicker) selectFilteredPath(path string) bool {
	for i, candidate := range d.filteredDest {
		if candidate.Path == path {
			d.destList.Select(widget.ListItemID(i))
			return true
		}
	}
	return false
}

func (d *destinationPicker) updateDestinationEmptyState() {
	if d.destScroll == nil || d.destEmpty == nil {
		return
	}
	if len(d.filteredDest) == 0 {
		d.destScroll.Hide()
		d.destEmpty.Show()
	} else {
		d.destEmpty.Hide()
		d.destScroll.Show()
	}
}

func destinationOpenMap(candidates []DestinationCandidate) map[string]bool {
	result := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.OpenInWindow {
			result[candidate.Path] = true
		}
	}
	return result
}
