package ui

import (
	"os"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	locale "github.com/jeandeaual/go-locale"
	"golang.org/x/text/width"

	"nmf/internal/keymanager"
)

const (
	fileViewerTextGridFallbackRows = 20
	fileViewerTextGridFallbackCols = 80
	fileViewerTextGridTabWidth     = 4
)

type fileViewerTextGrid struct {
	widget.BaseWidget

	grid          *widget.TextGrid
	lines         []string
	visible       []viewerVisibleLine
	topLine       int
	leftCol       int
	visibleRows   int
	visibleCols   int
	wrap          bool
	wideAmbiguous bool
	cellSize      fyne.Size
	selection     viewerTextSelection
	selecting     bool

	km         *keymanager.KeyManager
	onMoved    func()
	onCopy     func()
	debugPrint func(format string, args ...interface{})
}

type viewerTextPosition struct {
	line int
	col  int
}

type viewerTextSelection struct {
	start viewerTextPosition
	end   viewerTextPosition
	set   bool
}

type viewerVisibleLine struct {
	line       int
	text       string
	cellToCol  []int
	startCol   int
	displayLen int
}

func newFileViewerTextGrid(text string, km *keymanager.KeyManager, onMoved func(), debugPrint func(format string, args ...interface{})) *fileViewerTextGrid {
	start := time.Now()
	v := &fileViewerTextGrid{
		grid:          widget.NewTextGrid(),
		lines:         splitViewerLines(text),
		visibleRows:   fileViewerTextGridFallbackRows,
		visibleCols:   fileViewerTextGridFallbackCols,
		wideAmbiguous: viewerLocaleUsesWideAmbiguous(),
		km:            km,
		onMoved:       onMoved,
		debugPrint:    debugPrint,
	}
	v.grid.Scroll = fyne.ScrollNone
	v.ExtendBaseWidget(v)
	v.refreshGrid()
	v.debug("FileViewer: text-grid-init elapsed=%s lines=%d", time.Since(start), len(v.lines))
	return v
}

func splitViewerLines(text string) []string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func viewerLocaleUsesWideAmbiguous() bool {
	if lang, err := viewerSystemLanguage(); err == nil && lang != "" {
		return viewerLocaleLanguageUsesWideAmbiguous(lang)
	}
	if locales, err := viewerSystemLocales(); err == nil {
		for _, locale := range locales {
			if locale == "" {
				continue
			}
			return viewerLocaleLanguageUsesWideAmbiguous(locale)
		}
	}
	return viewerEnvUsesWideAmbiguous()
}

var (
	viewerSystemLanguage = locale.GetLanguage
	viewerSystemLocales  = locale.GetLocales
)

func viewerEnvUsesWideAmbiguous() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := os.Getenv(key)
		if value == "" {
			continue
		}
		return viewerLocaleLanguageUsesWideAmbiguous(value)
	}
	return false
}

func viewerLocaleLanguageUsesWideAmbiguous(locale string) bool {
	lang := strings.ToLower(strings.TrimSpace(locale))
	if lang == "" || lang == "c" || lang == "posix" {
		return false
	}
	if idx := strings.IndexAny(lang, "-_.@"); idx >= 0 {
		lang = lang[:idx]
	}
	switch lang {
	case "ja", "ko", "zh":
		return true
	default:
		return false
	}
}

func viewerDisplayLine(line string, wideAmbiguous bool) string {
	var b strings.Builder
	col := 0
	for _, r := range line {
		if r == '\t' {
			next := nextTabStop(col, fileViewerTextGridTabWidth)
			b.WriteString(strings.Repeat(" ", next-col))
			col = next
			continue
		}
		w := viewerRuneWidth(r, wideAmbiguous)
		if w == 0 {
			b.WriteRune(r)
			continue
		}
		b.WriteRune(r)
		col += w
		if w > 1 {
			b.WriteString(strings.Repeat(" ", w-1))
		}
	}
	return b.String()
}

func viewerDisplayLineWidth(line string, wideAmbiguous bool) int {
	col := 0
	for _, r := range line {
		if r == '\t' {
			col = nextTabStop(col, fileViewerTextGridTabWidth)
			continue
		}
		col += viewerRuneWidth(r, wideAmbiguous)
	}
	return col
}

