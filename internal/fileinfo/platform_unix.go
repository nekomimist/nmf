//go:build !windows
// +build !windows

package fileinfo

// IsWindowsHidden always returns false on non-Windows systems
func IsWindowsHidden(path string) bool {
	return false
}
