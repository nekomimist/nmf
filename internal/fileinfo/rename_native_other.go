//go:build !windows && !linux && !darwin

package fileinfo

import "fmt"

// RenameNativeNoReplace is unsupported on this platform.
func RenameNativeNoReplace(_, _ string) error {
	return fmt.Errorf("atomic no-replace rename is not supported on this platform")
}

func renameNativeSameDir(_, _ string, _ bool) error {
	return fmt.Errorf("atomic no-replace rename is not supported on this platform")
}
