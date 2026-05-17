package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

const (
	fileViewerMinWidth    float32 = 900
	fileViewerMinHeight   float32 = 760
	fileViewerSearchWidth float32 = 260
	fileViewerLineWidth   float32 = 90
)

type FileViewerDialog struct {
	preview *fileinfo.PreviewFile
	km      *keymanager.KeyManager
	parent  fyne.Window
	dialog  dialog.Dialog

	textEntry *ReadOnlyEntry
	hexEntry  *ReadOnlyEntry
	search    *widget.Entry
	jump      *widget.Entry
	status    *widget.Label
	lineLabel *widget.Label
	tabs      *container.AppTabs

	activeName string
	lastQuery  string
	lastIndex  int
	closed     bool
	handlerSet bool
}

func NewFileViewerDialog(preview *fileinfo.PreviewFile, km ...*keymanager.KeyManager) *FileViewerDialog {
	var keyManager *keymanager.KeyManager
	if len(km) > 0 {
		keyManager = km[0]
	}
	return &FileViewerDialog{
		preview:    preview,
		km:         keyManager,
		activeName: "Text",
		lastIndex:  -1,
	}
}

func (d *FileViewerDialog) ShowDialog(parent fyne.Window) {
	d.parent = parent
	d.textEntry = NewReadOnlyEntry(d.preview.Text, d.CancelDialog, d.handleEntryKey, d.handleEntryRune)
	d.hexEntry = NewReadOnlyEntry(fileinfo.FormatHexDump(d.preview.Data), d.CancelDialog, d.handleEntryKey, d.handleEntryRune)
	d.textEntry.OnCursorChanged = d.updateLineDisplay
	d.hexEntry.OnCursorChanged = d.updateLineDisplay
	d.textEntry.SetScrollHandler(d.handleEntryScroll)
	d.hexEntry.SetScrollHandler(d.handleEntryScroll)
	d.search = widget.NewEntry()
	d.search.SetPlaceHolder("Search")
	d.search.OnSubmitted = func(_ string) { d.findNext() }
	d.jump = widget.NewEntry()
	d.jump.SetPlaceHolder("Line")
	d.jump.OnSubmitted = func(_ string) { d.jumpToLine() }
	d.status = widget.NewLabel(d.statusText())
	d.status.Truncation = fyne.TextTruncateClip
	d.lineLabel = widget.NewLabel("")
	d.lineLabel.TextStyle = fyne.TextStyle{Monospace: true}

	d.tabs = container.NewAppTabs(
		container.NewTabItem("Text", d.textEntry),
		container.NewTabItem("Markdown", container.NewScroll(d.markdownView())),
		container.NewTabItem("Hex", d.hexEntry),
	)
	d.tabs.OnSelected = func(item *container.TabItem) {
		d.activeName = item.Text
		d.lastIndex = -1
		d.updateLineDisplay()
	}
	if d.preview.Binary {
		d.tabs.SelectIndex(2)
		d.activeName = "Hex"
	} else if d.preview.Markdown {
		d.tabs.SelectIndex(1)
		d.activeName = "Markdown"
	}

	toolbar := container.NewBorder(nil, nil, nil, container.NewHBox(
		widget.NewButtonWithIcon("", theme.ContentCopyIcon(), d.copySelection),
		widget.NewButtonWithIcon("", theme.CancelIcon(), d.CancelDialog),
	), container.NewHBox(
		container.NewGridWrap(fyne.NewSize(fileViewerSearchWidth, d.search.MinSize().Height), d.search),
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), d.findPrevious),
		widget.NewButtonWithIcon("", theme.NavigateNextIcon(), d.findNext),
		widget.NewSeparator(),
		container.NewGridWrap(fyne.NewSize(fileViewerLineWidth, d.jump.MinSize().Height), d.jump),
		widget.NewButtonWithIcon("", theme.ConfirmIcon(), d.jumpToLine),
	))

	content := container.NewBorder(
		container.NewVBox(widget.NewLabel(filepath.Base(d.preview.Path)), d.status, container.NewBorder(nil, nil, nil, d.lineLabel, toolbar)),
		nil,
		nil,
		nil,
		d.tabs,
	)
	if d.km != nil {
		d.km.PushHandler(keymanager.NewFileViewerKeyHandler(d))
		d.handlerSet = true
	}

	d.dialog = dialog.NewCustomWithoutButtons("Viewer", content, parent)
	d.dialog.SetOnClosed(func() {
		d.CancelDialog()
	})
	d.dialog.Show()
	d.dialog.Resize(fileViewerDialogSize(parent))
	d.updateLineDisplay()
	d.focusActiveEntry()
}

func fileViewerDialogSize(parent fyne.Window) fyne.Size {
	if parent == nil || parent.Canvas() == nil {
		return fyne.NewSize(fileViewerMinWidth, fileViewerMinHeight)
	}
	canvasSize := parent.Canvas().Size()
	width := canvasSize.Width * 0.96
	height := canvasSize.Height * 0.88
	if width < fileViewerMinWidth && canvasSize.Width >= fileViewerMinWidth {
		width = fileViewerMinWidth
	}
	if height < fileViewerMinHeight && canvasSize.Height >= fileViewerMinHeight {
		height = fileViewerMinHeight
	}
	if canvasSize.Width > 64 && width > canvasSize.Width-32 {
		width = canvasSize.Width - 32
	}
	if canvasSize.Height > 96 && height > canvasSize.Height-48 {
		height = canvasSize.Height - 48
	}
	return fyne.NewSize(width, height)
}

