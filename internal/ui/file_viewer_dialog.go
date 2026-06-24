package ui

import (
	"fmt"
	"html"
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
	"github.com/yuin/goldmark"
	goldmarkast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	tableast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

const (
	markdownTableTargetColumns int = 80
	markdownTableMinColumn     int = 8
)

const (
	viewerPaneAuto     = "auto"
	viewerPaneText     = "text"
	viewerPaneMarkdown = "markdown"
	viewerPaneHex      = "hex"
)

type FileViewerDialog struct {
	preview *fileinfo.PreviewFile
	km      *keymanager.KeyManager
	kmToken keymanager.HandlerToken
	parent  fyne.Window
	dialog  dialog.Dialog

	textGrid   *fileViewerTextGrid
	hexGrid    *fileViewerTextGrid
	mdGrid     *fileViewerTextGrid
	search     *IMEEntry
	jump       *IMEEntry
	status     *widget.Label
	lineLabel  *widget.Label
	wrapButton *widget.Button
	tabs       *container.AppTabs

	textTab *container.TabItem
	hexTab  *container.TabItem
	mdTab   *container.TabItem
	mdView  fyne.CanvasObject

	activeName  string
	closed      bool
	handlerSet  bool
	maxWidth    int
	maxHeight   int
	defaultPane string
	bindings    []config.KeyBindingEntry
	debugPrint  func(format string, args ...interface{})
}

func NewFileViewerDialog(preview *fileinfo.PreviewFile, km ...*keymanager.KeyManager) *FileViewerDialog {
	var keyManager *keymanager.KeyManager
	if len(km) > 0 {
		keyManager = km[0]
	}
	return &FileViewerDialog{
		preview:     preview,
		km:          keyManager,
		activeName:  "Text",
		defaultPane: viewerPaneAuto,
	}
}

func (d *FileViewerDialog) SetDebugPrint(debugPrint func(format string, args ...interface{})) {
	d.debugPrint = debugPrint
}

func (d *FileViewerDialog) SetKeyBindings(bindings []config.KeyBindingEntry) {
	d.bindings = bindings
}

func (d *FileViewerDialog) SetMaxSize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	d.maxWidth = width
	d.maxHeight = height
}

