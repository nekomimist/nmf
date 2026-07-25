//go:build linux

package fileinfo

import "golang.org/x/sys/unix"

func renameNoReplaceSyscall(oldNative, newNative string) error {
	return unix.Renameat2(unix.AT_FDCWD, oldNative, unix.AT_FDCWD, newNative, unix.RENAME_NOREPLACE)
}