func viewerRuneWidth(r rune, wideAmbiguous bool) int {
	if r == 0 || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	case width.EastAsianAmbiguous:
		if wideAmbiguous {
			return 2
		}
	}
	return 1
}

func nextTabStop(col, tabWidth int) int {
	if tabWidth <= 0 {
		tabWidth = fileViewerTextGridTabWidth
	}
	return ((col / tabWidth) + 1) * tabWidth
}

func (v *fileViewerTextGrid) CreateRenderer() fyne.WidgetRenderer {
	return &fileViewerTextGridRenderer{viewer: v}
}

func (v *fileViewerTextGrid) Resize(size fyne.Size) {
	v.BaseWidget.Resize(size)
	v.grid.Resize(size)
	v.updateVisibleRows(size)
}

func (v *fileViewerTextGrid) FocusGained() {}

func (v *fileViewerTextGrid) FocusLost() {}

func (v *fileViewerTextGrid) TypedKey(ev *fyne.KeyEvent) {
	if v.km != nil {
		v.km.HandleTypedKey(ev)
	}
}

func (v *fileViewerTextGrid) TypedRune(r rune) {
	if v.km != nil {
		v.km.HandleTypedRune(r)
	}
}

func (v *fileViewerTextGrid) TypedShortcut(shortcut fyne.Shortcut) {
	if _, ok := shortcut.(*fyne.ShortcutCopy); ok {
		start, end, chars := v.selectionDebugInfo()
		v.debug("FileViewer: text-grid-copy-shortcut ignored=keymanager-path selection=%t start=%d:%d end=%d:%d chars=%d",
			v.selection.set, start.line+1, start.col, end.line+1, end.col, chars)
		return
	}
	if v.km == nil {
		return
	}
	s, ok := shortcut.(*desktop.CustomShortcut)
	if !ok {
		return
	}
	modifiers := keymanager.ModifierState{
		ShiftPressed: s.Modifier&fyne.KeyModifierShift != 0,
		CtrlPressed:  s.Modifier&fyne.KeyModifierControl != 0,
		AltPressed:   s.Modifier&fyne.KeyModifierAlt != 0,
	}
	v.km.HandleShortcutKey(&fyne.KeyEvent{Name: s.KeyName}, modifiers)
}

func (v *fileViewerTextGrid) KeyDown(ev *fyne.KeyEvent) {
	if v.km != nil {
		v.km.HandleKeyDown(ev)
	}
}

func (v *fileViewerTextGrid) KeyUp(ev *fyne.KeyEvent) {
	if v.km != nil {
		v.km.HandleKeyUp(ev)
	}
}

func (v *fileViewerTextGrid) AcceptsTab() bool { return true }

func (v *fileViewerTextGrid) SetCopyHandler(onCopy func()) {
	v.onCopy = onCopy
}

func (v *fileViewerTextGrid) Tapped(ev *fyne.PointEvent) {
	if ev == nil {
		return
	}
	v.debug("FileViewer: text-grid-selection-clear x=%.1f y=%.1f", ev.Position.X, ev.Position.Y)
	v.selection = viewerTextSelection{}
	v.selecting = false
	v.refreshGrid()
}

func (v *fileViewerTextGrid) Dragged(ev *fyne.DragEvent) {
	if ev == nil {
		return
	}
	pos := v.textPositionForCanvasPosition(ev.Position)
	if !v.selecting {
		start := v.textPositionForCanvasPosition(fyne.NewPos(ev.Position.X-ev.Dragged.DX, ev.Position.Y-ev.Dragged.DY))
		v.selection = viewerTextSelection{start: start, end: pos, set: true}
		v.selecting = true
		startNorm, endNorm, chars := v.selectionDebugInfo()
		v.debug("FileViewer: text-grid-selection-start from=%d:%d to=%d:%d norm=%d:%d-%d:%d chars=%d x=%.1f y=%.1f dx=%.1f dy=%.1f",
			start.line+1, start.col, pos.line+1, pos.col,
			startNorm.line+1, startNorm.col, endNorm.line+1, endNorm.col, chars,
			ev.Position.X, ev.Position.Y, ev.Dragged.DX, ev.Dragged.DY)
	} else {
		v.selection.end = pos
		startNorm, endNorm, chars := v.selectionDebugInfo()
		v.debug("FileViewer: text-grid-selection-update to=%d:%d norm=%d:%d-%d:%d chars=%d x=%.1f y=%.1f dx=%.1f dy=%.1f",
			pos.line+1, pos.col,
			startNorm.line+1, startNorm.col, endNorm.line+1, endNorm.col, chars,
			ev.Position.X, ev.Position.Y, ev.Dragged.DX, ev.Dragged.DY)
	}
	v.refreshGrid()
}