func (d *FileViewerDialog) SetDefaultPane(pane string) {
	if normalized := normalizeViewerPane(pane); normalized != "" {
		d.defaultPane = normalized
	}
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
	d.textGrid.SetCopyHandler(d.copySelection)
	d.debug("FileViewer: text-grid elapsed=%s bytes=%d", time.Since(stepStart), len(text))
	stepStart = time.Now()

	hexContent := fyne.CanvasObject(widget.NewLabel("Hex preview will load when selected."))
	if d.preview.Binary {
		hexContent = d.createHexGrid()
	}
	d.debug("FileViewer: hex-tab-content elapsed=%s loaded=%t", time.Since(stepStart), d.hexGrid != nil)
	stepStart = time.Now()

	d.search = NewIMEEntry(parent)
	d.search.SetPlaceHolder("Search")
	d.search.OnEscape = d.focusActiveViewer
	d.search.OnSubmitted = func(_ string) { d.findNext() }
	d.jump = NewIMEEntry(parent)
	d.jump.SetPlaceHolder("Line")
	d.jump.OnEscape = d.focusActiveViewer
	d.jump.OnSubmitted = func(_ string) { d.jumpToLine() }
	d.status = widget.NewLabel(d.statusText())
	d.status.Truncation = fyne.TextTruncateClip
	d.lineLabel = widget.NewLabel("")
	d.lineLabel.TextStyle = fyne.TextStyle{Monospace: true}
	d.debug("FileViewer: controls elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()

	d.textTab = container.NewTabItem("Text (t)", d.textGrid)
	d.hexTab = container.NewTabItem("Hex (x)", hexContent)
	d.mdTab = container.NewTabItem("Markdown (m)", widget.NewLabel("Markdown preview will load when selected."))
	d.tabs = container.NewAppTabs(d.textTab, d.mdTab, d.hexTab)
	d.tabs.OnSelected = func(item *container.TabItem) {
		if item == d.hexTab {
			d.ensureHexGrid()
		}
		if item == d.mdTab {
			d.ensureMarkdownView()
		}
		d.activeName = viewerPaneNameForTab(d, item)
		d.updateLineDisplay()
	}
	d.selectInitialTab()
	d.debug("FileViewer: tabs elapsed=%s active=%s", time.Since(stepStart), d.activeName)
	stepStart = time.Now()

	d.wrapButton = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), d.ViewerToggleWrap)
	toolbar := container.NewBorder(nil, nil, nil, container.NewHBox(
		d.wrapButton,
		widget.NewButtonWithIcon("", theme.ContentCopyIcon(), d.copySelection),
		widget.NewButtonWithIcon("", theme.CancelIcon(), d.CancelDialog),
	), container.NewHBox(
		container.NewGridWrap(fyne.NewSize(fileViewerSearchWidth, d.search.MinSize().Height), lineEditThemeOverride(d.search)),
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), d.findPrevious),
		widget.NewButtonWithIcon("", theme.NavigateNextIcon(), d.findNext),
		widget.NewSeparator(),
		container.NewGridWrap(fyne.NewSize(fileViewerLineWidth, d.jump.MinSize().Height), lineEditThemeOverride(d.jump)),
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
		d.kmToken = d.km.PushHandler(keymanager.NewFileViewerKeyHandler(d, d.bindings))
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
	d.dialog.Resize(fileViewerDialogSize(parent, d.maxWidth, d.maxHeight))
	d.debug("FileViewer: dialog-resize elapsed=%s", time.Since(stepStart))
	stepStart = time.Now()
	d.updateLineDisplay()
	d.focusActiveViewer()
	d.debug("FileViewer: dialog-focus elapsed=%s", time.Since(stepStart))
	d.debug("FileViewer: dialog-ready elapsed=%s", time.Since(totalStart))
}

func fileViewerDialogSize(parent fyne.Window, maxWidth, maxHeight int) fyne.Size {
	if parent == nil || parent.Canvas() == nil {
		return cappedFileViewerSize(fileViewerFallbackWidth, fileViewerFallbackHeight, maxWidth, maxHeight)
	}
	canvasSize := parent.Canvas().Size()
	return cappedFileViewerSize(canvasSize.Width*fileViewerWidthRatio, canvasSize.Height*fileViewerHeightRatio, maxWidth, maxHeight)
}

func cappedFileViewerSize(width, height float32, maxWidth, maxHeight int) fyne.Size {
	if maxWidth > 0 && width > float32(maxWidth) {
		width = float32(maxWidth)
	}
	if maxHeight > 0 && height > float32(maxHeight) {
		height = float32(maxHeight)
	}
	return fyne.NewSize(width, height)
}

func (d *FileViewerDialog) markdownView() fyne.CanvasObject {
	return d.createMarkdownGrid()
}

func (d *FileViewerDialog) createMarkdownGrid() *fileViewerTextGrid {
	start := time.Now()
	if d.preview.Binary {
		d.debug("FileViewer: markdown-view elapsed=%s mode=binary-placeholder", time.Since(start))
		d.mdGrid = newFileViewerTextGrid("Binary file: markdown preview disabled. Use the Hex tab.", d.km, d.updateLineDisplay, d.debugPrint)
		d.mdGrid.SetCopyHandler(d.copySelection)
		return d.mdGrid
	}
	mdText := markdownViewerText(d.preview)
	d.mdGrid = newFileViewerTextGrid(mdText, d.km, d.updateLineDisplay, d.debugPrint)
	d.mdGrid.SetCopyHandler(d.copySelection)
	d.debug("FileViewer: markdown-view elapsed=%s mode=text-grid bytes=%d lines=%d", time.Since(start), len(mdText), d.mdGrid.TotalLines())
	return d.mdGrid
}

