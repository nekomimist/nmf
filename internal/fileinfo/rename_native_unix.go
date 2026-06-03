//go:build !windows
// +build !windows

package fileinfo

import "os"

func renameNativeSameDir(oldNative, newNative string, _ bool) error {
	return os.Rename(oldNative, newNative)
}