func (v *fileViewerTextGrid) DragEnd() {
	v.selecting = false
	start, end, chars := v.selectionDebugInfo()
	v.debug("FileViewer: text-grid-selection-end selection=%t start=%d:%d end=%d:%d chars=%d",
		v.selection.set, start.line+1, start.col, end.line+1, end.col, chars)
	if v.selection.set && compareViewerTextPosition(v.selection.start, v.selection.end) == 0 {
		v.selection = viewerTextSelection{}
		v.refreshGrid()
	}
}

func (v *fileViewerTextGrid) Scrolled(ev *fyne.ScrollEvent) {
	if ev == nil {
		return
	}
	if ev.Scrolled.DY < 0 {
		v.MoveRows(1)
	} else if ev.Scrolled.DY > 0 {
		v.MoveRows(-1)
	}
}

func (v *fileViewerTextGrid) MoveRows(delta int) {
	v.setTopLine(v.topLine + delta)
}

func (v *fileViewerTextGrid) PageDown() {
	v.MoveRows(v.pageRows())
}

func (v *fileViewerTextGrid) PageUp() {
	v.MoveRows(-v.pageRows())
}

func (v *fileViewerTextGrid) MoveColumns(delta int) {
	if v.wrap {
		return
	}
	v.setLeftCol(v.leftCol + delta)
}

func (v *fileViewerTextGrid) ToggleWrap() bool {
	v.wrap = !v.wrap
	v.leftCol = 0
	v.refreshGrid()
	if v.onMoved != nil {
		v.onMoved()
	}
	return v.wrap
}

func (v *fileViewerTextGrid) Wrap() bool {
	return v.wrap
}

func (v *fileViewerTextGrid) CurrentColumn() int {
	return v.leftCol + 1
}

func (v *fileViewerTextGrid) Home() {
	v.setTopLine(0)
}

func (v *fileViewerTextGrid) End() {
	v.setTopLine(v.maxTopLine())
}

func (v *fileViewerTextGrid) JumpToLine(line int) int {
	if line < 1 {
		line = 1
	}
	total := v.TotalLines()
	if line > total {
		line = total
	}
	v.setTopLine(line - 1)
	return line
}

func (v *fileViewerTextGrid) CurrentLine() int {
	line := v.topLine + 1
	if line < 1 {
		return 1
	}
	total := v.TotalLines()
	if line > total {
		return total
	}
	return line
}

func (v *fileViewerTextGrid) TotalLines() int {
	if len(v.lines) == 0 {
		return 1
	}
	return len(v.lines)
}

func (v *fileViewerTextGrid) pageRows() int {
	if v.visibleRows > 1 {
		return v.visibleRows - 1
	}
	return fileViewerTextGridFallbackRows
}

func (v *fileViewerTextGrid) updateVisibleRows(size fyne.Size) {
	if size.Width <= 0 && size.Height <= 0 {
		return
	}
	textSize := v.Theme().Size(theme.SizeNameText)
	cell := fyne.MeasureText("M", textSize, fyne.TextStyle{Monospace: true})
	if cell.Width <= 0 || cell.Height <= 0 {
		return
	}
	v.cellSize = cell
	rows := v.visibleRows
	if size.Height > 0 {
		rows = int(size.Height / cell.Height)
		if rows < 1 {
			rows = 1
		}
	}
	cols := v.visibleCols
	if size.Width > 0 {
		cols = int(size.Width / cell.Width)
		if cols < 1 {
			cols = 1
		}
	}
	if rows == v.visibleRows && cols == v.visibleCols {
		return
	}
	v.visibleRows = rows
	v.visibleCols = cols
	v.setLeftCol(v.leftCol)
	v.setTopLine(v.topLine)
}

