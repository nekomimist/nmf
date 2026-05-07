package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"fyne.io/fyne/v2/driver"

	"nmf/internal/fileinfo"
	"nmf/internal/shellmenu"
)

func (fm *FileManager) StartFileDrag(dragged fileinfo.FileInfo) {
	if runtime.GOOS != "windows" {
		debugPrint("FileManager: File drag source unsupported on %s", runtime.GOOS)
		return
	}

	paths, err := fm.collectDragSourcePaths(dragged)
	if err != nil {
		debugPrint("FileManager: File drag source rejected: %v", err)
		return
	}
	if len(paths) == 0 {
		debugPrint("FileManager: File drag source rejected: no paths")
		return
	}
	for i, path := range paths {
		debugPrint("FileManager: File drag source[%d]=%s", i, path)
	}

	nativeWindow, ok := fm.window.(driver.NativeWindow)
	if !ok {
		debugPrint("FileManager: Native window context unavailable for file drag")
		return
	}

	debugPrint("FileManager: File drag source starting sources=%d", len(paths))
	var dragErr error
	nativeWindow.RunNative(func(context any) {
		winCtx, ok := context.(driver.WindowsWindowContext)
		if !ok || winCtx.HWND == 0 {
			debugPrint("FileManager: File drag native context unavailable type=%T", context)
			dragErr = shellmenu.ErrUnsupported
			return
		}
		debugPrint("FileManager: File drag native hwnd=%d", winCtx.HWND)
		dragErr = shellmenu.StartFileDrag(winCtx.HWND, paths)
	})
	resetNativeDragState(fm.window)
	if dragErr != nil && !errors.Is(dragErr, shellmenu.ErrUnsupported) {
		debugPrint("FileManager: File drag source failed: %v", dragErr)
		return
	}
	if errors.Is(dragErr, shellmenu.ErrUnsupported) {
		debugPrint("FileManager: File drag source unsupported by native window")
		return
	}
	debugPrint("FileManager: File drag source finished sources=%d", len(paths))
}

func (fm *FileManager) collectDragSourcePaths(dragged fileinfo.FileInfo) ([]string, error) {
	candidates := fm.selectedDragCandidates()
	if len(candidates) == 0 {
		candidates = []fileinfo.FileInfo{dragged}
	}

	paths := make([]string, 0, len(candidates))
	for _, fi := range candidates {
		path, err := dragSourceNativePath(fi)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func (fm *FileManager) selectedDragCandidates() []fileinfo.FileInfo {
	selected := make([]fileinfo.FileInfo, 0)
	for p, ok := range fm.selectedFiles {
		if !ok {
			continue
		}
		for _, fi := range fm.files {
			if fi.Path == p {
				selected = append(selected, fi)
				break
			}
		}
	}
	return selected
}

func dragSourceNativePath(fi fileinfo.FileInfo) (string, error) {
	if fi.Name == "" || fi.Name == ".." {
		return "", fmt.Errorf("invalid drag source: %s", fi.Name)
	}
	if fi.Status == fileinfo.StatusDeleted {
		return "", fmt.Errorf("deleted item cannot be dragged: %s", fi.Path)
	}
	if fileinfo.IsArchivePath(fi.Path) {
		return "", fmt.Errorf("archive item cannot be dragged: %s", fi.Path)
	}

	_, parsed, err := fileinfo.ResolveRead(fi.Path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve drag source %s: %w", fi.Path, err)
	}
	if parsed.Provider != "local" {
		return "", fmt.Errorf("direct SMB item cannot be dragged: %s", fi.Path)
	}

	path := parsed.Native
	if strings.TrimSpace(path) == "" {
		path = fi.Path
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("cannot access drag source %s: %w", path, err)
	}
	return path, nil
}