func (d *FileViewerDialog) markdownView() fyne.CanvasObject {
	rich := widget.NewRichTextFromMarkdown(d.preview.Text)
	rich.Wrapping = fyne.TextWrapWord
	return rich
}

func (d *FileViewerDialog) statusText() string {
	parts := []string{
		fmt.Sprintf("encoding=%s", d.preview.Encoding),
		fmt.Sprintf("read=%s", fileinfo.FormatFileSize(int64(len(d.preview.Data)))),
	}
	if d.preview.SizeKnown {
		parts = append(parts, fmt.Sprintf("size=%s", fileinfo.FormatFileSize(d.preview.Size)))
	}
	if d.preview.Truncated {
		parts = append(parts, "truncated=1MiB")
	}
	if d.preview.Binary {
		parts = append(parts, "binary")
	}
	return strings.Join(parts, "  ")
}

func (d *FileViewerDialog) CancelDialog() {
	if d.closed {
		return
	}
	d.closed = true
	if d.handlerSet && d.km != nil {
		d.km.PopHandler()
		d.handlerSet = false
	}
	if d.dialog != nil {
		d.dialog.Hide()
	}
	if d.parent != nil {
		d.parent.Canvas().Unfocus()
	}
}

func (d *FileViewerDialog) CloseViewer() {
	d.CancelDialog()
}

func (d *FileViewerDialog) activeEntry() *ReadOnlyEntry {
	switch d.activeName {
	case "Hex":
		return d.hexEntry
	default:
		return d.textEntry
	}
}

func (d *FileViewerDialog) focusActiveEntry() {
	if d.parent == nil {
		return
	}
	entry := d.activeEntry()
	if entry != nil {
		d.parent.Canvas().Focus(entry)
	}
}

func (d *FileViewerDialog) handleEntryKey(ev *fyne.KeyEvent) bool {
	if ev == nil {
		return false
	}
	switch ev.Name {
	case fyne.KeyEscape:
		d.CloseViewer()
	case fyne.KeySpace:
		d.ViewerPageDown()
	case fyne.KeyPageDown:
		d.ViewerPageDown()
	case fyne.KeyPageUp:
		d.ViewerPageUp()
	case fyne.KeyHome:
		d.ViewerHome()
	case fyne.KeyEnd:
		d.ViewerEnd()
	default:
		return false
	}
	return true
}

func (d *FileViewerDialog) handleEntryRune(r rune) bool {
	switch r {
	case 'q':
		d.CloseViewer()
	case 'j':
		d.ViewerLineDown()
	case 'k':
		d.ViewerLineUp()
	case 'f':
		d.ViewerPageDown()
	case 'b':
		d.ViewerPageUp()
	case 'g':
		d.ViewerHome()
	case 'G':
		d.ViewerEnd()
	case 'n':
		d.ViewerSearchNext()
	case 'N':
		d.ViewerSearchPrevious()
	case '/':
		d.ViewerFocusSearch()
	case ':':
		d.ViewerFocusLine()
	default:
		return false
	}
	return true
}

func (d *FileViewerDialog) handleEntryScroll(deltaY float32) bool {
	if deltaY < 0 {
		d.ViewerLineDown()
	} else if deltaY > 0 {
		d.ViewerLineUp()
	} else {
		return false
	}
	return true
}

func (d *FileViewerDialog) copySelection() {
	entry := d.activeEntry()
	if entry == nil {
		return
	}
	text := entry.SelectedText()
	if text == "" {
		d.status.SetText(d.statusText() + "  copy=no-selection")
		return
	}
	app := fyne.CurrentApp()
	if app == nil || app.Clipboard() == nil {
		d.status.SetText(d.statusText() + "  copy=unavailable")
		return
	}
	app.Clipboard().SetContent(text)
	d.status.SetText(d.statusText() + fmt.Sprintf("  copied=%d", len(text)))
	d.focusActiveEntry()
}

func (d *FileViewerDialog) findNext() {
	d.find(1)
}

func (d *FileViewerDialog) findPrevious() {
	d.find(-1)
}

