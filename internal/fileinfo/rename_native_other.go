//go:build !windows && !linux && !darwin

package fileinfo

import "fmt"

func renameNativeSameDir(_, _ string, _ bool) error {
	return fmt.Errorf("atomic no-replace rename is not supported on this platform")
}
