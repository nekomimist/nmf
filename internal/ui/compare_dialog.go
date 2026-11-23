package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/filecompare"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
)

// CompareResult describes accepted compare dialog choices.
type CompareResult struct {
	Destination string
	Method      filecompare.Method
}

// CompareDialog presents compare options and a destination directory picker.
type CompareDialog struct {
	destinationPicker
	sourcePath           string
	sourceFileCount      int
	sourceDirectoryCount int
	methodRadio          *widget.RadioGroup
	compareButton        *widget.Button

	debugPrint func(format string, args ...interface{})
	keyManager *keymanager.KeyManager
	kmToken    keymanager.HandlerToken
	parent     fyne.Window
	dialog     dialog.Dialog
	sink       *KeySink
	closed     bool
	ownerFrame dialogHighlightState

	onAccept func(CompareResult)
}

// NewCompareDialog creates a new directory compare dialog.
func NewCompareDialog(
	sourcePath string,
	sourceFileCount, sourceDirectoryCount int,
	destCandidates []DestinationCandidate,
	km *keymanager.KeyManager,
	debugPrint func(format string, args ...interface{}),
	matchers ...*search.Provider,
) *CompareDialog {
	d := &CompareDialog{
		destinationPicker:    destinationPicker{allDest: destCandidates, openDest: destinationOpenMap(destCandidates)},
		sourcePath:           sourcePath,
		sourceFileCount:      sourceFileCount,
		sourceDirectoryCount: sourceDirectoryCount,
		keyManager:           km,
		debugPrint:           debugPrint,
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
	d.destinationPicker.createWidgets(func() {
		if d.parent != nil && d.sink != nil {
			d.parent.Canvas().Focus(d.sink)
		}
	})
	d.compareButton = dialogConfirmButton("Compare", d.AcceptSelection)
	d.methodRadio = widget.NewRadioGroup(compareMethodLabels(), func(string) {
		if d.hasSourceForMethod(d.selectedMethod()) {
			d.compareButton.Enable()
		} else {
			d.compareButton.Disable()
		}
		d.focusSink()
	})
	d.methodRadio.Required = true
	method := filecompare.MissingOrNewer
	if d.sourceFileCount == 0 && d.sourceDirectoryCount > 0 {
		method = filecompare.MissingSubdirectory
	}
	d.methodRadio.SetSelected(compareMethodLabel(method))
}

// ShowDialog renders and shows the compare dialog.
func (d *CompareDialog) ShowDialog(parent fyne.Window, onAccept func(CompareResult)) {
	d.parent = parent
	d.onAccept = onAccept
	dialogWidth := responsiveDialogWidth(parent, compareDialogWidth)

	header := widget.NewLabel(fmt.Sprintf("Files: %d, Subdirectories: %d", d.sourceFileCount, d.sourceDirectoryCount))
	header.TextStyle.Bold = true
	fromLabel := widget.NewLabel("From: " + compactComparePath(d.sourcePath, compareSourcePathMaxRunesForWidth(dialogWidth)))
	fromLabel.TextStyle.Monospace = true
	fromLabel.Truncation = fyne.TextTruncateEllipsis
	fromLine := container.NewGridWrap(fyne.NewSize(dialogWidth, fromLabel.MinSize().Height), fromLabel)
	headerBox := container.NewVBox(header, fromLine)

	methodLabel := widget.NewLabel("Mark entries where:")
	methodBox := container.NewVBox(methodLabel, d.methodRadio)

	searchSection := container.NewBorder(nil, nil, nil, historyOrderInline(d.orderRadio), d.searchEntry)
	fixed := d.destinationArea(dialogWidth, compareDialogListHeight)

	content := container.NewVBox(
		headerBox,
		widget.NewSeparator(),
		methodBox,
		widget.NewSeparator(),
		searchSection,
		fixed,
		dialogButtonBar(dialogCancelButton("Cancel", d.CancelDialog), d.compareButton),
	)

	handler := keymanager.NewCompareDialogKeyHandler(d, d.debugPrint)
	d.kmToken = d.keyManager.PushHandler(handler)
	d.sink = NewKeySink(content, d.keyManager, WithTabCapture(true))
	d.searchEntry.SetFocusRedirect(parent, d.sink)

	d.dialog = newModalDialogWithOptions("Compare Directories", d.sink, parent, modalDialogOptions{
		highlight: &d.ownerFrame,
	})
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

// SetOwnerHighlighted changes the accent frame around the owning dialog.
func (d *CompareDialog) SetOwnerHighlighted(highlighted bool) {
	d.ownerFrame.setHighlighted(highlighted)
}

func (d *CompareDialog) SelectCurrentItem() {
	d.debugPrint("CompareDialog: Select current dest: %s", d.selectedPath)
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
	labels := d.methodRadio.Options
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

func (d *CompareDialog) SelectMissingSubdirectory() {
	d.selectMethod(filecompare.MissingSubdirectory)
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
	method := d.selectedMethod()
	if d.closed || !d.hasSourceForMethod(method) {
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
	resolved, _, err := fileinfo.CanonicalDisplayPath(p)
	if err != nil {
		d.debugPrint("CompareDialog: Path is invalid: '%s' (%v)", p, err)
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
	case compareMethodLabel(filecompare.MissingSubdirectory):
		return filecompare.MissingSubdirectory
	default:
		return filecompare.MissingOrNewer
	}
}

func (d *CompareDialog) hasSourceForMethod(method filecompare.Method) bool {
	if method == filecompare.MissingSubdirectory {
		return d.sourceDirectoryCount > 0
	}
	return d.sourceFileCount > 0
}

func compareMethodLabels() []string {
	return []string{
		compareMethodLabel(filecompare.MissingOrNewer),
		compareMethodLabel(filecompare.Missing),
		compareMethodLabel(filecompare.Newer),
		compareMethodLabel(filecompare.SizeEqual),
		compareMethodLabel(filecompare.SizeTimeEqual),
		compareMethodLabel(filecompare.SizeContentEqual),
		compareMethodLabel(filecompare.MissingSubdirectory),
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
	case filecompare.MissingSubdirectory:
		return "Subdirectory missing in destination (Alt+D)"
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
