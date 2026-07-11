package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/ui"
)

type droppedJobManager interface {
	EnqueueCopyWithResolver([]string, string, jobs.ConflictResolver) *jobs.Job
	EnqueueCopyWithOptions([]string, string, jobs.ConflictResolver, jobs.TransferOptions) *jobs.Job
	EnqueueMoveWithResolver([]string, string, jobs.ConflictResolver) *jobs.Job
}

func (fm *FileManager) setupDropHandler() {
	if fm.window == nil {
		debugPrint("FileManager: Drop handler skipped window=nil")
		return
	}
	debugPrint("FileManager: Drop handler registered")
	fm.window.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		debugPrint("FileManager: Drop callback pos=%.1f,%.1f uri_count=%d", pos.X, pos.Y, len(uris))
		for i, uri := range uris {
			if uri == nil {
				debugPrint("FileManager: Drop uri[%d]=<nil>", i)
				continue
			}
			debugPrint("FileManager: Drop uri[%d] scheme=%s authority=%s path=%s raw=%s", i, uri.Scheme(), uri.Authority(), uri.Path(), uri.String())
		}
		fyne.Do(func() {
			debugPrint("FileManager: Drop UI dispatch start uri_count=%d", len(uris))
			fm.handleDroppedURIs(uris)
		})
	})
}

func (fm *FileManager) handleDroppedURIs(uris []fyne.URI) {
	debugPrint("FileManager: Drop handling start current=%s uri_count=%d", fm.currentPath, len(uris))
	dest, err := dropDestination(fm.currentPath)
	if err != nil {
		debugPrint("FileManager: Drop rejected: %v", err)
		fm.ShowMessageDialog("Drop", err.Error())
		fm.FocusFileList()
		return
	}
	debugPrint("FileManager: Drop destination=%s", dest)

	paths, err := droppedURIPaths(uris)
	if err != nil {
		debugPrint("FileManager: Drop rejected: %v", err)
		fm.ShowMessageDialog("Drop", err.Error())
		fm.FocusFileList()
		return
	}
	debugPrint("FileManager: Drop accepted sources=%d", len(paths))
	for i, p := range paths {
		debugPrint("FileManager: Drop source[%d]=%s", i, p)
	}

	fm.showDropActionDialog(paths, dest)
}

func dropDestination(currentPath string) (string, error) {
	if strings.TrimSpace(currentPath) == "" {
		return "", errors.New("Cannot drop files because the current directory is unknown.")
	}
	if fileinfo.IsArchivePath(currentPath) {
		return "", errors.New("Dropping files into archive views is not supported yet.")
	}

	vfs, parsed, err := fileinfo.ResolveRead(currentPath)
	if err != nil {
		return "", fmt.Errorf("Cannot use this directory as a drop destination: %w", err)
	}
	defer fileinfo.CloseVFS(vfs)
	debugPrint("FileManager: Drop destination resolved scheme=%s provider=%s native=%s display=%s", parsed.Scheme, parsed.Provider, parsed.Native, parsed.Display)
	if parsed.Scheme == fileinfo.SchemeArchive {
		return "", errors.New("Dropping files into archive views is not supported yet.")
	}
	if parsed.Provider != "local" {
		return "", errors.New("Dropping files into direct SMB views is not supported yet.")
	}

	dest := parsed.Native
	if strings.TrimSpace(dest) == "" {
		dest = currentPath
	}
	info, err := os.Stat(dest)
	if err != nil {
		return "", fmt.Errorf("Cannot use this directory as a drop destination: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Drop destination is not a directory: %s", dest)
	}
	return dest, nil
}

func droppedURIPaths(uris []fyne.URI) ([]string, error) {
	if len(uris) == 0 {
		return nil, errors.New("No dropped files were received.")
	}

	seen := make(map[string]bool, len(uris))
	paths := make([]string, 0, len(uris))
	for _, uri := range uris {
		if uri == nil {
			return nil, errors.New("A dropped item was empty.")
		}
		if strings.ToLower(uri.Scheme()) != "file" {
			return nil, fmt.Errorf("Only local files can be dropped. Unsupported URI: %s", uri.String())
		}
		if uri.Authority() != "" {
			return nil, fmt.Errorf("Only local files can be dropped. Unsupported URI: %s", uri.String())
		}

		p := uri.Path()
		if runtime.GOOS == "windows" {
			p = filepath.FromSlash(p)
		}
		p = filepath.Clean(p)
		if !filepath.IsAbs(p) {
			return nil, fmt.Errorf("Dropped file path is not absolute: %s", uri.String())
		}
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("Cannot access dropped file %s: %w", p, err)
		}

		key := normalizeComparablePath(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil, errors.New("No usable dropped files were received.")
	}
	return paths, nil
}

func (fm *FileManager) showDropActionDialog(paths []string, dest string) {
	debugPrint("FileManager: Drop dialog showing sources=%d dest=%s", len(paths), dest)
	var d *dialog.CustomDialog
	closeDialog := func() {
		debugPrint("FileManager: Drop dialog closed")
		if d != nil {
			d.Hide()
		}
		fm.FocusFileList()
	}
	queue := func(op ui.Operation) {
		debugPrint("FileManager: Drop action=%s requested sources=%d dest=%s", string(op), len(paths), dest)
		closeDialog()
		if op == ui.OpMove {
			paths = droppedMoveSources(paths, dest)
			if len(paths) == 0 {
				debugPrint("FileManager: Drop move skipped same-directory sources")
				fm.ShowMessageDialog("Move", "Dropped item(s) are already in this directory.")
				return
			}
		}
		enqueueDroppedTransfer(fm.jobManager(), op, paths, dest, fm.conflictResolver(), jobs.TransferOptions{PreserveTimestamps: fm.config.UI.Copy.PreserveTimestamps})
		debugPrint("FileManager: Drop queued action=%s sources=%d dest=%s", string(op), len(paths), dest)
	}

	summary := widget.NewLabel(dropSummary(paths, dest))
	summary.Wrapping = fyne.TextWrapWord
	targets := widget.NewList(
		func() int { return len(paths) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(filepath.Base(paths[i]))
		},
	)
	targetsScroll := container.NewScroll(targets)
	targetsScroll.SetMinSize(fyne.NewSize(520, 150))

	content := container.NewVBox(
		summary,
		targetsScroll,
		ui.DialogButtonBar(
			ui.DialogAuxButton("Copy", theme.ContentCopyIcon(), func() { queue(ui.OpCopy) }),
			ui.DialogAuxButton("Move", theme.ContentCutIcon(), func() { queue(ui.OpMove) }),
			ui.DialogCancelButton("Cancel", closeDialog),
		),
	)

	d = dialog.NewCustomWithoutButtons("Dropped files", content, fm.window)
	d.Show()
	d.Resize(fyne.NewSize(580, 300))
}

func dropSummary(paths []string, dest string) string {
	return fmt.Sprintf("Drop %d item(s) into:\n%s", len(paths), dest)
}

func droppedMoveSources(paths []string, dest string) []string {
	filtered := make([]string, 0, len(paths))
	for _, p := range paths {
		if !sameDirectoryPath(filepath.Dir(p), dest) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func enqueueDroppedTransfer(mgr droppedJobManager, op ui.Operation, sources []string, dest string, resolver jobs.ConflictResolver, options jobs.TransferOptions) *jobs.Job {
	if mgr == nil || len(sources) == 0 {
		return nil
	}
	if op == ui.OpMove {
		return mgr.EnqueueMoveWithResolver(sources, dest, resolver)
	}
	return mgr.EnqueueCopyWithOptions(sources, dest, resolver, options)
}
