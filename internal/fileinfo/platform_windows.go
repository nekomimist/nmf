//go:build windows
// +build windows

package fileinfo

import (
	"syscall"
)

// Windows file attributes constants
const (
	FILE_ATTRIBUTE_HIDDEN = 0x02
)

// IsWindowsHidden checks if a file has the Windows hidden attribute
func IsWindowsHidden(path string) bool {
	// Convert Go string to UTF-16 for Windows API
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}

	// Get file attributes using Windows API
	attrs, err := syscall.GetFileAttributes(pathPtr)
	if err != nil {
		return false
	}

	// Check if the hidden attribute is set
	return (attrs & FILE_ATTRIBUTE_HIDDEN) != 0
}
