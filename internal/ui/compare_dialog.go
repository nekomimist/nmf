package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/filecompare"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
	customtheme "nmf/internal/theme"
)

// CompareResult describes accepted compare dialog choices.
type CompareResult struct {
	Destination string
	Method      filecompare.Method
}

// CompareDialog presents compare options and a destination directory picker.
type CompareDialog struct {
	sourcePath   string
	sourceCount  int
	searchEntry  *CustomSearchEntry
	destList     *widget.List
	filteredDest []DestinationCandidate
	allDest      []DestinationCandidate
	openDest     map[string]bool
	dataBinding  binding.StringList
	selectedPath string
	selectedIdx  int
	methodRadio  *widget.RadioGroup

	debugPrint  func(format string, args ...interface{})
	keyManager  *keymanager.KeyManager
	kmToken     keymanager.HandlerToken
	matchers    *search.Provider
	parent      fyne.Window
	dialog      dialog.Dialog
	sink        *KeySink
	closed      bool
	destScroll  *dialogListScroller
	scrollRight bool

	onAccept      func(CompareResult)
	onPathChanged func(string)
}

// NewCompareDialog creates a new directory compare dialog.
func NewCompareDialog(
	sourcePath string,
	sourceCount int,
	destCandidates []DestinationCandidate,
	km *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
	matchers ...*search.Provider,
) *CompareDialog {
	d := &CompareDialog{
		sourcePath:  sourcePath,
		sourceCount: sourceCount,
		allDest:     destCandidates,
		openDest:    destinationOpenMap(destCandidates),
		keyManager:  km,
		debugPrint:  debugPrint,
	}
	if len(matchers) > 0 {
		d.matchers = matchers[0]
	}
	if d.matchers == nil {
		d.matchers = search.NewPlainProvider()
	}
	d.createWidgets()
	d.updateFiltered("")
	return d
}

func (d *CompareDialog) createWidgets() {
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
			if d.parent != nil && d.sink != nil {
				d.parent.Canvas().Focus(d.sink)
			}
		}
	}

	d.methodRadio = widget.NewRadioGroup(compareMethodLabels(), func(string) {
		d.focusSink()
	})
	d.methodRadio.Required = true
	d.methodRadio.SetSelected(compareMethodLabel(filecompare.MissingOrNewer))
}

// ShowDialog renders and shows the compare dialog.
func (d *CompareDialog) ShowDialog(parent fyne.Window, onAccept func(CompareResult)) {
	d.parent = parent
	d.onAccept = onAccept
	dialogWidth := responsiveDialogWidth(parent, compareDialogWidth)
	listSize := metricsSize(dialogWidth, compareDialogListHeight)

	header := widget.NewLabel(fmt.Sprintf("Compare %d file(s)", d.sourceCount))
	header.TextStyle.Bold = true
	fromLabel := widget.NewLabel("From: " + compactComparePath(d.sourcePath, compareSourcePathMaxRunesForWidth(dialogWidth)))
	fromLabel.TextStyle.Monospace = true
	fromLabel.Truncation = fyne.TextTruncateEllipsis
	fromLine := container.NewGridWrap(fyne.NewSize(dialogWidth, fromLabel.MinSize().Height), fromLabel)
	headerBox := container.NewVBox(header, fromLine)

	methodLabel := widget.NewLabel("Mark files where:")
	methodBox := container.NewVBox(methodLabel, d.methodRadio)

	searchLabel := widget.NewLabel("Destination:")
	searchSection := container.NewBorder(nil, nil, searchLabel, nil, d.searchEntry)
	destScroll := newDialogListScroller(d.destList, dialogDestinationTextWidth(d.allDest, dialogWidth), dialogWidth, compareDialogListHeight)
	d.destScroll = destScroll
	empty := widget.NewLabel("No matching destinations")
	empty.Alignment = fyne.TextAlignCenter
	empty.Hide()
	fixed := container.NewWithoutLayout(destScroll, empty)
	fixed.Resize(listSize)
	destScroll.Resize(listSize)
	destScroll.Move(fyne.NewPos(0, 0))
	empty.Resize(listSize)
	empty.Move(fyne.NewPos(0, 0))
	d.searchEntry.OnChanged = func(q string) {
		d.updateFiltered(q)
		if len(d.filteredDest) == 0 {
			destScroll.Hide()
			empty.Show()
		} else {
			empty.Hide()
			destScroll.Show()
		}
	}

	content := container.NewVBox(
		headerBox,
		widget.NewSeparator(),
		methodBox,
		widget.NewSeparator(),
		searchSection,
		fixed,
		dialogButtonBar(dialogCancelButton("Cancel", d.CancelDialog), dialogConfirmButton("Compare", d.AcceptSelection)),
	)

	handler := keymanager.NewCompareDialogKeyHandler(d, d.debugPrint)
	d.kmToken = d.keyManager.PushHandler(handler)
	d.sink = NewKeySink(content, d.keyManager, WithTabCapture(true))
	d.searchEntry.SetFocusRedirect(parent, d.sink)

	d.dialog = dialog.NewCustomWithoutButtons("Compare Directories", d.sink, parent)
	d.dialog.Show()
	if d.parent != nil && d.sink != nil {
		d.parent.Canvas().Focus(d.sink)
		d.searchEntry.RefreshIMEAnchor()
	}
}

