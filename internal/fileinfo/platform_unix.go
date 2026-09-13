//go:build !windows
// +build !windows

package fileinfo

import "os"

func readDirLocal(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }

// IsWindowsHidden always returns false on non-Windows systems
func IsWindowsHidden(path string) bool {
	return false
}
