package ui

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
)

func TestFileViewerTextGridWrappedPageMovement(t *testing.T) {
	for _, tc := range []struct {
		name, text, before, after, end string
		cols                           int
	}{
		{"mixed lines", "abcdefghijklmnop\nZ\nY", "abcd\nefgh\nijkl", "ijkl\nmnop\nZ", "Y", 4},
		{"single long line", "abcdefghijklmnop", "abcd\nefgh\nijkl", "ijkl\nmnop", "mnop", 4},
		{"empty lines", "abcdefgh\n\nZ\nijklmnop", "abcd\nefgh\n", "\nZ\nijkl", "mnop", 4},
		{"wide runes", "ab日x文yZ\nq", "ab\n日x\n文y", "文y\nZ\nq", "q", 3},
		{"tab crossing a wrap", "\tabcdefgh\nZ", "   \n ab\ncde", "cde\nfgh\nZ", "Z", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			grid := newFileViewerTextGrid(tc.text, nil, nil, nil)
			grid.visibleRows, grid.visibleCols = 3, tc.cols
			grid.SetWrap(true)
			if got := grid.grid.Text(); got != tc.before {
				t.Fatalf("initial text = %q, want %q", got, tc.before)
			}
			grid.PageDown()
			if got := grid.grid.Text(); got != tc.after {
				t.Fatalf("PageDown text = %q, want %q", got, tc.after)
			}
			grid.PageUp()
			if got := grid.grid.Text(); got != tc.before {
				t.Fatalf("PageUp text = %q, want original %q", got, tc.before)
			}
			grid.MoveRows(1000)
			if got := grid.grid.Text(); got != tc.end {
				t.Fatalf("text at end = %q, want %q", got, tc.end)
			}
			if grid.CurrentLine() != grid.TotalLines() {
				t.Fatalf("CurrentLine = %d, want last physical line %d", grid.CurrentLine(), grid.TotalLines())
			}
			grid.PageDown()
			if got := grid.grid.Text(); got != tc.end {
				t.Fatalf("PageDown beyond end = %q, want %q", got, tc.end)
			}
			grid.MoveRows(-1000)
			grid.PageUp()
			if got := grid.grid.Text(); got != tc.before {
				t.Fatalf("text at start = %q, want %q", got, tc.before)
			}
		})
	}
}

func TestFileViewerTextGridPageMovementWithOneVisibleRow(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		name := "unwrapped"
		if wrapped {
			name = "wrapped"
		}
		t.Run(name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			grid := newFileViewerTextGrid("abcdefgh\n"+strings.Repeat("x\n", 30), nil, nil, nil)
			grid.visibleRows, grid.visibleCols = 1, 4
			grid.SetWrap(wrapped)
			grid.Home()
			grid.PageDown()
			wantText, wantLine := "x", 2
			if wrapped {
				wantText, wantLine = "efgh", 1
			}
			if got := grid.grid.Text(); got != wantText || grid.CurrentLine() != wantLine {
				t.Fatalf("PageDown = %q at line %d, want %q at line %d", got, grid.CurrentLine(), wantText, wantLine)
			}
			grid.PageUp()
			if got := grid.grid.Text(); got != "abcd" || grid.CurrentLine() != 1 {
				t.Fatalf("PageUp = %q at line %d, want abcd at line 1", got, grid.CurrentLine())
			}
		})
	}
}

func TestFileViewerTextGridWrappedPageUpAcrossPhysicalLines(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	grid := newFileViewerTextGrid("abcdefghijklmnop\nZ\nY", nil, nil, nil)
	grid.visibleRows, grid.visibleCols = 3, 4
	grid.SetWrap(true)
	grid.JumpToLine(3)
	grid.PageUp()
	if got := grid.grid.Text(); got != "mnop\nZ\nY" {
		t.Fatalf("PageUp text = %q, want last wrapped row of physical line 1", got)
	}
	grid.PageDown()
	if got := grid.grid.Text(); got != "Y" || grid.CurrentLine() != 3 {
		t.Fatalf("PageDown = %q at line %d, want Y at line 3", got, grid.CurrentLine())
	}
}

