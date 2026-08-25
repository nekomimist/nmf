package main

import (
	"context"
	"time"

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

func (fm *FileManager) beginShortcutOpen() (uint64, context.Context) {
	fm.shortcutMu.Lock()
	defer fm.shortcutMu.Unlock()
	if fm.shortcutOpenCancel != nil {
		fm.shortcutOpenCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	fm.nextShortcutOpenID++
	fm.activeShortcutOpenID = fm.nextShortcutOpenID
	fm.shortcutOpenCancel = cancel
	return fm.activeShortcutOpenID, ctx
}

func (fm *FileManager) finishShortcutOpen(id uint64) bool {
	fm.shortcutMu.Lock()
	defer fm.shortcutMu.Unlock()
	if id == 0 || fm.activeShortcutOpenID != id {
		return false
	}
	fm.activeShortcutOpenID = 0
	if fm.shortcutOpenCancel != nil {
		fm.shortcutOpenCancel()
		fm.shortcutOpenCancel = nil
	}
	return true
}

// invalidateShortcutOpen cancels the matching shortcut operation. An id of
// zero invalidates whichever operation is active.
func (fm *FileManager) invalidateShortcutOpen(id uint64) bool {
	if fm == nil {
		return false
	}
	fm.shortcutMu.Lock()
	defer fm.shortcutMu.Unlock()
	if fm.activeShortcutOpenID == 0 || (id != 0 && fm.activeShortcutOpenID != id) {
		return false
	}
	fm.activeShortcutOpenID = 0
	if fm.shortcutOpenCancel != nil {
		fm.shortcutOpenCancel()
		fm.shortcutOpenCancel = nil
	}
	return true
}

func (fm *FileManager) cancelActiveShortcutOpen() {
	if !fm.invalidateShortcutOpen(0) {
		return
	}
	if fm.busy != nil {
		fm.busy.End()
	}
	fm.focusFileList("shortcut-open-cancel")
	debugPrint("FileManager: Shortcut open canceled")
}

func (fm *FileManager) openShortcut(path string) {
	id, ctx := fm.beginShortcutOpen()
	debugPrint("FileManager: Shortcut open start id=%d path=%s", id, path)
	if fm.busy != nil {
		// This callback intentionally cancels the active generation instead of
		// capturing id. BusyController preserves its original callback when an
		// already-active guard is updated.
		fm.busy.Begin("Resolving shortcut...", fm.cancelActiveShortcutOpen)
	}

	resolver := fm.shortcutResolver()
	opener := fm.shortcutDefaultOpener()
	go func() {
		started := time.Now()
		result := runShortcutOpen(ctx, path, resolver, opener)
		debugPrint("FileManager: Shortcut open worker id=%d elapsed=%s path=%s resolveErr=%v openErr=%v", id, time.Since(started), path, result.resolveErr, result.openErr)
		if ctx.Err() != nil {
			return
		}
		fyne.Do(func() {
			if fm.isWindowClosed() || !fm.finishShortcutOpen(id) {
				return
			}
			// End the shortcut guard before LoadDirectory begins its own busy
			// generation with the directory-load cancellation callback.
			if fm.busy != nil {
				fm.busy.End()
			}

			if result.navigate {
				fm.LoadDirectory(result.directory)
				return
			}
			if !result.delegated {
				debugPrint("FileManager: Shortcut target unavailable path=%s err=%v", path, result.resolveErr)
				fm.ShowMessageDialog("ショートカットを開けませんでした", result.resolveErr.Error())
				return
			}
			if result.resolveErr != nil {
				debugPrint("FileManager: Shortcut resolution delegated path=%s err=%v", path, result.resolveErr)
			}
			if result.openErr != nil {
				debugPrint("FileManager: Failed to open shortcut path=%s err=%v", path, result.openErr)
				fm.resetKeyStateAfterExternalOpen("open-shortcut-error")
				fm.ShowMessageDialog("ファイルを開けませんでした", result.openErr.Error())
			}
		})
	}()
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