func (d *FileViewerDialog) ensureMarkdownView() {
	if d.mdTab == nil {
		return
	}
	if d.mdView != nil && d.mdTab.Content == d.mdView {
		return
	}
	stepStart := time.Now()
	if d.mdGrid != nil {
		d.mdView = d.mdGrid
	} else {
		d.mdView = d.markdownView()
	}
	d.mdTab.Content = d.mdView
	if d.tabs != nil {
		d.tabs.Refresh()
	}
	d.debug("FileViewer: markdown-lazy-load elapsed=%s", time.Since(stepStart))
}

func (d *FileViewerDialog) createHexGrid() *fileViewerTextGrid {
	stepStart := time.Now()
	hex := viewerHex(d.preview)
	d.debug("FileViewer: hex-view elapsed=%s bytes=%d", time.Since(stepStart), len(hex))
	stepStart = time.Now()
	grid := newFileViewerTextGrid(hex, d.km, d.updateLineDisplay, d.debugPrint)
	grid.SetCopyHandler(d.copySelection)
	d.hexGrid = grid
	d.debug("FileViewer: hex-grid elapsed=%s bytes=%d", time.Since(stepStart), len(hex))
	return grid
}

func (d *FileViewerDialog) ensureHexGrid() {
	if d.hexGrid != nil || d.hexTab == nil {
		return
	}
	stepStart := time.Now()
	d.hexTab.Content = d.createHexGrid()
	if d.tabs != nil {
		d.tabs.Refresh()
	}
	d.debug("FileViewer: hex-lazy-load elapsed=%s", time.Since(stepStart))
}

func (d *FileViewerDialog) selectInitialTab() {
	if d.preview.Binary {
		d.selectViewerTab(viewerPaneHex)
		return
	}
	switch d.defaultPane {
	case viewerPaneText:
		d.selectViewerTab(viewerPaneText)
	case viewerPaneMarkdown:
		d.selectViewerTab(viewerPaneMarkdown)
	case viewerPaneHex:
		d.selectViewerTab(viewerPaneHex)
	default:
		if d.preview.Markdown {
			d.selectViewerTab(viewerPaneMarkdown)
			return
		}
		d.selectViewerTab(viewerPaneText)
	}
}

func normalizeViewerPane(pane string) string {
	switch strings.ToLower(strings.TrimSpace(pane)) {
	case viewerPaneAuto, viewerPaneText, viewerPaneMarkdown, viewerPaneHex:
		return strings.ToLower(strings.TrimSpace(pane))
	default:
		return ""
	}
}

func viewerPaneNameForTab(d *FileViewerDialog, item *container.TabItem) string {
	switch item {
	case d.hexTab:
		return "Hex"
	case d.mdTab:
		return "Markdown"
	default:
		return "Text"
	}
}

func (d *FileViewerDialog) selectViewerTab(pane string) bool {
	if d.tabs == nil {
		return false
	}
	switch normalizeViewerPane(pane) {
	case viewerPaneText:
		if d.textTab == nil {
			return false
		}
		d.tabs.Select(d.textTab)
	case viewerPaneMarkdown:
		if d.mdTab == nil {
			return false
		}
		d.tabs.Select(d.mdTab)
	case viewerPaneHex:
		if d.hexTab == nil {
			return false
		}
		d.tabs.Select(d.hexTab)
	default:
		return false
	}
	return true
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
	return sanitizeViewerText(preview.Text)
}

func markdownViewerText(preview *fileinfo.PreviewFile) string {
	if preview == nil {
		return ""
	}
	frontMatter, body := splitMarkdownFrontMatter(preview.Text)
	source := []byte(body)
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	doc := md.Parser().Parse(text.NewReader(source))
	lines := trimViewerBlankLines(renderMarkdownBlocks(source, doc, 0))
	if len(frontMatter) > 0 {
		lines = append(frontMatter, append([]string{""}, lines...)...)
	}
	if len(lines) == 0 {
		return viewerText(preview)
	}
	return sanitizeViewerText(strings.Join(lines, "\n"))
}

