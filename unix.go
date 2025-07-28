//go:build !windows
// +build !windows

package main

// isWindowsHidden always returns false on non-Windows systems
func isWindowsHidden(path string) bool {
	return false
}
