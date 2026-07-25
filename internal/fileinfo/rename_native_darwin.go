//go:build darwin

package fileinfo

import "golang.org/x/sys/unix"

func renameNoReplaceSyscall(oldNative, newNative string) error {
	return unix.RenamexNp(oldNative, newNative, unix.RENAME_EXCL)
}
