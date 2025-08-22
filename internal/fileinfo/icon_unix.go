//go:build !windows

package fileinfo

import (
	"fyne.io/fyne/v2"
)

// Non-Windows platforms: return nil to indicate using theme defaults.
func platformFetchExtIcon(ext string, size int) (fyne.Resource, error) {
	return nil, nil
}

func platformFetchFileIcon(path string, size int) (fyne.Resource, error) {
	return nil, nil
}

// preferFileIcon determines whether to fetch a file-specific icon (by path).
// Non-Windows: always false (use extension icons / theme defaults).
func preferFileIcon(path, ext string) bool { return false }
