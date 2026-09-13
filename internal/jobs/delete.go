package jobs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"nmf/internal/fileinfo"
)

var errUnsafeDeleteTarget = errors.New("unsafe delete target")

var trashPath = fileinfo.TrashPath

func deletePermanentPath(j *Job, execCtx *executionContext, src string) error {
	srcPath, err := resolveExecutionPath(src)
	if err != nil {
		return wrapPath(src, err)
	}
	if err := validateDeleteTarget(srcPath); err != nil {
		return wrapPath(srcPath.displayPath(), err)
	}
	return deletePermanentResolved(j, execCtx, srcPath)
}

func topLevelDirectoryResult(execCtx *executionContext, src string) (Result, bool) {
	path, err := resolveExecutionPath(src)
	if err != nil {
		return Result{}, false
	}
	info, err := lstatPath(execCtx, path)
	if err != nil || !info.IsDir() {
		return Result{}, false
	}
	return Result{Source: path.displayPath(), SourceIsDir: true}, true
}

func validateDeleteTarget(p executionPath) error {
	switch p.backend {
	case backendArchive:
		return fmt.Errorf("%w: archive paths are read-only", errUnsafeDeleteTarget)
	case backendSMB:
		clean := normalizeSMBExecutionPath(p.path)
		if clean == "/" || clean == "." || clean == "" {
			return fmt.Errorf("%w: refusing to delete SMB share root", errUnsafeDeleteTarget)
		}
	default:
		clean := filepath.Clean(p.path)
		if clean == "." || clean == "" {
			return fmt.Errorf("%w: refusing to delete empty or relative root path", errUnsafeDeleteTarget)
		}
		volume := filepath.VolumeName(clean)
		root := string(os.PathSeparator)
		if volume != "" {
			root = volume + string(os.PathSeparator)
		}
		if clean == root {
			return fmt.Errorf("%w: refusing to delete filesystem root", errUnsafeDeleteTarget)
		}
	}
	return nil
}

func deletePermanentResolved(j *Job, execCtx *executionContext, src executionPath) error {
	if canceled(j) {
		return errCanceled
	}

	fi, err := lstatPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}

	if fi.IsDir() && !isLinkLikeForTraversal(execCtx, src, fi) {
		entries, err := readDir(execCtx, src)
		if err != nil {
			return wrapPath(src.displayPath(), err)
		}
		for _, e := range entries {
			if canceled(j) {
				return errCanceled
			}
			child := joinPath(src, e.Name())
			if err := deletePermanentResolved(j, execCtx, child); err != nil {
				return err
			}
		}
	}

	if canceled(j) {
		return errCanceled
	}
	dbg("job %d: permanent delete %s", j.ID, src.displayPath())
	if err := removePath(execCtx, src); err != nil {
		return wrapPath(src.displayPath(), err)
	}
	return nil
}