func splitMarkdownFrontMatter(markdown string) ([]string, string) {
	if markdown == "" {
		return nil, markdown
	}
	markdown = strings.TrimPrefix(markdown, "\uFEFF")
	firstLineEnd := strings.IndexByte(markdown, '\n')
	if firstLineEnd < 0 {
		return nil, markdown
	}
	if strings.TrimSpace(strings.TrimSuffix(markdown[:firstLineEnd], "\r")) != "---" {
		return nil, markdown
	}
	bodyStart := firstLineEnd + 1
	for pos := bodyStart; pos <= len(markdown); {
		next := strings.IndexByte(markdown[pos:], '\n')
		lineEnd := len(markdown)
		if next >= 0 {
			lineEnd = pos + next
		}
		line := strings.TrimSpace(strings.TrimSuffix(markdown[pos:lineEnd], "\r"))
		if line == "---" || line == "..." {
			body := ""
			if lineEnd < len(markdown) {
				body = markdown[lineEnd+1:]
			}
			return formatMarkdownFrontMatter(markdown[bodyStart:pos]), body
		}
		if next < 0 {
			break
		}
		pos = lineEnd + 1
	}
	return nil, markdown
}

func formatMarkdownFrontMatter(src string) []string {
	rows := markdownFrontMatterRows(src)
	if len(rows) == 0 {
		return nil
	}
	widths := []int{3, 5}
	wideAmbiguous := viewerLocaleUsesWideAmbiguous()
	for _, row := range rows {
		for col, cell := range row {
			if width := viewerDisplayLineWidth(cell, wideAmbiguous); width > widths[col] {
				widths[col] = width
			}
		}
	}
	widths = capMarkdownTableWidths(widths, markdownTableTargetColumns)
	lines := []string{
		formatMarkdownTableRows([]string{"Name", "Value"}, widths, nil, wideAmbiguous)[0],
		formatMarkdownTableSeparator(widths),
	}
	for _, row := range rows {
		lines = append(lines, formatMarkdownTableRows(row, widths, nil, wideAmbiguous)...)
	}
	return lines
}

func markdownFrontMatterRows(src string) [][]string {
	var rows [][]string
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if len(rows) > 0 {
				rows[len(rows)-1][1] = strings.TrimSpace(rows[len(rows)-1][1] + " " + strings.TrimSpace(line))
			}
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		rows = append(rows, []string{strings.TrimSpace(key), strings.TrimSpace(value)})
	}
	return rows
}

func renderMarkdownBlocks(source []byte, node goldmarkast.Node, depth int) []string {
	var lines []string
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		lines = append(lines, renderMarkdownBlock(source, child, depth)...)
	}
	return lines
}

func renderMarkdownBlock(source []byte, node goldmarkast.Node, depth int) []string {
	switch n := node.(type) {
	case *goldmarkast.Heading:
		return []string{strings.Repeat("#", n.Level) + " " + markdownInlineText(source, n), ""}
	case *goldmarkast.Paragraph:
		if text := markdownInlineText(source, n); text != "" {
			return []string{text, ""}
		}
	case *goldmarkast.List:
		return renderMarkdownList(source, n, depth)
	case *goldmarkast.Blockquote:
		lines := renderMarkdownBlocks(source, n, depth)
		for i, line := range lines {
			if line != "" {
				lines[i] = "> " + line
			}
		}
		return append(lines, "")
	case *goldmarkast.CodeBlock:
		return renderMarkdownCodeBlock(source, n, "")
	case *goldmarkast.FencedCodeBlock:
		return renderMarkdownCodeBlock(source, n, string(n.Language(source)))
	case *goldmarkast.ThematicBreak:
		return []string{strings.Repeat("-", 40), ""}
	case *tableast.Table:
		return append(renderMarkdownTable(source, n), "")
	default:
		return renderMarkdownBlocks(source, node, depth)
	}
	return nil
}

