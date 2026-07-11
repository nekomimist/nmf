//go:build darwin

package fileinfo

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameNativeSameDir(oldNative, newNative string, _ bool) error {
	if err := unix.RenamexNp(oldNative, newNative, unix.RENAME_EXCL); err != nil {
		return &os.LinkError{Op: "rename", Old: oldNative, New: newNative, Err: err}
	}
	return nil
}
