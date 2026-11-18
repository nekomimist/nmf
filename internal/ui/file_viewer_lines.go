package ui

import (
	"sort"
	"strings"
	"unicode/utf8"

	"nmf/internal/fileinfo"
)

// A text pane retains its physical lines. The hex pane retains the preview
// bytes and formats an addressed row only when display, search or copy needs it.
type viewerLines struct {
	lines       []string
	foldedLines []string
	hexData     []byte
	checkpoints map[int][]viewerLineCheckpoint
}

func (v *viewerLines) TotalLines() int {
	if len(v.lines) > 0 {
		return len(v.lines)
	}
	// Preserve the blank line after the classic hex dump's final newline.
	return (len(v.hexData)+15)/16 + 1
}

func (v *viewerLines) lineAt(i int) string {
	if len(v.lines) > 0 {
		return v.lines[i]
	}
	return fileinfo.FormatHexDumpLine(v.hexData, 16*i)
}

func (v *viewerLines) runeCount() int {
	if len(v.lines) > 0 {
		count := len(v.lines) - 1
		for _, line := range v.lines {
			count += utf8.RuneCountInString(line)
		}
		return count
	}
	rows := v.TotalLines() - 1
	if rows == 0 {
		return 0
	}
	return (rows-1)*(len(v.lineAt(0))+1) + len(v.lineAt(rows-1)) + 1
}

func viewerRuneOffset(s string, col int) int {
	for offset := range s {
		if col <= 0 {
			return offset
		}
		col--
	}
	return len(s)
}

func (v *viewerLines) textForRange(start, end viewerTextPosition) string {
	var b strings.Builder
	for i := start.line; i <= end.line; i++ {
		line := v.lineAt(i)
		if i == end.line {
			line = line[:viewerRuneOffset(line, end.col)]
		}
		if i == start.line {
			line = line[viewerRuneOffset(line, start.col):]
		} else {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

func (v *viewerLines) searchLine(i int, matchCase bool) string {
	line := v.lineAt(i)
	if matchCase {
		return line
	}
	if len(v.lines) == 0 {
		return strings.ToLower(line)
	}
	if v.foldedLines == nil {
		v.foldedLines = make([]string, len(v.lines))
	}
	if v.foldedLines[i] == "" {
		v.foldedLines[i] = strings.ToLower(line)
	}
	return v.foldedLines[i]
}

func (v *viewerLines) findMatch(query string, start viewerTextPosition, direction int, matchCase bool) (viewerTextPosition, bool, bool) {
	if !matchCase {
		query = strings.ToLower(query)
	}
	total := v.TotalLines()
	line := start.line
	wrapped := false
	for visited := 0; visited <= total; visited++ {
		src := v.searchLine(line, matchCase)
		offset := -1
		if direction > 0 {
			begin := 0
			if visited == 0 {
				begin = viewerRuneOffset(src, start.col)
			}
			if n := strings.Index(src[begin:], query); n >= 0 {
				offset = begin + n
			}
		} else if visited != 0 || start.col >= 0 {
			end := len(src)
			if visited == 0 {
				// Restrict where the match starts, allowing overlapping matches.
				end = min(end, viewerRuneOffset(src, start.col)+len(query))
			}
			offset = strings.LastIndex(src[:end], query)
		}
		if offset >= 0 {
			return viewerTextPosition{line: line, col: utf8.RuneCountInString(src[:offset])}, wrapped, true
		}
		line += direction
		if line >= total {
			line = 0
			wrapped = true
		} else if line < 0 {
			line = total - 1
			wrapped = true
		}
	}
	return viewerTextPosition{}, false, false
}

// Sparse byte/rune/display positions let horizontal scrolling seek into long
// physical lines without measuring the preceding megabyte on every keypress.
type viewerLineCheckpoint struct {
	bytes, runes, column int
}

func (v *viewerLines) measureLine(i int, text string) int {
	const stride = 2048
	if len(text) <= stride {
		return viewerTextGridLineWidth(text)
	}
	var points []viewerLineCheckpoint
	column, runes, next := 0, 0, stride
	for offset, r := range text {
		if offset >= next {
			points = append(points, viewerLineCheckpoint{bytes: offset, runes: runes, column: column})
			next = offset + stride
		}
		column += viewerRuneCells(r, column)
		runes++
	}
	if len(points) > 0 {
		if v.checkpoints == nil {
			v.checkpoints = make(map[int][]viewerLineCheckpoint)
		}
		v.checkpoints[i] = points
	}
	return column
}

// visibleLine maps only visible cells. Tabs and clipped wide runes render
// as spaces while retaining their physical-line selection offsets.
func (v *viewerLines) visibleLine(line, start, width int) viewerVisibleLine {
	row := viewerVisibleLine{line: line, startCol: start}
	var rendered strings.Builder
	text := v.lineAt(line)
	col, logical := 0, 0
	if points := v.checkpoints[line]; len(points) > 0 {
		n := sort.Search(len(points), func(i int) bool { return points[i].column > start })
		if n > 0 {
			point := points[n-1]
			text = text[point.bytes:]
			col, logical = point.column, point.runes
		}
	}
	end := start + width
	for _, r := range text {
		cells := viewerRuneCells(r, col)
		next := col + cells
		if next > start && col < end {
			if len(row.cellToCol) == 0 {
				row.cellToCol = append(row.cellToCol, logical)
			}
			n := min(next, end) - max(col, start)
			if r == '\t' || n < cells {
				for range n {
					rendered.WriteByte(' ')
				}
			} else {
				rendered.WriteRune(r)
			}
			for c := max(col, start) + 1; c <= min(next, end); c++ {
				pos := logical
				if c == next {
					pos++
				}
				row.cellToCol = append(row.cellToCol, pos)
			}
		}
		logical++
		col = next
		if col >= end {
			break
		}
	}
	if len(row.cellToCol) == 0 {
		row.cellToCol = []int{logical}
	}
	row.displayLen = len(row.cellToCol) - 1
	row.text = rendered.String()
	return row
}