func renderMarkdownList(source []byte, list *goldmarkast.List, depth int) []string {
	var lines []string
	index := list.Start
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		marker := "-"
		if list.IsOrdered() {
			marker = strconv.Itoa(index) + "."
			index++
		}
		itemLines := renderMarkdownListItem(source, item, depth+1)
		indent := strings.Repeat("  ", depth)
		if len(itemLines) == 0 {
			lines = append(lines, indent+marker)
			continue
		}
		lines = append(lines, indent+marker+" "+itemLines[0])
		continuation := indent + strings.Repeat(" ", len(marker)+1)
		for _, line := range itemLines[1:] {
			if line == "" {
				lines = append(lines, "")
			} else {
				lines = append(lines, continuation+line)
			}
		}
	}
	return append(lines, "")
}

func renderMarkdownListItem(source []byte, item goldmarkast.Node, depth int) []string {
	var lines []string
	for child := item.FirstChild(); child != nil; child = child.NextSibling() {
		switch child.(type) {
		case *goldmarkast.List:
			lines = append(lines, renderMarkdownBlock(source, child, depth)...)
		default:
			if text := markdownInlineText(source, child); text != "" {
				lines = append(lines, text)
			} else {
				lines = append(lines, trimViewerBlankLines(renderMarkdownBlock(source, child, depth))...)
			}
		}
	}
	return trimViewerBlankLines(lines)
}

type markdownCodeBlock interface {
	Lines() *text.Segments
}

func renderMarkdownCodeBlock(source []byte, node markdownCodeBlock, language string) []string {
	lines := []string{"```" + language}
	segments := node.Lines()
	for i := 0; i < segments.Len(); i++ {
		segment := segments.At(i)
		lines = append(lines, strings.TrimSuffix(string(segment.Value(source)), "\n"))
	}
	return append(lines, "```", "")
}

func renderMarkdownTable(source []byte, table *tableast.Table) []string {
	var rows [][]string
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		rows = append(rows, markdownTableRow(source, row))
	}
	if len(rows) == 0 {
		return nil
	}
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	widths := make([]int, cols)
	wideAmbiguous := viewerLocaleUsesWideAmbiguous()
	for _, row := range rows {
		for col, cell := range row {
			if width := viewerDisplayLineWidth(cell, wideAmbiguous); width > widths[col] {
				widths[col] = width
			}
		}
	}
	for i, width := range widths {
		if width < 3 {
			widths[i] = 3
		}
	}
	widths = capMarkdownTableWidths(widths, markdownTableTargetColumns)

	lines := make([]string, 0, len(rows)+1)
	for i, row := range rows {
		lines = append(lines, formatMarkdownTableRows(row, widths, table.Alignments, wideAmbiguous)...)
		if i == 0 {
			lines = append(lines, formatMarkdownTableSeparator(widths))
		}
	}
	return lines
}

func markdownTableRow(source []byte, row goldmarkast.Node) []string {
	var cells []string
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		cells = append(cells, markdownInlineText(source, cell))
	}
	return cells
}

func capMarkdownTableWidths(widths []int, target int) []int {
	capped := append([]int(nil), widths...)
	if len(capped) == 0 || target <= 0 {
		return capped
	}
	available := target - (len(capped)*3 + 1)
	minWidth := markdownTableMinColumn
	if available < len(capped)*minWidth {
		minWidth = max(3, available/len(capped))
	}
	if minWidth < 3 {
		minWidth = 3
	}
	for markdownTableContentWidth(capped) > available {
		col := -1
		for i, width := range capped {
			if width <= minWidth {
				continue
			}
			if col < 0 || width > capped[col] {
				col = i
			}
		}
		if col < 0 {
			break
		}
		capped[col]--
	}
	return capped
}

func markdownTableContentWidth(widths []int) int {
	total := 0
	for _, width := range widths {
		total += width
	}
	return total
}

