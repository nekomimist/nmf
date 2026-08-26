package main

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
)

func TestUpdateFileListRowPreservesCursorAnchorAndDiagnostics(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cfg := config.Default()
	theme := customtheme.NewCustomTheme(cfg, nil)
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{
			files: []fileinfo.FileInfo{
				{Name: "alpha.txt", Path: "/tmp/alpha.txt"},
				{Name: "beta.txt", Path: "/tmp/beta.txt"},
			},
			cursorPath: "/tmp/alpha.txt",
		}),
		cursorRefreshSeq: 1,
		config:           cfg,
		customTheme:      theme,
		windowActive:     true,
	}

	row, ok := fm.newFileListRow().(*ui.FileListRow)
	if !ok {
		t.Fatalf("file list template type = %T, want *ui.FileListRow", fm.newFileListRow())
	}
	fm.updateFileListRow(widget.ListItemID(0), row)

	if fm.fileListRows[0] != row {
		t.Fatalf("tracked row 0 = %p, want %p", fm.fileListRows[0], row)
	}
	if got := fm.fileListRowAssignments[row]; got.index != 0 || got.path != "/tmp/alpha.txt" {
		t.Fatalf("tracked assignment = %+v, want index 0 alpha path", got)
	}
	if fm.cursorAnchor.object != row || fm.cursorAnchor.path != "/tmp/alpha.txt" {
		t.Fatalf("cursor anchor = %+v, want alpha row %p", fm.cursorAnchor, row)
	}
	if fm.cursorItemUpdateSeq != fm.cursorRefreshSeq {
		t.Fatalf("cursorItemUpdateSeq = %d, want %d", fm.cursorItemUpdateSeq, fm.cursorRefreshSeq)
	}

	// Fyne recycles visible row templates. Once the same object is assigned to
	// a non-cursor item, it must no longer be used to position cursor menus.
	fm.updateFileListRow(widget.ListItemID(1), row)
	if fm.fileListRows[0] != nil || fm.fileListRows[1] != row {
		t.Fatalf("tracked rows after reuse = %+v, want only row 1", fm.fileListRows)
	}
	if got := fm.fileListRowAssignments[row]; got.index != 1 || got.path != "/tmp/beta.txt" {
		t.Fatalf("tracked assignment after reuse = %+v, want index 1 beta path", got)
	}
	if fm.cursorAnchor.object != nil || fm.cursorAnchor.path != "" {
		t.Fatalf("cursor anchor after row reuse = %+v, want cleared", fm.cursorAnchor)
	}
}