func TestFileViewerTextGridWrappedRowAndWheelMovement(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		cols       int
		rows       []string
	}{
		{"wide rune at boundary", "ab日x", 3, []string{"ab", "日x"}},
		{"narrow viewport", "日本x", 1, []string{"日", "本", "x"}},
		{"tab continuation", "a\tb", 3, []string{"a  ", " b"}},
		{"empty physical lines", "a\n\nb\n", 2, []string{"a", "", "b", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			grid := newFileViewerTextGrid(tc.text, nil, nil, nil)
			grid.visibleRows, grid.visibleCols = 1, tc.cols
			grid.SetWrap(true)
			for i, want := range tc.rows {
				if got := grid.grid.Text(); got != want {
					t.Fatalf("row %d = %q, want %q", i, got, want)
				}
				if i+1 < len(tc.rows) {
					grid.MoveRows(1)
				}
			}
			for i := len(tc.rows) - 2; i >= 0; i-- {
				grid.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: 1}})
				if got := grid.grid.Text(); got != tc.rows[i] {
					t.Fatalf("wheel back to row %d = %q, want %q", i, got, tc.rows[i])
				}
			}
			grid.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: -1}})
			if got := grid.grid.Text(); got != tc.rows[1] {
				t.Fatalf("wheel forward = %q, want %q", got, tc.rows[1])
			}
		})
	}
}

func TestFileViewerTextGridWrappedPositionAfterNavigation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	grid := newFileViewerTextGrid("abcdefghijklmnop\nqrstuvwxyz", nil, nil, nil)
	grid.visibleRows, grid.visibleCols = 3, 4
	grid.SetWrap(true)
	check := func(want string, line int) {
		t.Helper()
		if got := grid.grid.Text(); got != want || grid.CurrentLine() != line {
			t.Fatalf("text = %q at line %d, want %q at line %d", got, grid.CurrentLine(), want, line)
		}
	}
	grid.PageDown()
	grid.JumpToLine(1)
	check("abcd\nefgh\nijkl", 1)
	grid.PageDown()
	grid.Home()
	check("abcd\nefgh\nijkl", 1)
	grid.End()
	check("yz", 2)
	grid.JumpToLine(2)
	check("qrst\nuvwx\nyz", 2)
	grid.MoveRows(1)
	check("uvwx\nyz", 2)
	grid.SetWrap(false)
	check("qrst", 2)
	grid.SetWrap(true)
	check("qrst\nuvwx\nyz", 2)
}

func TestFileViewerTextGridWrappedResizePreservesContent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	grid := newFileViewerTextGrid("abcdefghijklmnopqrstuvwx", nil, nil, nil)
	grid.visibleRows, grid.visibleCols = 3, 4
	grid.SetWrap(true)
	grid.PageDown()
	cell := grid.textGridCellSize()
	grid.Resize(fyne.NewSize(6*cell.Width, 3*cell.Height))
	if got := grid.grid.Text(); got != "ghijkl\nmnopqr\nstuvwx" {
		t.Fatalf("text after width change = %q, want row containing old start at i", got)
	}
	grid.Resize(fyne.NewSize(6*cell.Width, cell.Height))
	if got := grid.grid.Text(); got != "ghijkl" {
		t.Fatalf("text after height change = %q, want same start", got)
	}
	grid.PageDown()
	if got := grid.grid.Text(); got != "mnopqr" {
		t.Fatalf("PageDown after resize = %q, want mnopqr", got)
	}
}

