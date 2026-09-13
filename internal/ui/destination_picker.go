package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/search"
	customtheme "nmf/internal/theme"
)

// DestinationCandidate describes a destination and where it came from.
type DestinationCandidate struct {
	Path         string
	OpenInWindow bool
}

// destinationPicker owns filtering, selection, and scrolling shared by the
// transfer and compare dialogs. Each dialog retains its own close and
// key-handler lifecycle.
type destinationPicker struct {
	searchEntry           *CustomSearchEntry
	destList              *widget.List
	filteredDest, allDest []DestinationCandidate
	openDest              map[string]bool
	dataBinding           binding.StringList
	selectedPath          string
	selectedIdx           int
	matchers              *search.Provider
	destScroll            *dialogListScroller
	destEmpty             *widget.Label
	scrollRight           bool
	onPathChanged         func(string)
}

func (d *destinationPicker) destinationArea(width, height float32) fyne.CanvasObject {
	size := metricsSize(width, height)
	d.destScroll = newDialogListScroller(d.destList, dialogDestinationTextWidth(d.allDest, width), width, height)
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
	d.dataBinding = binding.NewStringList()
	d.destList = widget.NewListWithData(
		d.dataBinding,
		func() fyne.CanvasObject {
			text := canvas.NewText("", currentAppThemeColor(fynetheme.ColorNameForeground))
			text.TextStyle = fyne.TextStyle{Monospace: true}
			text.TextSize = fynetheme.TextSize()
			return text
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			str, _ := item.(binding.String).Get()
			if text, ok := obj.(*canvas.Text); ok {
				text.Text = str
				text.TextSize = fynetheme.TextSize()
				text.Color = d.destinationTextColor(str)
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
}

// updateFiltered updates destination list
func (d *destinationPicker) updateFiltered(q string) {
	if q == "" {
		d.filteredDest = d.allDest
	} else {
		matcher := d.matchers.Build(q)
		d.filteredDest = d.filteredDest[:0:0]
		for _, p := range d.allDest {
			if matcher.Match(p.Path) {
				d.filteredDest = append(d.filteredDest, p)
			}
		}
	}
	d.dataBinding.Set(destinationPaths(d.filteredDest))
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

// SetDestinations replaces destination candidates while preserving the current search.
func (d *destinationPicker) SetDestinations(candidates []DestinationCandidate, preferredPath string) {
	previousPath := d.selectedPath
	query := d.GetSearchText()
	d.allDest = append([]DestinationCandidate(nil), candidates...)
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

func destinationPaths(candidates []DestinationCandidate) []string {
	paths := make([]string, len(candidates))
	for i, candidate := range candidates {
		paths[i] = candidate.Path
	}
	return paths
}

func dialogDestinationTextWidth(candidates []DestinationCandidate, minimum float32) float32 {
	return dialogTextWidth(destinationPaths(candidates), minimum)
}