func formatMarkdownTableRows(row []string, widths []int, alignments []tableast.Alignment, wideAmbiguous bool) []string {
	cells := make([][]string, len(widths))
	maxRows := 1
	for col, width := range widths {
		cell := ""
		if col < len(row) {
			cell = row[col]
		}
		cells[col] = wrapMarkdownTableCell(cell, width, wideAmbiguous)
		if len(cells[col]) > maxRows {
			maxRows = len(cells[col])
		}
	}
	lines := make([]string, 0, maxRows)
	for rowIndex := 0; rowIndex < maxRows; rowIndex++ {
		lines = append(lines, formatMarkdownTableRow(cells, rowIndex, widths, alignments, wideAmbiguous))
	}
	return lines
}

func formatMarkdownTableRow(cells [][]string, rowIndex int, widths []int, alignments []tableast.Alignment, wideAmbiguous bool) string {
	var b strings.Builder
	b.WriteByte('|')
	for col, width := range widths {
		cell := ""
		if col < len(cells) && rowIndex < len(cells[col]) {
			cell = cells[col][rowIndex]
		}
		align := tableast.AlignNone
		if col < len(alignments) {
			align = alignments[col]
		}
		b.WriteByte(' ')
		b.WriteString(padMarkdownTableCell(cell, width, align, wideAmbiguous))
		b.WriteByte(' ')
		b.WriteByte('|')
	}
	return b.String()
}

func wrapMarkdownTableCell(cell string, width int, wideAmbiguous bool) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(cell)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := ""
	for _, word := range words {
		if current == "" {
			if viewerDisplayLineWidth(word, wideAmbiguous) <= width {
				current = word
				continue
			}
			lines = append(lines, splitMarkdownTableWord(word, width, wideAmbiguous)...)
			continue
		}
		candidate := current + " " + word
		if viewerDisplayLineWidth(candidate, wideAmbiguous) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		current = ""
		if viewerDisplayLineWidth(word, wideAmbiguous) <= width {
			current = word
		} else {
			lines = append(lines, splitMarkdownTableWord(word, width, wideAmbiguous)...)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func splitMarkdownTableWord(word string, width int, wideAmbiguous bool) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	var b strings.Builder
	col := 0
	for _, r := range word {
		w := viewerRuneWidth(r, wideAmbiguous)
		if w < 1 {
			w = 1
		}
		if col > 0 && col+w > width {
			lines = append(lines, b.String())
			b.Reset()
			col = 0
		}
		b.WriteRune(r)
		col += w
	}
	if b.Len() > 0 {
		lines = append(lines, b.String())
	}
	if len(lines) == 0 {
		return []string{word}
	}
	return lines
}

func formatMarkdownTableSeparator(widths []int) string {
	var b strings.Builder
	b.WriteByte('|')
	for _, width := range widths {
		b.WriteByte(' ')
		b.WriteString(strings.Repeat("-", width))
		b.WriteByte(' ')
		b.WriteByte('|')
	}
	return b.String()
}

func padMarkdownTableCell(cell string, width int, align tableast.Alignment, wideAmbiguous bool) string {
	padding := width - viewerDisplayLineWidth(cell, wideAmbiguous)
	if padding <= 0 {
		return cell
	}
	switch align {
	case tableast.AlignRight:
		return strings.Repeat(" ", padding) + cell
	case tableast.AlignCenter:
		left := padding / 2
		return strings.Repeat(" ", left) + cell + strings.Repeat(" ", padding-left)
	default:
		return cell + strings.Repeat(" ", padding)
	}
}

func markdownInlineText(source []byte, node goldmarkast.Node) string {
	var b strings.Builder
	_ = goldmarkast.Walk(node, func(n goldmarkast.Node, entering bool) (goldmarkast.WalkStatus, error) {
		if !entering {
			return goldmarkast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *goldmarkast.Text:
			b.WriteString(string(t.Value(source)))
			if t.HardLineBreak() || t.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *goldmarkast.String:
			b.Write(t.Value)
		}
		return goldmarkast.WalkContinue, nil
	})
	return strings.TrimSpace(html.UnescapeString(b.String()))
}

func trimViewerBlankLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
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
	return fileinfo.FormatHexDump(preview.Data)
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
			d.km.RemoveHandler(d.kmToken)
			d.handlerSet = false
		}
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.textGrid, d.hexGrid, d.mdGrid, d.search, d.jump)
	})
}