func (v *fileViewerTextGrid) setTopLine(line int) {
	maxTop := v.maxTopLine()
	if line < 0 {
		line = 0
	}
	if line > maxTop {
		line = maxTop
	}
	if line == v.topLine {
		v.refreshGrid()
		return
	}
	v.topLine = line
	v.refreshGrid()
	if v.onMoved != nil {
		v.onMoved()
	}
}

func (v *fileViewerTextGrid) setLeftCol(col int) {
	maxCol := v.maxLeftCol()
	if col < 0 {
		col = 0
	}
	if col > maxCol {
		col = maxCol
	}
	if col == v.leftCol {
		v.refreshGrid()
		return
	}
	v.leftCol = col
	v.refreshGrid()
	if v.onMoved != nil {
		v.onMoved()
	}
}

func (v *fileViewerTextGrid) maxTopLine() int {
	total := v.TotalLines()
	maxTop := total - 1
	if maxTop < 0 {
		return 0
	}
	return maxTop
}

func (v *fileViewerTextGrid) maxLeftCol() int {
	if v.wrap {
		return 0
	}
	cols := v.visibleCols
	if cols < 1 {
		cols = fileViewerTextGridFallbackCols
	}
	maxWidth := 0
	for _, line := range v.lines {
		if w := viewerDisplayLineWidth(line, v.wideAmbiguous); w > maxWidth {
			maxWidth = w
		}
	}
	maxCol := maxWidth - cols
	if maxCol < 0 {
		return 0
	}
	return maxCol
}

func (v *fileViewerTextGrid) refreshGrid() {
	start := time.Now()
	rows := v.visibleRows
	if rows < 1 {
		rows = fileViewerTextGridFallbackRows
	}
	cols := v.visibleCols
	if cols < 1 {
		cols = fileViewerTextGridFallbackCols
	}
	v.visible = v.renderVisibleLines(rows, cols)
	displayLines := make([]string, 0, len(v.visible))
	for _, row := range v.visible {
		displayLines = append(displayLines, row.text)
	}
	v.grid.SetText(strings.Join(displayLines, "\n"))
	v.applySelectionStyle()
	v.grid.Refresh()
	v.debug("FileViewer: text-grid-refresh elapsed=%s top=%d col=%d rows=%d cols=%d wrap=%t",
		time.Since(start), v.topLine+1, v.leftCol+1, rows, cols, v.wrap)
}

func (v *fileViewerTextGrid) debug(format string, args ...interface{}) {
	if v.debugPrint != nil {
		v.debugPrint(format, args...)
	}
}

func (v *fileViewerTextGrid) renderVisibleLines(rows, cols int) []viewerVisibleLine {
	visible := make([]viewerVisibleLine, 0, rows)
	for i := v.topLine; i < len(v.lines) && len(visible) < rows; i++ {
		lineMap := newViewerDisplayLineMap(v.lines[i], v.wideAmbiguous)
		if v.wrap {
			visible = append(visible, lineMap.wrap(i, cols, rows-len(visible))...)
			continue
		}
		visible = append(visible, lineMap.slice(i, v.leftCol, cols))
	}
	return visible
}

func (v *fileViewerTextGrid) applySelectionStyle() {
	if !v.selection.set || len(v.visible) == 0 {
		return
	}
	start, end := v.normalizedSelection()
	if compareViewerTextPosition(start, end) == 0 {
		return
	}
	style := &widget.CustomTextGridStyle{
		BGColor: theme.Color(theme.ColorNameSelection),
	}
	for rowIdx, row := range v.visible {
		if row.line < start.line || row.line > end.line || row.displayLen == 0 {
			continue
		}
		startCol := 0
		endCol := row.displayLen
		if row.line == start.line {
			startCol = row.displayColumnForLogicalCol(start.col)
		}
		if row.line == end.line {
			endCol = row.displayColumnForLogicalCol(end.col)
		}
		if startCol < 0 {
			startCol = 0
		}
		if endCol > row.displayLen {
			endCol = row.displayLen
		}
		if startCol >= endCol {
			continue
		}
		v.grid.SetStyleRange(rowIdx, startCol, rowIdx, endCol-1, style)
	}
}

