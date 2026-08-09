package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowFileViewer opens the selected file in the built-in preview viewer.
func (fm *FileManager) ShowFileViewer() {
	totalStart := time.Now()
	currentIdx := fm.GetCurrentCursorIndex()
	files := fm.GetFiles()
	if currentIdx < 0 || currentIdx >= len(files) {
		return
	}

	file := files[currentIdx]
	if file.Name == ".." || file.IsDir {
		return
	}

	debugPrint("FileViewer: open-start path=%s", file.Path)
	viewerID, viewerCtx := fm.beginViewerLoad()
	fm.busy.Begin("Opening preview...", func() {
		if fm.invalidateViewerLoad(viewerID) {
			fm.busy.End()
			fm.FocusFileList()
		}
	})
	go func() {
		stepStart := time.Now()
		preview, err := fileinfo.ReadPreviewFileWithDebugContext(viewerCtx, file.Path, debugPrint)
		debugPrint("FileViewer: read-preview elapsed=%s path=%s err=%v", time.Since(stepStart), file.Path, err)
		fyne.Do(func() {
			if fm.isWindowClosed() || !fm.finishViewerLoad(viewerID) {
				return
			}
			fm.busy.End()
			if err != nil {
				debugPrint("FileViewer: open failed path=%s err=%v", file.Path, err)
				fm.ShowMessageDialog("Viewer failed", err.Error())
				fm.FocusFileList()
				return
			}

			dialog := ui.NewFileViewerDialog(preview, fm.keyManager)
			dialog.SetMaxSize(fm.config.UI.Viewer.MaxWidth, fm.config.UI.Viewer.MaxHeight)
			dialog.SetDefaultPane(fm.config.UI.Viewer.DefaultPane)
			dialog.SetDefaultWrap(fm.config.UI.Viewer.DefaultWrap)
			dialog.SetKeyBindings(fm.config.UI.KeyBindings)
			dialog.SetDebugPrint(debugPrint)
			if runtime := fm.runtime; runtime != nil {
				dialog.SetSearchState(runtime.viewerSearchText, runtime.viewerSearchMatchCase, func(text string, matchCase bool) {
					runtime.viewerSearchText = text
					runtime.viewerSearchMatchCase = matchCase
				})
			}
			session := &fileViewerSession{
				fm:         fm,
				dialog:     dialog,
				targetPath: file.Path,
			}
			dialog.SetFileActions(
				func() { session.moveCursor(-1) },
				func() { session.moveCursor(1) },
				session.toggleMark,
			)
			dialog.SetOnClosed(session.close)
			stepStart = time.Now()
			dialog.ShowDialog(fm.window)
			debugPrint("FileViewer: show-dialog elapsed=%s", time.Since(stepStart))
			debugPrint("FileViewer: open-ready elapsed=%s", time.Since(totalStart))
		})
	}()
}

type fileViewerSession struct {
	fm         *FileManager
	dialog     *ui.FileViewerDialog
	targetPath string
	closed     bool
}

func (s *fileViewerSession) moveCursor(delta int) {
	if s == nil || s.closed || s.fm == nil || delta == 0 {
		return
	}
	current := s.fm.GetCurrentCursorIndex()
	next := current + delta
	if next < 0 || next >= s.fm.FileCount() {
		return
	}
	s.fm.SetCursorByIndex(next)
	s.fm.RefreshCursor()
	s.loadCurrent()
}

func (s *fileViewerSession) toggleMark() {
	if s == nil || s.closed || s.fm == nil {
		return
	}
	current := s.fm.GetCurrentCursorIndex()
	file, ok := s.fm.FileAt(current)
	if !ok || file.Name == ".." || file.Status == fileinfo.StatusDeleted {
		return
	}

	selected := s.fm.GetSelectedFiles()
	s.fm.SetFileSelected(file.Path, !selected[file.Path])
	s.fm.RefreshFileList()

	if current >= s.fm.FileCount()-1 {
		return
	}
	s.fm.SetCursorByIndex(current + 1)
	s.fm.RefreshCursor()
	s.loadCurrent()
}

func (s *fileViewerSession) loadCurrent() {
	if s == nil || s.closed || s.fm == nil || s.dialog == nil {
		return
	}
	current := s.fm.GetCurrentCursorIndex()
	file, ok := s.fm.FileAt(current)
	if !ok {
		s.fm.invalidateViewerLoad(0)
		s.targetPath = ""
		s.dialog.ShowUnavailable("", "Preview unavailable: no directory entry selected.")
		return
	}

	if reason := fileViewerUnavailableReason(file); reason != "" {
		s.fm.invalidateViewerLoad(0)
		s.targetPath = file.Path
		s.dialog.ShowUnavailable(file.Path, reason)
		return
	}

	path := file.Path
	s.targetPath = path
	s.dialog.ShowLoading(path)
	debugPrint("FileViewer: switch-start path=%s", path)
	viewerID, viewerCtx := s.fm.beginViewerLoad()
	go func() {
		stepStart := time.Now()
		preview, err := fileinfo.ReadPreviewFileWithDebugContext(viewerCtx, path, debugPrint)
		debugPrint("FileViewer: switch-read elapsed=%s path=%s err=%v", time.Since(stepStart), path, err)
		fyne.Do(func() {
			if s.closed || s.fm.isWindowClosed() || !s.fm.finishViewerLoad(viewerID) {
				return
			}
			if !s.currentPathMatches(path) {
				s.loadCurrent()
				return
			}
			if err != nil {
				debugPrint("FileViewer: switch failed path=%s err=%v", path, err)
				s.dialog.ShowUnavailable(path, fmt.Sprintf("Preview unavailable: %v", err))
				return
			}
			s.dialog.UpdatePreview(preview)
			debugPrint("FileViewer: switch-ready path=%s", path)
		})
	}()
}

func (s *fileViewerSession) currentPathMatches(path string) bool {
	if s == nil || s.fm == nil || s.targetPath != path {
		return false
	}
	current := s.fm.GetCurrentCursorIndex()
	file, ok := s.fm.FileAt(current)
	return ok && file.Path == path && fileViewerUnavailableReason(file) == ""
}

func (s *fileViewerSession) close() {
	if s == nil || s.closed {
		return
	}
	s.closed = true
	if s.fm != nil {
		s.fm.invalidateViewerLoad(0)
	}
}

func fileViewerUnavailableReason(file fileinfo.FileInfo) string {
	switch {
	case file.Name == "..":
		return "Parent directory: preview unavailable."
	case file.Status == fileinfo.StatusDeleted:
		return "Deleted entry: preview unavailable."
	case file.IsDir:
		return "Directory: preview unavailable."
	default:
		return ""
	}
}
