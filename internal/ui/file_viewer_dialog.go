package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	fileViewerTextLimit   int     = 64 << 10
	fileViewerHexLimit    int     = 64 << 10
	hexDumpFullLineBytes  int     = 79
)

type FileViewerDialog struct {
	preview *fileinfo.PreviewFile
	km      *keymanager.KeyManager
	parent  fyne.Window
	dialog  dialog.Dialog

	textGrid  *fileViewerTextGrid
	hexEntry  *ReadOnlyEntry
	search    *widget.Entry
	jump      *widget.Entry
	status    *widget.Label
	lineLabel *widget.Label
	tabs      *container.AppTabs

	textTab *container.TabItem
	hexTab  *container.TabItem

	activeName string
	lastQuery  string
	lastIndex  int
	closed     bool
	handlerSet bool
	debugPrint func(format string, args ...interface{})
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

func (d *FileViewerDialog) SetDebugPrint(debugPrint func(format string, args ...interface{})) {
	d.debugPrint = debugPrint
}

func (d *FileViewerDialog) ShowDialog(parent fyne.Window) {
	totalStart := time.Now()
	stepStart := totalStart
	d.parent = parent
	d.debug("FileViewer: dialog-start bytes=%d text_bytes=%d binary=%t markdown=%t",
		len(d.preview.Data), len(d.preview.Text), d.preview.Binary, d.preview.Markdown)

	text := viewerText(d.preview)
	d.debug("FileViewer: text-view elapsed=%s bytes=%d", time.Since(stepStart), len(text))
	stepStart = time.Now()
	d.textGrid = newFileViewerTextGrid(text, d.km, d.updateLineDisplay, d.debugPrint)
	d.debug("FileViewer: text-grid elapsed=%s bytes=%d", time.Since(stepStart), len(text))
	stepStart = time.Now()

	hexContent := fyne.CanvasObject(widget.NewLabel("Hex preview will load when selected."))
	if d.preview.Binary {
		hexContent = d.createHexEntry()
	}
	d.debug("FileViewer: hex-tab-content elapsed=%s loaded=%t", time.Since(stepStart), d.hexEntry != nil)
	stepStart = time.Now()

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
	d.debug("FileViewer: controls elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()

	d.textTab = container.NewTabItem("Text", d.textGrid)
	d.hexTab = container.NewTabItem("Hex", hexContent)
	d.tabs = container.NewAppTabs(d.textTab, container.NewTabItem("Markdown", container.NewScroll(d.markdownView())), d.hexTab)
	d.tabs.OnSelected = func(item *container.TabItem) {
		if item == d.hexTab {
			d.ensureHexEntry()
		}
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
	d.debug("FileViewer: tabs elapsed=%s active=%s", time.Since(stepStart), d.activeName)
	stepStart = time.Now()

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
	d.debug("FileViewer: layout elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()

	if d.km != nil {
		d.km.PushHandler(keymanager.NewFileViewerKeyHandler(d))
		d.handlerSet = true
	}
	d.debug("FileViewer: handler elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()

	d.dialog = dialog.NewCustomWithoutButtons("Viewer", content, parent)
	d.dialog.SetOnClosed(func() {
		d.CancelDialog()
	})
	d.debug("FileViewer: dialog-create elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()

	d.dialog.Show()
	d.debug("FileViewer: dialog-show elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()
	d.dialog.Resize(fileViewerDialogSize(parent))
	d.debug("FileViewer: dialog-resize elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()
	d.updateLineDisplay()
	d.focusActiveViewer()
	d.debug("FileViewer: dialog-focus elapsed=%s", time.Since(stepStart))
	d.debug("FileViewer: dialog-ready elapsed=%s", time.Since(totalStart))
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
	start := time.Now()
	if d.preview.Binary {
		d.debug("FileViewer: markdown-view elapsed=%s mode=binary-placeholder", time.Since(start))
		return widget.NewLabel("Binary file: markdown preview disabled. Use the Hex tab.")
	}
	rich := widget.NewRichTextFromMarkdown(viewerText(d.preview))
	rich.Wrapping = fyne.TextWrapWord
	d.debug("FileViewer: markdown-view elapsed=%s mode=markdown", time.Since(start))
	return rich
}

func (d *FileViewerDialog) configureViewerEntry(entry *ReadOnlyEntry) {
	entry.OnCursorChanged = d.updateLineDisplay
	entry.SetScrollHandler(d.handleEntryScroll)
}

func (d *FileViewerDialog) createHexEntry() *ReadOnlyEntry {
	stepStart := time.Now()
	hex := viewerHex(d.preview)
	d.debug("FileViewer: hex-view elapsed=%s bytes=%d", time.Since(stepStart), len(hex))
	stepStart = time.Now()
	entry := NewReadOnlyEntry(hex, d.CancelDialog, d.handleEntryKey, d.handleEntryRune)
	d.configureViewerEntry(entry)
	d.hexEntry = entry
	d.debug("FileViewer: hex-entry elapsed=%s bytes=%d", time.Since(stepStart), len(hex))
	return entry
}

func (d *FileViewerDialog) ensureHexEntry() {
	if d.hexEntry != nil || d.hexTab == nil {
		return
	}
	stepStart := time.Now()
	d.hexTab.Content = d.createHexEntry()
	if d.tabs != nil {
		d.tabs.Refresh()
	}
	d.debug("FileViewer: hex-lazy-load elapsed=%s", time.Since(stepStart))
}

func (d *FileViewerDialog) debug(format string, args ...interface{}) {
	if d.debugPrint != nil {
		d.debugPrint(format, args...)
	}
}

func viewerText(preview *fileinfo.PreviewFile) string {
	if preview == nil {
		return ""
	}
	if preview.Binary {
		return "Binary file: text preview disabled. Use the Hex tab."
	}
	text, truncated := truncateUTF8Bytes(preview.Text, fileViewerTextLimit)
	text = sanitizeViewerText(text)
	if truncated {
		text += fmt.Sprintf("\n\n[viewer text truncated at %s]", fileinfo.FormatFileSize(int64(fileViewerTextLimit)))
	}
	return text
}

func sanitizeViewerText(text string) string {
	var b strings.Builder
	changed := false
	for i, r := range text {
		if viewerPrintableRune(r) {
			if changed {
				b.WriteRune(r)
			}
			continue
		}
		if !changed {
			b.Grow(len(text))
			b.WriteString(text[:i])
			changed = true
		}
		writeEscapedRune(&b, r)
	}
	if !changed {
		return text
	}
	return b.String()
}

func viewerPrintableRune(r rune) bool {
	switch r {
	case '\n', '\r', '\t':
		return true
	default:
		return unicode.IsGraphic(r)
	}
}

func writeEscapedRune(b *strings.Builder, r rune) {
	if r <= 0xffff {
		fmt.Fprintf(b, "\\u%04X", r)
		return
	}
	fmt.Fprintf(b, "\\U%08X", r)
}

func viewerHex(preview *fileinfo.PreviewFile) string {
	if preview == nil {
		return ""
	}
	data := preview.Data
	truncated := false
	limit := hexDumpDataLimit(fileViewerHexLimit)
	if len(data) > limit {
		data = data[:limit]
		truncated = true
	}
	text := fileinfo.FormatHexDump(data)
	if truncated {
		text += fmt.Sprintf("\n[viewer hex truncated at %s of %s read]\n",
			fileinfo.FormatFileSize(int64(fileViewerHexLimit)),
			fileinfo.FormatFileSize(int64(len(preview.Data))))
	}
	return text
}

func hexDumpDataLimit(textLimit int) int {
	if textLimit <= 0 {
		return 0
	}
	lines := textLimit / hexDumpFullLineBytes
	if lines < 1 {
		lines = 1
	}
	return lines * 16
}

func truncateUTF8Bytes(text string, limit int) (string, bool) {
	if limit < 0 {
		limit = 0
	}
	if len(text) <= limit {
		return text, false
	}
	for limit > 0 && !utf8.ValidString(text[:limit]) {
		limit--
	}
	return text[:limit], true
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
	deferDialogClose(d.km, "viewer.close", func() {
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
	})
}

func (d *FileViewerDialog) CloseViewer() {
	d.CancelDialog()
}

func (d *FileViewerDialog) activeEntry() *ReadOnlyEntry {
	switch d.activeName {
	case "Hex":
		return d.hexEntry
	default:
		return nil
	}
}

func (d *FileViewerDialog) focusActiveViewer() {
	if d.parent == nil {
		return
	}
	if d.activeName == "Text" && d.textGrid != nil {
		d.parent.Canvas().Focus(d.textGrid)
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
	if d.activeName == "Text" {
		d.setStatusSuffix("copy=unsupported")
		d.focusActiveViewer()
		return
	}
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
	d.focusActiveViewer()
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
	if d.activeName == "Text" {
		d.setStatusSuffix("search=unsupported")
		d.focusActiveViewer()
		return
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
		d.focusActiveViewer()
		return
	}
	d.lastIndex = next
	d.moveEntryToByteOffset(entry, next)
	d.setStatusSuffix(fmt.Sprintf("search=%d", next))
	d.focusActiveViewer()
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
	if d.activeName == "Text" {
		if d.textGrid == nil {
			return
		}
		line = d.textGrid.JumpToLine(line)
		d.updateLineDisplay()
		d.setStatusSuffix(fmt.Sprintf("line=%d", line))
		d.focusActiveViewer()
		return
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
	d.focusActiveViewer()
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
	if d.activeName == "Text" && d.textGrid != nil {
		d.textGrid.PageDown()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
	d.moveCursorRows(20)
}

func (d *FileViewerDialog) ViewerPageUp() {
	if d.activeName == "Text" && d.textGrid != nil {
		d.textGrid.PageUp()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
	d.moveCursorRows(-20)
}

func (d *FileViewerDialog) ViewerHome() {
	if d.activeName == "Text" && d.textGrid != nil {
		d.textGrid.Home()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
	entry := d.activeEntry()
	if entry == nil {
		return
	}
	entry.CursorRow = 0
	entry.CursorColumn = 0
	entry.Refresh()
	d.updateLineDisplay()
	d.focusActiveViewer()
}

func (d *FileViewerDialog) ViewerEnd() {
	if d.activeName == "Text" && d.textGrid != nil {
		d.textGrid.End()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
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
	d.focusActiveViewer()
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
	if d.activeName == "Text" && d.textGrid != nil {
		d.textGrid.MoveRows(delta)
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
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
	d.focusActiveViewer()
}

func (d *FileViewerDialog) updateLineDisplay() {
	if d.lineLabel == nil {
		return
	}
	if d.activeName == "Text" && d.textGrid != nil {
		d.lineLabel.SetText(fmt.Sprintf("line=%d/%d", d.textGrid.CurrentLine(), d.textGrid.TotalLines()))
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
