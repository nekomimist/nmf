package main

import (
	"context"

	"fyne.io/fyne/v2"

	"nmf/internal/fileinfo"
)

type shortcutOpenResult struct {
	directory  string
	navigate   bool
	delegated  bool
	resolveErr error
	openErr    error
}

func (fm *FileManager) isShortcutNavigationCandidate(path string) bool {
	if fm != nil && fm.shortcutCandidateFn != nil {
		return fm.shortcutCandidateFn(path)
	}
	return fileinfo.IsShortcutNavigationCandidate(path)
}

func (fm *FileManager) shortcutResolver() func(context.Context, string) (string, bool, error) {
	if fm != nil && fm.shortcutResolverFn != nil {
		return fm.shortcutResolverFn
	}
	return fileinfo.ResolveShortcutNavigationDirContext
}

func (fm *FileManager) shortcutDefaultOpener() func(string) error {
	if fm != nil && fm.shortcutDefaultOpenFn != nil {
		return fm.shortcutDefaultOpenFn
	}
	return fileinfo.OpenWithDefaultApp
}

func (fm *FileManager) beginFileOperation() (uint64, context.Context) {
	fm.fileOperationMu.Lock()
	defer fm.fileOperationMu.Unlock()
	if fm.fileOperationCancel != nil {
		fm.fileOperationCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	fm.nextFileOperationID++
	fm.activeFileOperationID = fm.nextFileOperationID
	fm.fileOperationCancel = cancel
	return fm.activeFileOperationID, ctx
}

func (fm *FileManager) finishFileOperation(id uint64) bool {
	fm.fileOperationMu.Lock()
	defer fm.fileOperationMu.Unlock()
	if id == 0 || fm.activeFileOperationID != id {
		return false
	}
	fm.activeFileOperationID = 0
	if fm.fileOperationCancel != nil {
		fm.fileOperationCancel()
		fm.fileOperationCancel = nil
	}
	return true
}

// invalidateFileOperation cancels the matching file operation. An id of
// zero invalidates whichever operation is active.
func (fm *FileManager) invalidateFileOperation(id uint64) bool {
	if fm == nil {
		return false
	}
	fm.fileOperationMu.Lock()
	defer fm.fileOperationMu.Unlock()
	if fm.activeFileOperationID == 0 || (id != 0 && fm.activeFileOperationID != id) {
		return false
	}
	fm.activeFileOperationID = 0
	if fm.fileOperationCancel != nil {
		fm.fileOperationCancel()
		fm.fileOperationCancel = nil
	}
	return true
}

func (fm *FileManager) cancelActiveFileOperation() {
	if !fm.invalidateFileOperation(0) {
		return
	}
	if fm.busy != nil {
		fm.busy.End()
	}
	fm.focusFileList("file-operation-cancel")
	debugPrint("FileManager: File operation canceled")
}

// runFileOperation performs blocking opens and format probes on a worker. The
// returned closure belongs to the UI thread and only runs for the active request.
func (fm *FileManager) runFileOperation(message string, work func(context.Context) func()) {
	id, ctx := fm.beginFileOperation()
	if fm.busy != nil {
		fm.busy.Begin(message, fm.cancelActiveFileOperation)
	}
	go func() {
		apply := work(ctx)
		if ctx.Err() != nil {
			return
		}
		fyne.Do(func() {
			if fm.isWindowClosed() || !fm.finishFileOperation(id) {
				return
			}
			if fm.busy != nil {
				fm.busy.End()
			}
			if apply != nil {
				apply()
			}
		})
	}()
}

func (fm *FileManager) openFileAsync(path string, inspect bool) {
	fm.runFileOperation("Opening file...", func(ctx context.Context) func() {
		if inspect && fileinfo.IsSupportedArchiveContext(ctx, path) {
			return func() { fm.LoadDirectory(fileinfo.ArchiveRootPath(path)) }
		}
		if inspect && fm.isShortcutNavigationCandidate(path) {
			result := runShortcutOpen(ctx, path, fm.shortcutResolver(), fm.shortcutDefaultOpener())
			if result.navigate {
				return func() { fm.LoadDirectory(result.directory) }
			}
			if !result.delegated {
				return func() {
					fm.ShowMessageDialog("ショートカットを開けませんでした", result.resolveErr.Error())
				}
			}
			return fm.fileOpenCompletion(result.openErr)
		}
		return fm.fileOpenCompletion(fileinfo.OpenWithDefaultAppContext(ctx, path))
	})
}

func (fm *FileManager) fileOpenCompletion(err error) func() {
	if err == nil {
		return nil
	}
	return func() {
		debugPrint("FileManager: File open failed: %v", err)
		fm.resetKeyStateAfterExternalOpen("open-file-error")
		fm.ShowMessageDialog("ファイルを開けませんでした", err.Error())
	}
}

func runShortcutOpen(
	ctx context.Context,
	path string,
	resolver func(context.Context, string) (string, bool, error),
	opener func(string) error,
) shortcutOpenResult {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return shortcutOpenResult{resolveErr: err}
	}

	directory, ok, resolveErr := resolver(ctx, path)
	result := shortcutOpenResult{directory: directory, navigate: ok, resolveErr: resolveErr}
	if err := ctx.Err(); err != nil {
		result.navigate = false
		result.resolveErr = err
		return result
	}
	if resolveErr == nil && ok {
		return result
	}
	if resolveErr != nil && !fileinfo.IsShortcutNavigationReadError(resolveErr) {
		result.navigate = false
		return result
	}

	// Empty-target and shortcut-read failures retain the previous behavior of
	// asking Windows to open the .lnk itself. Keep that call on this worker too:
	// ShellExecuteW may resolve the same slow network target.
	result.navigate = false
	result.delegated = true
	if opener != nil {
		result.openErr = opener(path)
	}
	return result
}