func (d *FileViewerDialog) CloseViewer() {
	d.CancelDialog()
}

func (d *FileViewerDialog) activeGrid() *fileViewerTextGrid {
	switch d.activeName {
	case "Hex":
		return d.hexGrid
	case "Markdown":
		return d.mdGrid
	case "Text":
		return d.textGrid
	default:
		return nil
	}
}

func (d *FileViewerDialog) focusActiveViewer() {
	if d.parent == nil {
		return
	}
	grid := d.activeGrid()
	if grid != nil {
		d.parent.Canvas().Focus(grid)
	}
}

func (d *FileViewerDialog) copySelection() {
	grid := d.activeGrid()
	if grid == nil {
		d.debug("FileViewer: copy-selection active=%s grid=false", d.activeName)
		d.setStatusSuffix("copy=unsupported")
		d.focusActiveViewer()
		return
	}
	text := grid.SelectedText()
	start, end, chars := grid.selectionDebugInfo()
	d.debug("FileViewer: copy-selection active=%s selection=%t start=%d:%d end=%d:%d chars=%d text_chars=%d",
		d.activeName, grid.selection.set, start.line+1, start.col, end.line+1, end.col, chars, len([]rune(text)))
	if text == "" {
		d.setStatusSuffix("copy=no-selection")
		d.focusActiveViewer()
		return
	}
	clipboard := fyne.CurrentApp().Clipboard()
	if clipboard == nil {
		d.debug("FileViewer: copy-selection clipboard=false")
		d.setStatusSuffix("copy=no-clipboard")
		d.focusActiveViewer()
		return
	}
	clipboard.SetContent(text)
	d.debug("FileViewer: copy-selection clipboard-set chars=%d", len([]rune(text)))
	d.setStatusSuffix(fmt.Sprintf("copied=%d", len([]rune(text))))
	d.focusActiveViewer()
}

func (d *FileViewerDialog) findNext() {
	d.find(1)
}

func (d *FileViewerDialog) findPrevious() {
	d.find(-1)
}

func (d *FileViewerDialog) find(direction int) {
	query := d.search.Text
	if query == "" {
		if grid := d.activeGrid(); grid != nil {
			grid.Find("", direction)
		}
		return
	}
	if d.activeName == "Markdown" {
		d.selectViewerTab(viewerPaneText)
	}
	if grid := d.activeGrid(); grid != nil {
		result := grid.Find(query, direction)
		d.updateLineDisplay()
		if !result.Matched {
			d.setStatusSuffix("search=not-found")
			d.focusActiveViewer()
			return
		}
		suffix := fmt.Sprintf("match line=%d col=%d", result.Line, result.Column)
		if result.Wrapped {
			suffix += " wrapped"
		}
		d.setStatusSuffix(suffix)
		d.focusActiveViewer()
		return
	}
}

func (d *FileViewerDialog) jumpToLine() {
	line, err := strconv.Atoi(strings.TrimSpace(d.jump.Text))
	if err != nil || line <= 0 {
		return
	}
	if d.activeName == "Markdown" {
		d.selectViewerTab(viewerPaneText)
	}
	if grid := d.activeGrid(); grid != nil {
		line = grid.JumpToLine(line)
		d.updateLineDisplay()
		d.setStatusSuffix(fmt.Sprintf("line=%d", line))
		d.focusActiveViewer()
		return
	}
}

func (d *FileViewerDialog) ViewerLineDown() {
	d.moveCursorRows(1)
}

func (d *FileViewerDialog) ViewerLineUp() {
	d.moveCursorRows(-1)
}