func (v *fileViewerTextGrid) textPositionForCanvasPosition(pos fyne.Position) viewerTextPosition {
	rowIdx := 0
	if v.cellSize.Height > 0 {
		rowIdx = int(pos.Y / v.cellSize.Height)
	}
	if rowIdx < 0 {
		rowIdx = 0
	}
	if rowIdx >= len(v.visible) {
		rowIdx = len(v.visible) - 1
	}
	if rowIdx < 0 {
		return viewerTextPosition{}
	}
	col := 0
	if v.cellSize.Width > 0 {
		col = int((pos.X + v.cellSize.Width/2) / v.cellSize.Width)
	}
	if col < 0 {
		col = 0
	}
	return v.visible[rowIdx].logicalPositionForDisplayColumn(col)
}

func (v *fileViewerTextGrid) SelectedText() string {
	if !v.selection.set {
		return ""
	}
	start, end := v.normalizedSelection()
	if compareViewerTextPosition(start, end) == 0 {
		return ""
	}
	return viewerTextForRange(v.lines, start, end)
}

func (v *fileViewerTextGrid) selectionDebugInfo() (viewerTextPosition, viewerTextPosition, int) {
	if !v.selection.set {
		return viewerTextPosition{}, viewerTextPosition{}, 0
	}
	start, end := v.normalizedSelection()
	text := viewerTextForRange(v.lines, start, end)
	return start, end, len([]rune(text))
}

func (v *fileViewerTextGrid) normalizedSelection() (viewerTextPosition, viewerTextPosition) {
	start := v.clampTextPosition(v.selection.start)
	end := v.clampTextPosition(v.selection.end)
	if compareViewerTextPosition(end, start) < 0 {
		return end, start
	}
	return start, end
}

func (v *fileViewerTextGrid) clampTextPosition(pos viewerTextPosition) viewerTextPosition {
	if len(v.lines) == 0 {
		return viewerTextPosition{}
	}
	if pos.line < 0 {
		pos.line = 0
	}
	if pos.line >= len(v.lines) {
		pos.line = len(v.lines) - 1
	}
	lineLen := len([]rune(v.lines[pos.line]))
	if pos.col < 0 {
		pos.col = 0
	}
	if pos.col > lineLen {
		pos.col = lineLen
	}
	return pos
}

type viewerDisplayLineMap struct {
	text      []rune
	cellToCol []int
}

func newViewerDisplayLineMap(line string, wideAmbiguous bool) viewerDisplayLineMap {
	var text []rune
	cellToCol := []int{0}
	displayCol := 0
	logicalCol := 0
	for _, r := range line {
		widthCells := viewerRuneWidth(r, wideAmbiguous)
		if r == '\t' {
			widthCells = nextTabStop(displayCol, fileViewerTextGridTabWidth) - displayCol
		}
		if widthCells < 1 {
			widthCells = 1
		}
		for cell := 0; cell < widthCells; cell++ {
			if r == '\t' || cell > 0 {
				text = append(text, ' ')
			} else {
				text = append(text, r)
			}
			if cell < widthCells-1 {
				cellToCol = append(cellToCol, logicalCol)
			} else {
				cellToCol = append(cellToCol, logicalCol+1)
			}
		}
		displayCol += widthCells
		logicalCol++
	}
	return viewerDisplayLineMap{text: text, cellToCol: cellToCol}
}

