package jobs

import (
	"errors"
	"os"

	"nmf/internal/fileinfo"
)

// openTransferDestination resolves and validates a copy/move destination and
// returns the execution context every source of that job shares. The caller
// owns the context and must close it. Sharing one context per job is what keeps
// a remote backend dialed once instead of once per source, so this preamble
// runs before any transfer rather than inside the per-source loop.
func openTransferDestination(destDir string) (executionPath, *executionContext, error) {
	destPath, err := resolveExecutionPath(destDir)
	if err != nil {
		return executionPath{}, nil, wrapPath(destDir, err)
	}
	execCtx := newExecutionContext()
	if err := validateDestinationDirectory(execCtx, destPath); err != nil {
		if closeErr := execCtx.close(); closeErr != nil {
			dbg("execution context close error: %v", closeErr)
		}
		return executionPath{}, nil, err
	}
	return destPath, execCtx, nil
}

// transferSource copies or moves one top-level source into an already-validated
// destination, reusing the job's shared execution context.
func transferSource(j *Job, execCtx *executionContext, src string, destDir executionPath) error {
	srcPath, err := resolveExecutionPath(src)
	if err != nil {
		return wrapPath(src, err)
	}
	result := Result{Source: srcPath.displayPath()}
	if err := copyOrMovePathResolved(j, execCtx, srcPath, destDir, &result); err != nil {
		return err
	}
	if result.SourceIsDir && result.Destination != "" {
		j.addResult(result)
	}
	return nil
}

func validateDestinationDirectory(execCtx *executionContext, dest executionPath) error {
	if dest.backend == backendArchive {
		return wrapPath(dest.displayPath(), errors.New("archive destinations are read-only"))
	}
	info, err := statPath(execCtx, dest)
	if err != nil {
		return wrapPath(dest.displayPath(), err)
	}
	if !info.IsDir() {
		return wrapPath(dest.displayPath(), errors.New("destination is not a directory"))
	}
	return nil
}

func copyOrMovePathResolved(j *Job, execCtx *executionContext, src executionPath, destDir executionPath, result *Result) error {
	if destDir.backend == backendArchive {
		return wrapPath(destDir.displayPath(), errors.New("archive destinations are read-only"))
	}
	if j.Type == TypeMove && src.backend == backendArchive {
		return wrapPath(src.displayPath(), errors.New("cannot move out of an archive; use copy instead"))
	}

	fi, err := lstatPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	base := baseName(src)
	if err := validateArchiveSourceName(src, base); err != nil {
		return wrapPath(src.displayPath(), err)
	}
	dst := joinPath(destDir, base)
	dst, skipped, overwrite, err := resolveDestinationConflict(j, execCtx, src, dst, fi)
	if err != nil {
		return err
	}
	if skipped {
		return errSkipped
	}
	if result != nil {
		result.Source = src.displayPath()
		result.Destination = dst.displayPath()
		result.SourceIsDir = fi.IsDir()
	}
	if sameExecutionPath(src, dst) {
		dbg("job %d: source and destination are identical; no-op %s", j.ID, src.displayPath())
		if result != nil {
			result.Destination = ""
		}
		return nil
	}

	return transferResolvedPath(j, execCtx, src, dst, fi, overwrite)
}

// transferResolvedPath runs only after conflict resolution and same-path checks.
func transferResolvedPath(j *Job, execCtx *executionContext, src, dst executionPath, fi os.FileInfo, overwrite bool) error {
	if fi.IsDir() && !isLinkLikeForTraversal(execCtx, src, fi) {
		if err := validateDirectoryTransfer(src, dst, j.Type); err != nil {
			return err
		}
	}
	moved, err := tryFastMovePath(j, execCtx, src, dst, fi, overwrite)
	if err != nil || moved {
		return err
	}

	target, isLink, err := linkTargetForCopy(execCtx, src, fi)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	if isLink {
		return transferSymlink(j, execCtx, src, dst, target, overwrite)
	}
	if fi.IsDir() {
		return transferDirectory(j, execCtx, src, dst, fi)
	}
	return transferFile(j, execCtx, src, dst, fi, overwrite)
}