func (d *CompareDialog) focusSink() {
	if d.parent != nil && d.sink != nil {
		d.parent.Canvas().Focus(d.sink)
	}
}

func (d *CompareDialog) updateFiltered(q string) {
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
}

// SetOnSelectedPathChanged sets a callback for destination selection changes.
func (d *CompareDialog) SetOnSelectedPathChanged(callback func(string)) {
	d.onPathChanged = callback
	d.notifySelectedPathChanged()
}

func (d *CompareDialog) notifySelectedPathChanged() {
	if d.onPathChanged != nil {
		d.onPathChanged(d.selectedPath)
	}
}

func (d *CompareDialog) MoveUp() {
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

func (d *CompareDialog) MoveDown() {
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

func (d *CompareDialog) MoveToTop() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		d.destList.Select(0)
	}
}

func (d *CompareDialog) MoveToBottom() {
	if d.destList != nil && len(d.filteredDest) > 0 {
		d.destList.Select(len(d.filteredDest) - 1)
	}
}

func (d *CompareDialog) ClearSearch() {
	if d.searchEntry != nil {
		d.searchEntry.SetText("")
	}
}

func (d *CompareDialog) AppendToSearch(c string) {
	if d.searchEntry != nil {
		d.searchEntry.SetText(d.searchEntry.Text + c)
	}
}

func (d *CompareDialog) BackspaceSearch() {
	if d.searchEntry != nil {
		t := d.searchEntry.Text
		if len(t) > 0 {
			d.searchEntry.SetText(trimLastRune(t))
		}
	}
}

func (d *CompareDialog) CopySelectedPathToSearch() {
	if d.searchEntry != nil && d.selectedPath != "" {
		d.searchEntry.SetText(d.selectedPath)
	}
}

func (d *CompareDialog) SelectCurrentItem() {
	d.debugPrint("CompareDialog: Select current dest: %s", d.selectedPath)
}

func (d *CompareDialog) ScrollSelectedRight() {
	d.scrollRight = true
	d.applyHorizontalScroll()
}

func (d *CompareDialog) ResetHorizontalScroll() {
	d.scrollRight = false
	if d.destScroll != nil {
		d.destScroll.ResetHorizontalScroll()
	}
}

func (d *CompareDialog) applyHorizontalScroll() {
	if !d.scrollRight || d.destScroll == nil || d.selectedPath == "" {
		return
	}
	d.destScroll.ScrollPathRight(d.selectedPath)
}

func (d *CompareDialog) NextMethod() {
	d.moveMethod(1)
}

func (d *CompareDialog) PreviousMethod() {
	d.moveMethod(-1)
}

func (d *CompareDialog) moveMethod(delta int) {
	if d.methodRadio == nil {
		return
	}
	labels := compareMethodLabels()
	current := 0
	for i, label := range labels {
		if label == d.methodRadio.Selected {
			current = i
			break
		}
	}
	next := current + delta
	if next < 0 {
		next = 0
	}
	if next >= len(labels) {
		next = len(labels) - 1
	}
	d.methodRadio.SetSelected(labels[next])
}

func (d *CompareDialog) SelectMissingOrNewer() {
	d.selectMethod(filecompare.MissingOrNewer)
}

func (d *CompareDialog) SelectMissing() {
	d.selectMethod(filecompare.Missing)
}

func (d *CompareDialog) SelectNewer() {
	d.selectMethod(filecompare.Newer)
}

func (d *CompareDialog) SelectSizeEqual() {
	d.selectMethod(filecompare.SizeEqual)
}

func (d *CompareDialog) SelectSizeTimeEqual() {
	d.selectMethod(filecompare.SizeTimeEqual)
}

func (d *CompareDialog) SelectSizeContentEqual() {
	d.selectMethod(filecompare.SizeContentEqual)
}

func (d *CompareDialog) selectMethod(method filecompare.Method) {
	if d.methodRadio == nil {
		return
	}
	d.methodRadio.SetSelected(compareMethodLabel(method))
	d.focusSink()
}

func (d *CompareDialog) AcceptSelection() {
	d.accept(false)
}

func (d *CompareDialog) AcceptDirectPath() {
	d.accept(true)
}

