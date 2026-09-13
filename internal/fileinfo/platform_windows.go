//go:build windows
// +build windows

package fileinfo

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func readDirLocal(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	// FindFirstFile can report PATH_NOT_FOUND for a regular file. Preserve
	// the not-a-directory distinction so parent fallback stops at blockers.
	if os.IsNotExist(err) {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			return nil, &os.PathError{Op: "readdir", Path: path, Err: windows.ERROR_DIRECTORY}
		}
	}
	return entries, err
}

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
