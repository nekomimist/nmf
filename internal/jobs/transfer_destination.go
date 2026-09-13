package jobs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"

	"nmf/internal/fileinfo"
)

// SMB rename does not replace existing paths. Keep the previous destination
// until publication succeeds, and retain its backup if restoration fails.
func replaceSMBPath(ops fileinfo.SMBPathOps, tmp, dst executionPath, overwrite bool) error {
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
			if restoreErr := ops.Rename(backup.path, dst.path); restoreErr != nil {
				return fmt.Errorf("previous destination retained at %s: %w", backup.displayPath(), errors.Join(publishErr, restoreErr))
			}
			return publishErr
		}
		if err := ops.Remove(backup.path); err != nil {
			return fmt.Errorf("replacement succeeded; previous destination retained at %s: %w", backup.displayPath(), err)
		}
		return nil
	}
	return fmt.Errorf("could not preserve existing destination %s: %w", dst.displayPath(), err)
}

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
		defer removePath(execCtx, tmp)
		return replacePath(j, execCtx, tmp, dst, true)
	}
	return fmt.Errorf("could not create temporary symlink in %s", dirPath(dst).displayPath())
}