func TestFileViewerTextGridWrappedSearchRevealsMatch(t *testing.T) {
	for _, tc := range []struct {
		name, text, query, want string
		cols, line, col         int
	}{
		{"single line tail", "abcdefghijklmnop", "mnop", "mnop", 4, 1, 13},
		{"another line tail", "x\nabcdefghijklmnop", "mnop", "mnop", 4, 2, 13},
		{"across wrap", "abcdefghijklmnop", "ghij", "efgh\nijkl", 4, 1, 7},
		{"wide boundary", "ab日x文yZ", "文", "文y\nZ", 3, 1, 5},
		{"tab before match", "\tabcdefgh", "fgh", "fgh", 3, 1, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			grid := newFileViewerTextGrid(tc.text, nil, nil, nil)
			grid.visibleRows, grid.visibleCols = 2, tc.cols
			grid.SetWrap(true)
			result := grid.Find(tc.query, 1)
			if !result.Matched || result.Line != tc.line || result.Column != tc.col {
				t.Fatalf("Find = %+v, want match at %d:%d", result, tc.line, tc.col)
			}
			if got := grid.grid.Text(); got != tc.want {
				t.Fatalf("search text = %q, want %q", got, tc.want)
			}
			if grid.CurrentLine() != tc.line {
				t.Fatalf("CurrentLine = %d, want physical line %d", grid.CurrentLine(), tc.line)
			}
			styled := false
			for _, row := range grid.grid.Rows {
				for _, cell := range row.Cells {
					styled = styled || cell.Style != nil
				}
			}
			if !styled {
				t.Fatal("visible search match has no highlight")
			}
			grid.Find("missing", 1)
			if got := grid.grid.Text(); got != tc.want {
				t.Fatalf("failed search changed text to %q", got)
			}
			grid.Find("", 1)
			if got := grid.grid.Text(); got != tc.want {
				t.Fatalf("clearing search changed text to %q", got)
			}
		})
	}
}

func TestFileViewerTextGridWrappedSearchNextPrevious(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	grid := newFileViewerTextGrid("hit-----hit-----hit", nil, nil, nil)
	grid.visibleRows, grid.visibleCols = 2, 4
	grid.SetWrap(true)
	for _, tc := range []struct {
		direction, col int
		wrapped        bool
	}{
		{1, 1, false}, {1, 9, false}, {1, 17, false},
		{-1, 9, false}, {-1, 1, false}, {-1, 17, true},
	} {
		result := grid.Find("hit", tc.direction)
		if !result.Matched || result.Column != tc.col || result.Wrapped != tc.wrapped {
			t.Fatalf("Find(%d) = %+v, want column %d wrapped=%t", tc.direction, result, tc.col, tc.wrapped)
		}
		if got := grid.grid.Text(); !strings.HasPrefix(got, "hit") {
			t.Fatalf("Find(%d) text = %q, want visible hit", tc.direction, got)
		}
	}
}

func TestFileViewerTextGridSelectionAfterWrappedScroll(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	grid := newFileViewerTextGrid("a\t日本z\nlast", nil, nil, nil)
	grid.visibleRows, grid.visibleCols = 3, 3
	grid.cellSize = fyne.NewSize(10, 20)
	grid.SetWrap(true)
	grid.MoveRows(1)
	grid.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(30, 21)},
		Dragged:    fyne.NewDelta(30, 21),
	})
	grid.DragEnd()
	if got := grid.SelectedText(); got != "\t日本z" {
		t.Fatalf("SelectedText = %q, want original tab and wide runes", got)
	}
}

func TestFileViewerDialogWrappedPagingInEveryTextPane(t *testing.T) {
	for _, pane := range []string{"Text", "Markdown", "Hex"} {
		t.Run(pane, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			grid := newFileViewerTextGrid("abcdefghijklmnop\nZ", nil, nil, nil)
			grid.visibleRows, grid.visibleCols = 3, 4
			grid.SetWrap(true)
			d := &FileViewerDialog{activeName: pane, lineLabel: widget.NewLabel("")}
			switch pane {
			case "Text":
				d.textGrid = grid
			case "Markdown":
				d.mdGrid = grid
			case "Hex":
				d.hexGrid = grid
			}
			d.ViewerPageDown()
			if got := grid.grid.Text(); got != "ijkl\nmnop\nZ" {
				t.Fatalf("PageDown text = %q", got)
			}
			if got := d.lineLabel.Text; got != "line=1/2  wrap" {
				t.Fatalf("line status = %q, want physical line 1/2", got)
			}
			d.ViewerPageUp()
			if got := grid.grid.Text(); got != "abcd\nefgh\nijkl" {
				t.Fatalf("PageUp text = %q", got)
			}
		})
	}
}

