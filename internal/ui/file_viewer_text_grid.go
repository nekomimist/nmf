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
	topLine       int
	leftCol       int
	visibleRows   int
	visibleCols   int
	wrap          bool
	wideAmbiguous bool

	km         *keymanager.KeyManager
	onMoved    func()
	debugPrint func(format string, args ...interface{})
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
	displayLines := make([]string, 0, rows)
	for i := v.topLine; i < len(v.lines) && len(displayLines) < rows; i++ {
		display := viewerDisplayLine(v.lines[i], v.wideAmbiguous)
		if v.wrap {
			displayLines = append(displayLines, viewerWrapDisplayLine(display, cols, rows-len(displayLines))...)
			continue
		}
		displayLines = append(displayLines, viewerSliceDisplayLine(display, v.leftCol, cols))
	}
	v.grid.SetText(strings.Join(displayLines, "\n"))
	v.grid.Refresh()
	v.debug("FileViewer: text-grid-refresh elapsed=%s top=%d col=%d rows=%d cols=%d wrap=%t",
		time.Since(start), v.topLine+1, v.leftCol+1, rows, cols, v.wrap)
}

func (v *fileViewerTextGrid) debug(format string, args ...interface{}) {
	if v.debugPrint != nil {
		v.debugPrint(format, args...)
	}
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
