//go:build linux || darwin

package fileinfo

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// renameNoReplace performs the platform's no-clobber rename syscall
// (renameat2 on Linux, renamex_np on darwin). It is a variable so tests can
// simulate filesystems that reject the flag.
var renameNoReplace = renameNoReplaceSyscall

// renameNativeSameDir renames within one directory, preferring the kernel's
// no-clobber rename and degrading to a plain rename where that flag is not
// implemented.
func renameNativeSameDir(oldNative, newNative string, caseOnlyRename bool) error {
	err := renameNoReplace(oldNative, newNative)
	switch {
	case err == nil:
		return nil
	case renameFlagsUnsupported(err):
		return renameNativeFallback(oldNative, newNative, caseOnlyRename)
	case caseOnlyRename && errors.Is(err, unix.EEXIST):
		// Case-insensitive filesystem (drvfs, CIFS, exFAT, APFS by default):
		// the flag resolves the destination to the source itself, so a plain
		// rename is the only way to change the case. RenamePortable has
		// already proven they are the same file via sameNativeFile.
		return renameNativeFallback(oldNative, newNative, caseOnlyRename)
	}
	return &os.LinkError{Op: "rename", Old: oldNative, New: newNative, Err: err}
}

// renameFlagsUnsupported reports whether err means the kernel or the backing
// filesystem rejected the no-replace rename *flag* rather than the rename
// itself. Flagged rename is not universally implemented: 9p/drvfs (WSL's
// /mnt/<drive> and its UNC mounts), vfat, exfat and several network
// filesystems answer EINVAL, older kernels answer ENOSYS, and macOS returns
// ENOTSUP on non-APFS volumes. Those paths must keep working, so the caller
// degrades instead of surfacing the error.
func renameFlagsUnsupported(err error) bool {
	return errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.ENOSYS) ||
		errors.Is(err, unix.ENOTSUP) ||
		errors.Is(err, unix.EOPNOTSUPP)
}

// renameNativeFallback performs the rename without kernel-enforced no-clobber.
// It re-checks the destination immediately beforehand so the window in which a
// concurrent writer could be overwritten stays as short as a stat, matching the
// guarantee this package offered before flagged rename was introduced.
//
// caseOnlyRename skips that check on purpose: the destination "exists" only
// because it resolves to the source itself.
func renameNativeFallback(oldNative, newNative string, caseOnlyRename bool) error {
	if !caseOnlyRename {
		if _, err := os.Lstat(newNative); err == nil {
			return &os.LinkError{Op: "rename", Old: oldNative, New: newNative, Err: unix.EEXIST}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(oldNative, newNative)
}