func (d *CompareDialog) accept(direct bool) {
	if d.closed {
		return
	}
	d.closed = true

	acceptedPath := ""
	search := d.GetSearchText()
	if search != "" && (direct || len(d.filteredDest) == 0) {
		if resolvedPath, ok := d.resolveDirectoryPath(search); ok {
			d.debugPrint("CompareDialog: direct path accept: %s", resolvedPath)
			acceptedPath = resolvedPath
		} else if d.selectedPath != "" {
			acceptedPath = d.selectedPath
		}
	} else if d.selectedPath != "" {
		acceptedPath = d.selectedPath
	}
	method := d.selectedMethod()
	deferDialogClose(d.keyManager, "compare.accept", func() {
		d.notifyDialogClosed()
		d.keyManager.RemoveHandler(d.kmToken)
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink, d.searchEntry)
		if d.onAccept != nil && acceptedPath != "" {
			d.onAccept(CompareResult{Destination: acceptedPath, Method: method})
		}
	})
}

func (d *CompareDialog) GetSearchText() string {
	if d.searchEntry != nil {
		return d.searchEntry.Text
	}
	return ""
}

func (d *CompareDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.closed = true
	deferDialogClose(d.keyManager, "compare.cancel", func() {
		d.notifyDialogClosed()
		d.keyManager.RemoveHandler(d.kmToken)
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink, d.searchEntry)
	})
}

func (d *CompareDialog) notifyDialogClosed() {
	if d.onPathChanged != nil {
		d.onPathChanged("")
	}
}

func (d *CompareDialog) resolveDirectoryPath(p string) (string, bool) {
	resolved, _, err := fileinfo.ResolveAccessibleDirectoryPath(p)
	if err != nil {
		d.debugPrint("CompareDialog: Path is not accessible: '%s' (%v)", p, err)
		return "", false
	}
	return resolved, true
}

func (d *CompareDialog) selectedMethod() filecompare.Method {
	switch d.methodRadio.Selected {
	case compareMethodLabel(filecompare.Missing):
		return filecompare.Missing
	case compareMethodLabel(filecompare.Newer):
		return filecompare.Newer
	case compareMethodLabel(filecompare.SizeEqual):
		return filecompare.SizeEqual
	case compareMethodLabel(filecompare.SizeTimeEqual):
		return filecompare.SizeTimeEqual
	case compareMethodLabel(filecompare.SizeContentEqual):
		return filecompare.SizeContentEqual
	default:
		return filecompare.MissingOrNewer
	}
}

func (d *CompareDialog) destinationTextColor(path string) color.Color {
	if d.openDest[path] {
		themeProvider := currentThemeColorProvider()
		if themeProvider != nil {
			return themeProvider.GetCustomColor(customtheme.ColorCopyMoveOpenDestination)
		}
	}
	return currentAppThemeColor(fynetheme.ColorNameForeground)
}

func compareMethodLabels() []string {
	return []string{
		compareMethodLabel(filecompare.MissingOrNewer),
		compareMethodLabel(filecompare.Missing),
		compareMethodLabel(filecompare.Newer),
		compareMethodLabel(filecompare.SizeEqual),
		compareMethodLabel(filecompare.SizeTimeEqual),
		compareMethodLabel(filecompare.SizeContentEqual),
	}
}

func compareMethodLabel(method filecompare.Method) string {
	switch method {
	case filecompare.Missing:
		return "Missing in destination (Alt+M)"
	case filecompare.Newer:
		return "Newer than destination (Alt+N)"
	case filecompare.SizeEqual:
		return "File size matches (Alt+S)"
	case filecompare.SizeTimeEqual:
		return "File size and timestamp match (Alt+T)"
	case filecompare.SizeContentEqual:
		return "File size and content match (Alt+C)"
	default:
		return "Missing in destination or newer (Alt+U)"
	}
}

func compactComparePath(p string, maxRunes int) string {
	runes := []rune(p)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return p
	}

	marker := compactPathMarker(p)
	markerRunes := []rune(marker)
	if maxRunes <= len(markerRunes)+2 {
		return string(runes[len(runes)-maxRunes:])
	}

	available := maxRunes - len(markerRunes)
	prefixLen := available / 2
	suffixLen := available - prefixLen
	if prefixLen < 1 {
		prefixLen = 1
	}
	if suffixLen < 1 {
		suffixLen = 1
	}
	if prefixLen+suffixLen > len(runes) {
		return p
	}
	return string(runes[:prefixLen]) + marker + string(runes[len(runes)-suffixLen:])
}

func compareSourcePathMaxRunesForWidth(width float32) int {
	charWidth := fyne.MeasureText("M", fynetheme.TextSize(), fyne.TextStyle{Monospace: true}).Width
	if charWidth <= 0 {
		return compareSourcePathMaxRunes
	}
	maxRunes := int((width - fyne.MeasureText("From: ", fynetheme.TextSize(), fyne.TextStyle{}).Width) / charWidth)
	if maxRunes < compareSourcePathMaxRunes {
		return compareSourcePathMaxRunes
	}
	return maxRunes
}

func compactPathMarker(p string) string {
	switch {
	case strings.Contains(p, "/"):
		return "/.../"
	case strings.Contains(p, `\`):
		return `\...\`
	default:
		return "..."
	}
}