func transferSymlink(j *Job, execCtx *executionContext, src, dst executionPath, target string, overwrite bool) error {
	if err := copySymlink(execCtx, j, target, dst, overwrite); err != nil {
		return wrapPath(dst.displayPath(), err)
	}
	if j.Type == TypeMove {
		dbg("job %d: unlink %s", j.ID, src.displayPath())
		if err := removePath(execCtx, src); err != nil {
			return wrapPath(src.displayPath(), err)
		}
	}
	return nil
}

func transferDirectory(j *Job, execCtx *executionContext, src, dst executionPath, fi os.FileInfo) error {
	dbg("job %d: mkdir %s (mode=%v)", j.ID, dst.displayPath(), fi.Mode())
	if err := ensureDir(execCtx, dst, fi.Mode()); err != nil {
		return wrapPath(dst.displayPath(), err)
	}
	if err := transferDirectoryChildren(j, execCtx, src, dst); err != nil {
		return err
	}
	// Finish only when every child transferred. A skipped child keeps its
	// source directory, and timestamp preservation waits for a complete copy.
	if shouldPreserveTimestamps(j) {
		if err := chtimesPath(execCtx, dst, fi.ModTime(), fi.ModTime()); err != nil {
			return wrapPath(dst.displayPath(), err)
		}
	}
	return removeTransferredSource(j, execCtx, src, "rmdir")
}

func transferDirectoryChildren(j *Job, execCtx *executionContext, src, dst executionPath) error {
	entries, err := readDir(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	skippedChild := false
	for _, e := range entries {
		if canceled(j) {
			return errCanceled
		}
		if err := validateArchiveSourceName(src, e.Name()); err != nil {
			return wrapPath(src.displayPath(), err)
		}
		child := joinPath(src, e.Name())
		dbg("job %d: recurse %s -> %s", j.ID, child.displayPath(), dst.displayPath())
		err := copyOrMovePathResolved(j, execCtx, child, dst, nil)
		if errors.Is(err, errSkipped) {
			skippedChild = true
			continue
		}
		if err != nil {
			return err
		}
	}
	if skippedChild {
		return errSkipped
	}
	return nil
}

func transferFile(j *Job, execCtx *executionContext, src, dst executionPath, fi os.FileInfo, overwrite bool) error {
	dbg("job %d: file %s -> %s", j.ID, src.displayPath(), dst.displayPath())
	if err := copyFileWithCancel(j, execCtx, src, dst, fi, overwrite); err != nil {
		return err
	}
	return removeTransferredSource(j, execCtx, src, "remove")
}

func removeTransferredSource(j *Job, execCtx *executionContext, src executionPath, operation string) error {
	if j.Type != TypeMove {
		return nil
	}
	if canceled(j) {
		return errCanceled
	}
	dbg("job %d: %s %s", j.ID, operation, src.displayPath())
	if err := removePath(execCtx, src); err != nil {
		return wrapPath(src.displayPath(), err)
	}
	return nil
}

func validateArchiveSourceName(src executionPath, name string) error {
	if src.backend != backendArchive {
		return nil
	}
	return fileinfo.ValidateArchiveEntryBaseName(name)
}

func shouldPreserveTimestamps(j *Job) bool {
	if j == nil {
		return false
	}
	return j.Type == TypeMove || j.Options.PreserveTimestamps
}

func tryFastMovePath(j *Job, execCtx *executionContext, src, dst executionPath, fi os.FileInfo, overwrite bool) (bool, error) {
	if j.Type != TypeMove {
		return false, nil
	}
	if canceled(j) {
		return false, errCanceled
	}
	if fi.IsDir() && !overwrite {
		exists, err := pathExists(execCtx, dst)
		if err != nil {
			return false, wrapPath(dst.displayPath(), err)
		}
		if exists {
			return false, nil
		}
	}
	if err := renamePath(execCtx, src, dst, overwrite); err == nil {
		dbg("job %d: rename %s -> %s", j.ID, src.displayPath(), dst.displayPath())
		return true, nil
	} else {
		dbg("job %d: rename fallback %s -> %s: %v", j.ID, src.displayPath(), dst.displayPath(), err)
	}
	return false, nil
}