func (d *FileViewerDialog) ViewerPageDown() {
	if grid := d.activeGrid(); grid != nil {
		grid.PageDown()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
}

func (d *FileViewerDialog) ViewerPageUp() {
	if grid := d.activeGrid(); grid != nil {
		grid.PageUp()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
}

func (d *FileViewerDialog) ViewerColumnLeft() {
	if grid := d.activeGrid(); grid != nil {
		grid.MoveColumns(-1)
		d.updateLineDisplay()
		d.focusActiveViewer()
	}
}

func (d *FileViewerDialog) ViewerColumnRight() {
	if grid := d.activeGrid(); grid != nil {
		grid.MoveColumns(1)
		d.updateLineDisplay()
		d.focusActiveViewer()
	}
}

func (d *FileViewerDialog) ViewerToggleWrap() {
	grid := d.activeGrid()
	if grid == nil {
		return
	}
	wrapped := grid.ToggleWrap()
	d.updateLineDisplay()
	if wrapped {
		d.setStatusSuffix("wrap=on")
	} else {
		d.setStatusSuffix("wrap=off")
	}
	d.focusActiveViewer()
}

func (d *FileViewerDialog) ViewerShowText() {
	d.showViewerPane(viewerPaneText)
}

func (d *FileViewerDialog) ViewerShowMarkdown() {
	d.showViewerPane(viewerPaneMarkdown)
}

func (d *FileViewerDialog) ViewerShowHex() {
	d.showViewerPane(viewerPaneHex)
}

func (d *FileViewerDialog) showViewerPane(pane string) {
	if !d.selectViewerTab(pane) {
		return
	}
	d.updateLineDisplay()
	d.focusActiveViewer()
}

func (d *FileViewerDialog) ViewerHome() {
	if grid := d.activeGrid(); grid != nil {
		grid.Home()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
}

func (d *FileViewerDialog) ViewerEnd() {
	if grid := d.activeGrid(); grid != nil {
		grid.End()
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
}

func (d *FileViewerDialog) ViewerSearchNext() {
	d.findNext()
}

func (d *FileViewerDialog) ViewerSearchPrevious() {
	d.findPrevious()
}

func (d *FileViewerDialog) ViewerFocusSearch() {
	d.deferViewerFocus("viewer.focusSearch", func() {
		if d.parent != nil && d.search != nil {
			d.parent.Canvas().Focus(d.search)
		}
	})
}

func (d *FileViewerDialog) deferViewerFocus(label string, focus func()) {
	if d.km != nil {
		d.km.BeginOwnerTransition(label, focus)
		return
	}
	focus()
}

func (d *FileViewerDialog) ViewerFocusLine() {
	d.deferViewerFocus("viewer.focusLine", func() {
		if d.parent != nil && d.jump != nil {
			d.parent.Canvas().Focus(d.jump)
		}
	})
}

func (d *FileViewerDialog) ViewerCopySelection() {
	d.debug("FileViewer: copy-selection via=keymanager")
	d.copySelection()
}

func (d *FileViewerDialog) ViewerSelectAll() {
	grid := d.activeGrid()
	if grid == nil {
		d.debug("FileViewer: select-all active=%s grid=false", d.activeName)
		d.setStatusSuffix("select=unsupported")
		d.focusActiveViewer()
		return
	}
	chars := grid.SelectAll()
	d.debug("FileViewer: select-all active=%s chars=%d", d.activeName, chars)
	d.setStatusSuffix(fmt.Sprintf("selected=%d", chars))
	d.focusActiveViewer()
}

func (d *FileViewerDialog) moveCursorRows(delta int) {
	if grid := d.activeGrid(); grid != nil {
		grid.MoveRows(delta)
		d.updateLineDisplay()
		d.focusActiveViewer()
		return
	}
}

func (d *FileViewerDialog) updateLineDisplay() {
	if d.lineLabel == nil {
		return
	}
	if grid := d.activeGrid(); grid != nil {
		mode := fmt.Sprintf("col=%d", grid.CurrentColumn())
		if grid.Wrap() {
			mode = "wrap"
		}
		d.lineLabel.SetText(fmt.Sprintf("line=%d/%d  %s", grid.CurrentLine(), grid.TotalLines(), mode))
		return
	}
	d.lineLabel.SetText("")
}

func (d *FileViewerDialog) setStatusSuffix(suffix string) {
	d.status.SetText(d.statusText() + "  " + suffix)
}
