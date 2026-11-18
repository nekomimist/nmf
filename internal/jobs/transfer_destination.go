package jobs

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"time"

	"nmf/internal/fileinfo"
)

func copySymlink(execCtx *executionContext, j *Job, target string, dst executionPath, overwrite bool) error {
	if !overwrite {
		return symlinkPath(execCtx, target, dst)
	}
	for range 100 {
		tmp := joinPath(dirPath(dst), ".nmf-"+rand.Text()+".part")
		if err := symlinkPath(execCtx, target, tmp); err != nil {
			if os.IsExist(err) {
				continue
			}
			return err
		}
		defer removeTransferTemp(execCtx, tmp)
		return replacePath(j, execCtx, tmp, dst, true)
	}
	return fmt.Errorf("could not create temporary symlink in %s", dirPath(dst).displayPath())
}

var errSMBBackupRetained = errors.New("previous destination retained")

// SMB rename does not replace existing paths. Keep the previous destination
// until publication succeeds, and retain its backup if restoration fails.

func replaceSMBPath(execCtx *executionContext, ops fileinfo.SMBPathOps, tmp, dst executionPath, overwrite bool) error {
	err := ops.Rename(tmp.path, dst.path)
	if err == nil || !overwrite {
		return err
	}
	info, statErr := ops.Lstat(dst.path)
	if statErr != nil || info.IsDir() {
		return err
	}
	for range 100 {
		backup := joinPath(dirPath(dst), ".nmf-"+rand.Text()+".backup")
		if backupErr := ops.Rename(dst.path, backup.path); backupErr != nil {
			if os.IsExist(backupErr) {
				continue
			}
			return errors.Join(err, backupErr)
		}
		if publishErr := ops.Rename(tmp.path, dst.path); publishErr != nil {
			restoreErr := withSMBCleanup(execCtx, dst, ops, func(recovery fileinfo.SMBPathOps) error {
				return recovery.Rename(backup.path, dst.path)
			})
			if restoreErr != nil {
				return fmt.Errorf("%w at %s: %w", errSMBBackupRetained, backup.displayPath(), errors.Join(publishErr, restoreErr))
			}
			return publishErr
		}
		removeErr := withSMBCleanup(execCtx, dst, ops, func(recovery fileinfo.SMBPathOps) error {
			return recovery.Remove(backup.path)
		})
		if removeErr != nil {
			return fmt.Errorf("replacement succeeded; %w at %s: %w", errSMBBackupRetained, backup.displayPath(), removeErr)
		}
		return nil
	}
	return fmt.Errorf("could not preserve existing destination %s: %w", dst.displayPath(), err)
}

func replacePath(j *Job, execCtx *executionContext, tmp executionPath, dst executionPath, overwrite bool) error {
	if dst.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if dst.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(dst)
		if err != nil {
			return err
		}
		return replaceSMBPath(execCtx, ops, tmp, dst, overwrite)
	}
	if dst.localRoot != nil {
		if overwrite {
			return dst.localRoot.Rename(tmp.path, dst.path)
		}
		// A hard link publishes the completed temporary file without replacing
		// a concurrent creator. Filesystems without links use exclusive copy.
		if err := dst.localRoot.Link(tmp.path, dst.path); err == nil {
			return removePath(execCtx, tmp)
		} else if os.IsExist(err) {
			return err
		}
		return publishExclusiveCopy(j, execCtx, tmp, dst)
	}
	if overwrite {
		// Native rename replaces files where supported. Never delete the old
		// destination after a failed rename: a second failure would lose it.
		return os.Rename(tmp.path, dst.path)
	}
	if err := fileinfo.RenameNativeNoReplace(tmp.path, dst.path); err == nil {
		return nil
	} else if os.IsExist(err) {
		return err
	}
	return publishExclusiveCopy(j, execCtx, tmp, dst)
}

// A canceled job has closed its SMB transport. Give removal of its temporary
// file or restoration of the previous destination one bounded recovery session.
func withSMBCleanup(execCtx *executionContext, p executionPath, ops fileinfo.SMBPathOps, cleanup func(fileinfo.SMBPathOps) error) error {
	if execCtx.request.Err() == nil || p.smbOpener == nil {
		err := cleanup(ops)
		if err == nil || execCtx.request.Err() == nil || p.smbOpener == nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(execCtx.request), 3*time.Second)
	defer cancel()
	session, err := p.smbOpener.OpenSession(ctx)
	if err != nil {
		return err
	}
	defer session.Close()
	return cleanup(session)
}

func removeTransferTemp(execCtx *executionContext, p executionPath) error {
	if p.backend != backendSMB || execCtx.request.Err() == nil {
		return removePath(execCtx, p)
	}
	ops, err := execCtx.smbOpsFor(p)
	if err != nil {
		return err
	}
	return withSMBCleanup(execCtx, p, ops, func(recovery fileinfo.SMBPathOps) error { return recovery.Remove(p.path) })
}
