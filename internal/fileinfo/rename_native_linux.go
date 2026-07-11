//go:build linux

package fileinfo

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameNativeSameDir(oldNative, newNative string, _ bool) error {
	if err := unix.Renameat2(unix.AT_FDCWD, oldNative, unix.AT_FDCWD, newNative, unix.RENAME_NOREPLACE); err != nil {
		return &os.LinkError{Op: "rename", Old: oldNative, New: newNative, Err: err}
	}
	return nil
}
