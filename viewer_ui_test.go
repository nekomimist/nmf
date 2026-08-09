package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

func TestViewerLoadGenerationRejectsCanceledAndStaleResults(t *testing.T) {
	fm := &FileManager{}
	first, firstCtx := fm.beginViewerLoad()
	second, _ := fm.beginViewerLoad()
	if firstCtx.Err() == nil {
		t.Fatal("starting a replacement viewer load should cancel the previous context")
	}

	if fm.invalidateViewerLoad(first) {
		t.Fatal("stale viewer cancellation should not cancel the active request")
	}
	if fm.finishViewerLoad(first) {
		t.Fatal("stale viewer result should not finish")
	}
	if !fm.invalidateViewerLoad(second) {
		t.Fatal("active viewer cancellation should succeed")
	}
	if fm.finishViewerLoad(second) {
		t.Fatal("canceled viewer result should not finish")
	}
}

func TestFileViewerUnavailableReason(t *testing.T) {
	tests := []struct {
		name string
		file fileinfo.FileInfo
		want string
	}{
		{
			name: "parent",
			file: fileinfo.FileInfo{Name: "..", Path: "/tmp", IsDir: true},
			want: "Parent directory: preview unavailable.",
		},
		{
			name: "directory",
			file: fileinfo.FileInfo{Name: "docs", Path: "/tmp/docs", IsDir: true},
			want: "Directory: preview unavailable.",
		},
		{
			name: "deleted",
			file: fileinfo.FileInfo{Name: "gone.txt", Path: "/tmp/gone.txt", Status: fileinfo.StatusDeleted},
			want: "Deleted entry: preview unavailable.",
		},
		{
			name: "regular",
			file: fileinfo.FileInfo{Name: "note.txt", Path: "/tmp/note.txt"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileViewerUnavailableReason(tt.file); got != tt.want {
				t.Fatalf("fileViewerUnavailableReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileViewerSessionMovesCursorToUnavailableEntry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	files := []fileinfo.FileInfo{
		{Name: "note.txt", Path: "/tmp/note.txt"},
		{Name: "docs", Path: "/tmp/docs", IsDir: true},
	}
	fm := newFileViewerSessionTestManager(files)
	fm.SetCursorByIndex(0)
	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	fm.window = w

	d := ui.NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     files[0].Path,
		Data:     []byte("note"),
		Text:     "note",
		Encoding: "UTF-8",
	})
	d.ShowDialog(w)
	defer d.CancelDialog()
	session := &fileViewerSession{fm: fm, dialog: d, targetPath: files[0].Path}

	session.moveCursor(1)

	if got := fm.GetCurrentCursorIndex(); got != 1 {
		t.Fatalf("cursor index = %d, want 1", got)
	}
	if session.targetPath != files[1].Path {
		t.Fatalf("target path = %q, want %q", session.targetPath, files[1].Path)
	}
}

func TestFileViewerSessionToggleMarkAdvancesAndRefreshesPreviewState(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	files := []fileinfo.FileInfo{
		{Name: "note.txt", Path: "/tmp/note.txt"},
		{Name: "docs", Path: "/tmp/docs", IsDir: true},
	}
	fm := newFileViewerSessionTestManager(files)
	fm.SetCursorByIndex(0)
	w := test.NewWindow(widget.NewLabel("parent"))
	defer w.Close()
	fm.window = w

	d := ui.NewFileViewerDialog(&fileinfo.PreviewFile{
		Path:     files[0].Path,
		Data:     []byte("note"),
		Text:     "note",
		Encoding: "UTF-8",
	})
	d.ShowDialog(w)
	defer d.CancelDialog()
	session := &fileViewerSession{fm: fm, dialog: d, targetPath: files[0].Path}

	session.toggleMark()

	selected := fm.GetSelectedFiles()
	if !selected[files[0].Path] {
		t.Fatalf("selected files = %+v, want note.txt marked", selected)
	}
	if got := fm.GetCurrentCursorIndex(); got != 1 {
		t.Fatalf("cursor index = %d, want 1 after mark", got)
	}
	if session.targetPath != files[1].Path {
		t.Fatalf("target path = %q, want %q", session.targetPath, files[1].Path)
	}
}

func TestFileViewerSessionSkipsMarkForParentAndDeleted(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	files := []fileinfo.FileInfo{
		{Name: "..", Path: "/tmp", IsDir: true},
		{Name: "gone.txt", Path: "/tmp/gone.txt", Status: fileinfo.StatusDeleted},
	}
	fm := newFileViewerSessionTestManager(files)
	session := &fileViewerSession{fm: fm}

	for i := range files {
		fm.SetCursorByIndex(i)
		session.toggleMark()
	}

	selected := fm.GetSelectedFiles()
	if len(selected) != 0 {
		t.Fatalf("selected files = %+v, want none", selected)
	}
}

func TestFileViewerSessionRejectsStaleTargetPath(t *testing.T) {
	files := []fileinfo.FileInfo{
		{Name: "first.txt", Path: "/tmp/first.txt"},
		{Name: "second.txt", Path: "/tmp/second.txt"},
	}
	fm := newFileViewerSessionTestManager(files)
	fm.SetCursorByIndex(1)
	session := &fileViewerSession{fm: fm, targetPath: files[0].Path}

	if session.currentPathMatches(files[0].Path) {
		t.Fatal("stale preview path matched the current cursor")
	}
	session.targetPath = files[1].Path
	if !session.currentPathMatches(files[1].Path) {
		t.Fatal("current preview path did not match the current cursor")
	}
	updated := fm.GetFiles()
	updated[1].Status = fileinfo.StatusDeleted
	if err := fm.browserModel().ReplaceFiles(updated, false); err != nil {
		t.Fatalf("ReplaceFiles returned error: %v", err)
	}
	if session.currentPathMatches(files[1].Path) {
		t.Fatal("deleted current entry should reject an in-flight preview")
	}
}

func TestFileViewerSessionCloseCancelsActiveLoad(t *testing.T) {
	fm := &FileManager{}
	loadID, loadCtx := fm.beginViewerLoad()
	session := &fileViewerSession{fm: fm}

	session.close()

	if !session.closed {
		t.Fatal("session was not marked closed")
	}
	if loadCtx.Err() == nil {
		t.Fatal("active preview context was not canceled")
	}
	if fm.finishViewerLoad(loadID) {
		t.Fatal("closed session left its preview load active")
	}
}

func newFileViewerSessionTestManager(files []fileinfo.FileInfo) *FileManager {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{files: files}),
	}
	fm.fileList = widget.NewList(
		fm.FileCount,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(widget.ListItemID, fyne.CanvasObject) {},
	)
	return fm
}