func BenchmarkFileViewerTextGridWrappedPaging(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	grid := newFileViewerTextGrid(strings.Repeat("x", 1<<20), nil, nil, nil)
	grid.SetWrap(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		grid.PageDown()
		grid.PageUp()
	}
}

func TestViewerUnwrappedCellsMatchCompleteLineMap(t *testing.T) {
	text := strings.Repeat("a日\tß", 2300) + "end"
	content := viewerLines{lines: []string{text}}
	width := content.measureLine(0, text)
	ref := newViewerDisplayLineMap(text)
	for _, start := range []int{0, 1, 2, 5, 2039, 2048, 2053, 5000, width - 1, width, width + 1} {
		for _, cols := range []int{1, 2, 8, 80} {
			got := content.visibleLine(0, start, cols)
			want := ref.slice(0, start, cols)
			if got.text != want.text || !slices.Equal(got.cellToCol, want.cellToCol) || got.displayLen != want.displayLen {
				t.Fatalf("slice at %d width %d = %+v, want %+v", start, cols, got, want)
			}
		}
	}
}

func TestViewerSearchWrapFromFirstMatch(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	grid := newFileViewerTextGrid("日日日\n日日", nil, nil, nil)
	for _, tc := range []struct {
		direction, line, col int
		wrapped              bool
	}{
		{1, 1, 1, false}, {-1, 2, 1, true}, {-1, 1, 2, false}, {-1, 1, 1, false}, {1, 1, 2, false}, {1, 2, 1, false}, {1, 1, 1, true},
	} {
		got := grid.Find("日日", tc.direction)
		if !got.Matched || got.Line != tc.line || got.Column != tc.col || got.Wrapped != tc.wrapped {
			t.Fatalf("Find direction=%d: %+v, want %+v", tc.direction, got, tc)
		}
	}
}

func TestHexViewerSearchCopyAndJump(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	data := append(bytes.Repeat([]byte{0xff}, 16*120), []byte("last")...)
	grid := newViewerGrid(viewerLines{hexData: data}, nil, nil, nil)
	if got := grid.TotalLines(); got != 122 {
		t.Fatalf("lines=%d", got)
	}
	grid.JumpToLine(121)
	if !strings.Contains(grid.grid.Text(), "00000780") {
		t.Fatalf("jumped viewport=%q", grid.grid.Text())
	}
	if got := grid.Find("LAST", 1); !got.Matched || got.Line != 121 {
		t.Fatalf("Find LAST=%+v", got)
	}
	want := fileinfo.FormatHexDump(data)
	if n := grid.SelectAll(); n != len(want) {
		t.Fatalf("select all length=%d want %d", n, len(want))
	}
	if got := grid.SelectedText(); got != want {
		t.Fatal("selected hex dump differs from the classic representation")
	}
}

func BenchmarkFileViewerUnwrappedViewport(b *testing.B) {
	content := viewerLines{lines: []string{strings.Repeat("a", 1<<20)}}
	content.measureLine(0, content.lines[0])
	grid := fileViewerTextGrid{viewerLines: content}
	for _, tc := range []struct {
		name  string
		start int
	}{{"start", 0}, {"end", (1 << 20) - 100}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				grid.leftCol = tc.start + i%10
				grid.visible = grid.renderVisibleLines(30, 80)
			}
		})
	}
}

func BenchmarkFileViewerHexGrid(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	d := NewFileViewerDialog(&fileinfo.PreviewFile{Binary: true, Data: make([]byte, 1<<20)})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.createHexGrid()
	}
}