func (d *FileViewerDialog) find(direction int) {
	query := strings.TrimSpace(d.search.Text)
	if query == "" {
		return
	}
	if d.activeName == "Markdown" {
		d.tabs.SelectIndex(0)
		d.activeName = "Text"
	}
	entry := d.activeEntry()
	if entry == nil || entry.Text == "" {
		return
	}
	haystack := strings.ToLower(entry.Text)
	needle := strings.ToLower(query)
	if query != d.lastQuery {
		d.lastQuery = query
		d.lastIndex = -1
	}

	next := -1
	if direction >= 0 {
		start := d.lastIndex + 1
		if start < 0 || start >= len(haystack) {
			start = 0
		}
		if idx := strings.Index(haystack[start:], needle); idx >= 0 {
			next = start + idx
		} else if idx := strings.Index(haystack, needle); idx >= 0 {
			next = idx
		}
	} else {
		start := d.lastIndex
		if start < 0 || start > len(haystack) {
			start = len(haystack)
		}
		if idx := strings.LastIndex(haystack[:start], needle); idx >= 0 {
			next = idx
		} else {
			next = strings.LastIndex(haystack, needle)
		}
	}
	if next < 0 {
		d.setStatusSuffix("search=no-match")
		d.focusActiveEntry()
		return
	}
	d.lastIndex = next
	d.moveEntryToByteOffset(entry, next)
	d.setStatusSuffix(fmt.Sprintf("search=%d", next))
	d.focusActiveEntry()
}

func (d *FileViewerDialog) jumpToLine() {
	line, err := strconv.Atoi(strings.TrimSpace(d.jump.Text))
	if err != nil || line <= 0 {
		return
	}
	if d.activeName == "Markdown" {
		d.tabs.SelectIndex(0)
		d.activeName = "Text"
	}
	entry := d.activeEntry()
	if entry == nil {
		return
	}
	maxLine := 1 + strings.Count(entry.Text, "\n")
	if line > maxLine {
		line = maxLine
	}
	entry.CursorRow = line - 1
	entry.CursorColumn = 0
	entry.Refresh()
	d.updateLineDisplay()
	d.setStatusSuffix(fmt.Sprintf("line=%d", line))
	d.focusActiveEntry()
}

func (d *FileViewerDialog) moveEntryToByteOffset(entry *ReadOnlyEntry, offset int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(entry.Text) {
		offset = len(entry.Text)
	}
	row, col := rowColForPrefix(entry.Text[:offset])
	entry.CursorRow = row
	entry.CursorColumn = col
	entry.Refresh()
	d.updateLineDisplay()
}

func rowColForPrefix(text string) (int, int) {
	row := 0
	col := 0
	for _, r := range text {
		if r == '\n' {
			row++
			col = 0
			continue
		}
		col++
	}
	return row, col
}

func (d *FileViewerDialog) ViewerLineDown() {
	d.moveCursorRows(1)
}

func (d *FileViewerDialog) ViewerLineUp() {
	d.moveCursorRows(-1)
}

func (d *FileViewerDialog) ViewerPageDown() {
	d.moveCursorRows(20)
}

func (d *FileViewerDialog) ViewerPageUp() {
	d.moveCursorRows(-20)
}

func (d *FileViewerDialog) ViewerHome() {
	entry := d.activeEntry()
	if entry == nil {
		return
	}
	entry.CursorRow = 0
	entry.CursorColumn = 0
	entry.Refresh()
	d.updateLineDisplay()
	d.focusActiveEntry()
}

func (d *FileViewerDialog) ViewerEnd() {
	entry := d.activeEntry()
	if entry == nil {
		return
	}
	entry.CursorRow = lineCount(entry.Text) - 1
	if entry.CursorRow < 0 {
		entry.CursorRow = 0
	}
	entry.CursorColumn = 0
	entry.Refresh()
	d.updateLineDisplay()
	d.focusActiveEntry()
}

func (d *FileViewerDialog) ViewerSearchNext() {
	d.findNext()
}

func (d *FileViewerDialog) ViewerSearchPrevious() {
	d.findPrevious()
}

func (d *FileViewerDialog) ViewerFocusSearch() {
	if d.parent != nil && d.search != nil {
		d.parent.Canvas().Focus(d.search)
	}
}

func (d *FileViewerDialog) ViewerFocusLine() {
	if d.parent != nil && d.jump != nil {
		d.parent.Canvas().Focus(d.jump)
	}
}

func (d *FileViewerDialog) moveCursorRows(delta int) {
	entry := d.activeEntry()
	if entry == nil {
		return
	}
	maxRow := lineCount(entry.Text) - 1
	next := entry.CursorRow + delta
	if next < 0 {
		next = 0
	}
	if next > maxRow {
		next = maxRow
	}
	entry.CursorRow = next
	entry.CursorColumn = 0
	entry.Refresh()
	d.updateLineDisplay()
	d.focusActiveEntry()
}

func (d *FileViewerDialog) updateLineDisplay() {
	if d.lineLabel == nil {
		return
	}
	entry := d.activeEntry()
	if entry == nil {
		d.lineLabel.SetText("")
		return
	}
	line := entry.CursorRow + 1
	total := lineCount(entry.Text)
	if total < 1 {
		total = 1
	}
	if line < 1 {
		line = 1
	}
	if line > total {
		line = total
	}
	d.lineLabel.SetText(fmt.Sprintf("line=%d/%d", line, total))
}

func (d *FileViewerDialog) setStatusSuffix(suffix string) {
	d.status.SetText(d.statusText() + "  " + suffix)
}

func lineCount(text string) int {
	if text == "" {
		return 1
	}
	return 1 + strings.Count(text, "\n")
}
