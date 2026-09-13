package jobs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"nmf/internal/fileinfo"
)

func isLinkLikeForTraversal(execCtx *executionContext, p executionPath, fi os.FileInfo) bool {
	if fi.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if !fileinfo.IsLinkModeCandidate(fi.Mode()) {
		return false
	}
	_, err := readlinkPath(execCtx, p)
	return err == nil
}

func linkTargetForCopy(execCtx *executionContext, p executionPath, fi os.FileInfo) (string, bool, error) {
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := readlinkPath(execCtx, p)
		return target, true, err
	}
	if !fileinfo.IsLinkModeCandidate(fi.Mode()) {
		return "", false, nil
	}
	target, err := readlinkPath(execCtx, p)
	if err != nil {
		return "", false, nil
	}
	return target, true, nil
}

func lstatPath(execCtx *executionContext, p executionPath) (os.FileInfo, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.Stat(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.Lstat(p.path)
	}
	if p.localRoot != nil {
		return p.localRoot.Lstat(p.path)
	}
	return os.Lstat(p.path)
}

func statPath(execCtx *executionContext, p executionPath) (os.FileInfo, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.Stat(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.Stat(p.path)
	}
	if p.localRoot != nil {
		return p.localRoot.Stat(p.path)
	}
	return os.Stat(p.path)
}

func readDir(execCtx *executionContext, p executionPath) ([]os.DirEntry, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.ReadDir(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.ReadDir(p.path)
	}
	if p.localRoot != nil {
		dir, err := p.localRoot.Open(p.path)
		if err != nil {
			return nil, err
		}
		defer dir.Close()
		return dir.ReadDir(-1)
	}
	return os.ReadDir(p.path)
}

func ensureDir(execCtx *executionContext, p executionPath, mode os.FileMode) error {
	if p.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	perm := mode.Perm()
	if perm == 0 {
		perm = 0755
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return err
		}
		return ops.MkdirAll(p.path, perm)
	}
	// MkdirAll preserves permissions of existing directories. In particular,
	// ensuring a file's parent must not reset a private directory to 0755.
	if p.localRoot != nil {
		return p.localRoot.MkdirAll(p.path, perm)
	}
	return os.MkdirAll(p.path, perm)
}

// createDirectoryIfMissing creates exactly p and reports whether this call
// created it. Its callers already guarantee that p's parent exists.
func createDirectoryIfMissing(execCtx *executionContext, p executionPath, mode os.FileMode) (bool, error) {
	if p.backend == backendArchive {
		return false, errors.New("archive paths are read-only")
	}
	if info, err := lstatPath(execCtx, p); err == nil {
		if !info.IsDir() || isLinkLikeForTraversal(execCtx, p, info) {
			return false, fmt.Errorf("path exists and is not a plain directory")
		}
		return false, nil
	} else if !fileinfo.IsNotExist(err) {
		return false, err
	}

	perm := mode.Perm()
	if perm == 0 {
		perm = 0755
	}
	var err error
	if p.backend == backendSMB {
		ops, opsErr := execCtx.smbOpsFor(p)
		if opsErr != nil {
			return false, opsErr
		}
		err = ops.Mkdir(p.path, perm)
	} else if p.localRoot != nil {
		err = p.localRoot.Mkdir(p.path, perm)
	} else {
		err = os.Mkdir(p.path, perm)
	}
	if err == nil {
		return true, nil
	}

	// A concurrent creator may have won the race. Treat a resulting directory
	// as pre-existing rather than claiming it as nmf's newly created root.
	info, statErr := lstatPath(execCtx, p)
	if statErr == nil && info.IsDir() && !isLinkLikeForTraversal(execCtx, p, info) {
		return false, nil
	}
	return false, err
}

func chtimesPath(execCtx *executionContext, p executionPath, atime, mtime time.Time) error {
	if p.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return err
		}
		return ops.Chtimes(p.path, atime, mtime)
	}
	if p.localRoot != nil {
		return p.localRoot.Chtimes(p.path, atime, mtime)
	}
	return os.Chtimes(p.path, atime, mtime)
}

func removePath(execCtx *executionContext, p executionPath) error {
	if p.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return err
		}
		return ops.Remove(p.path)
	}
	if p.localRoot != nil {
		return p.localRoot.Remove(p.path)
	}
	return os.Remove(p.path)
}

func readlinkPath(execCtx *executionContext, p executionPath) (string, error) {
	if p.backend == backendArchive {
		return "", errors.New("archive symlink targets are not supported")
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return "", err
		}
		return ops.Readlink(p.path)
	}
	if p.localRoot != nil {
		return p.localRoot.Readlink(p.path)
	}
	return os.Readlink(p.path)
}

func symlinkPath(execCtx *executionContext, target string, link executionPath) error {
	if link.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if link.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(link)
		if err != nil {
			return err
		}
		return ops.Symlink(target, link.path)
	}
	if link.localRoot != nil {
		return link.localRoot.Symlink(target, link.path)
	}
	return os.Symlink(target, link.path)
}

func renamePath(execCtx *executionContext, src executionPath, dst executionPath, overwrite bool) error {
	if src.localRoot != nil || dst.localRoot != nil {
		if src.localRoot != dst.localRoot || !overwrite {
			return errors.New("rename between confined paths requires replacement within the same root")
		}
		return dst.localRoot.Rename(src.path, dst.path)
	}
	if src.backend != dst.backend {
		return errors.New("cannot rename across backends")
	}
	if dst.backend == backendArchive {
		return errors.New("archive paths are read-only")
	}
	if dst.backend == backendSMB {
		if normalizeSMBRoot(src.smbDisplayRoot) != normalizeSMBRoot(dst.smbDisplayRoot) {
			return errors.New("cannot rename across SMB shares")
		}
		ops, err := execCtx.smbOpsFor(src)
		if err != nil {
			return err
		}
		return ops.Rename(src.path, dst.path)
	}
	if !overwrite {
		return fileinfo.RenameNativeNoReplace(src.path, dst.path)
	}
	return os.Rename(src.path, dst.path)
}

func openReadPath(execCtx *executionContext, p executionPath) (io.ReadCloser, error) {
	if p.backend == backendArchive {
		vfs, err := execCtx.archiveVFSFor(p)
		if err != nil {
			return nil, err
		}
		return vfs.Open(p.path)
	}
	if p.backend == backendSMB {
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.Open(p.path)
	}
	if p.localRoot != nil {
		return p.localRoot.Open(p.path)
	}
	return os.Open(p.path)
}

func createExclusivePath(execCtx *executionContext, p executionPath, mode os.FileMode) (io.ReadWriteCloser, error) {
	if p.backend == backendArchive {
		return nil, errors.New("archive paths are read-only")
	}
	perm := mode.Perm()
	if perm == 0 {
		perm = 0666
	}
	if p.backend == backendSMB {
		// SMB create attributes can map mode bits differently than local fs.
		// Use a writable default for temp output, then rely on replace semantics.
		ops, err := execCtx.smbOpsFor(p)
		if err != nil {
			return nil, err
		}
		return ops.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0666)
	}
	if p.localRoot != nil {
		return p.localRoot.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm)
	}
	return os.OpenFile(p.path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm)
}