func (m viewerDisplayLineMap) slice(line, startCol, width int) viewerVisibleLine {
	if width < 1 {
		width = fileViewerTextGridFallbackCols
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol >= len(m.text) {
		last := 0
		if len(m.cellToCol) > 0 {
			last = m.cellToCol[len(m.cellToCol)-1]
		}
		return viewerVisibleLine{
			line:       line,
			cellToCol:  []int{last},
			startCol:   startCol,
			displayLen: 0,
		}
	}
	end := startCol + width
	if end > len(m.text) {
		end = len(m.text)
	}
	return viewerVisibleLine{
		line:       line,
		text:       string(m.text[startCol:end]),
		cellToCol:  append([]int(nil), m.cellToCol[startCol:end+1]...),
		startCol:   startCol,
		displayLen: end - startCol,
	}
}

func (m viewerDisplayLineMap) wrap(line, width, limit int) []viewerVisibleLine {
	if width < 1 {
		width = fileViewerTextGridFallbackCols
	}
	if limit < 1 {
		return nil
	}
	if len(m.text) == 0 {
		return []viewerVisibleLine{{
			line:       line,
			cellToCol:  []int{0},
			displayLen: 0,
		}}
	}
	rows := make([]viewerVisibleLine, 0, limit)
	for start := 0; start < len(m.text) && len(rows) < limit; start += width {
		rows = append(rows, m.slice(line, start, width))
	}
	return rows
}

func (row viewerVisibleLine) logicalPositionForDisplayColumn(col int) viewerTextPosition {
	if col < 0 {
		col = 0
	}
	if col >= len(row.cellToCol) {
		col = len(row.cellToCol) - 1
	}
	if col < 0 {
		return viewerTextPosition{line: row.line}
	}
	return viewerTextPosition{line: row.line, col: row.cellToCol[col]}
}

func (row viewerVisibleLine) displayColumnForLogicalCol(logicalCol int) int {
	if len(row.cellToCol) == 0 {
		return 0
	}
	if logicalCol <= row.cellToCol[0] {
		return 0
	}
	for col, mapped := range row.cellToCol {
		if mapped >= logicalCol {
			return col
		}
	}
	return row.displayLen
}

func viewerTextForRange(lines []string, start, end viewerTextPosition) string {
	if len(lines) == 0 || compareViewerTextPosition(start, end) >= 0 {
		return ""
	}
	start = clampViewerTextPosition(lines, start)
	end = clampViewerTextPosition(lines, end)
	if compareViewerTextPosition(start, end) >= 0 {
		return ""
	}
	if start.line == end.line {
		return string([]rune(lines[start.line])[start.col:end.col])
	}
	parts := make([]string, 0, end.line-start.line+1)
	parts = append(parts, string([]rune(lines[start.line])[start.col:]))
	for line := start.line + 1; line < end.line; line++ {
		parts = append(parts, lines[line])
	}
	parts = append(parts, string([]rune(lines[end.line])[:end.col]))
	return strings.Join(parts, "\n")
}

func clampViewerTextPosition(lines []string, pos viewerTextPosition) viewerTextPosition {
	if len(lines) == 0 {
		return viewerTextPosition{}
	}
	if pos.line < 0 {
		pos.line = 0
	}
	if pos.line >= len(lines) {
		pos.line = len(lines) - 1
	}
	lineLen := len([]rune(lines[pos.line]))
	if pos.col < 0 {
		pos.col = 0
	}
	if pos.col > lineLen {
		pos.col = lineLen
	}
	return pos
}

func compareViewerTextPosition(a, b viewerTextPosition) int {
	if a.line < b.line {
		return -1
	}
	if a.line > b.line {
		return 1
	}
	if a.col < b.col {
		return -1
	}
	if a.col > b.col {
		return 1
	}
	return 0
}

func viewerSliceDisplayLine(line string, startCol, width int) string {
	if width < 1 {
		width = fileViewerTextGridFallbackCols
	}
	if startCol < 0 {
		startCol = 0
	}
	runes := []rune(line)
	if startCol >= len(runes) {
		return ""
	}
	end := startCol + width
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[startCol:end])
}

func viewerWrapDisplayLine(line string, width, limit int) []string {
	if width < 1 {
		width = fileViewerTextGridFallbackCols
	}
	if limit < 1 {
		return nil
	}
	if line == "" {
		return []string{""}
	}
	var rows []string
	for len(line) > 0 && len(rows) < limit {
		rows = append(rows, viewerSliceDisplayLine(line, 0, width))
		if len([]rune(line)) <= width {
			break
		}
		line = string([]rune(line)[width:])
	}
	return rows
}

type fileViewerTextGridRenderer struct {
	viewer *fileViewerTextGrid
}

func (r *fileViewerTextGridRenderer) Destroy() {}

func (r *fileViewerTextGridRenderer) Layout(size fyne.Size) {
	r.viewer.grid.Move(fyne.NewPos(0, 0))
	r.viewer.grid.Resize(size)
}

func (r *fileViewerTextGridRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *fileViewerTextGridRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.viewer.grid}
}

func (r *fileViewerTextGridRenderer) Refresh() {
	r.viewer.grid.Refresh()
}
