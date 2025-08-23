//go:build !windows
// +build !windows

package ui

import (
	"path/filepath"

	"nmf/internal/fileinfo"
)

// GetSystemRoot returns the platform-specific root for tree navigation
func GetSystemRoot() string {
	return "/"
}

// IsVirtualRoot checks if the given path is a virtual root (always false on Unix)
func IsVirtualRoot(path string) bool {
	return false
}

// GetPlatformSpecificChildren returns children for platform-specific paths
// On Unix, there are no special virtual roots, so this always returns false
func GetPlatformSpecificChildren(path string) ([]string, bool) {
	return nil, false
}

// GetPlatformDisplayName returns platform-specific display name for a path
// On Unix, there are no special display names, so this always returns false
func GetPlatformDisplayName(path string) (string, bool) {
	return "", false
}

// IsPlatformDirectory checks if a path is a directory using platform-specific logic
// On Unix, there's no special handling needed, so this always returns false
func IsPlatformDirectory(path string) (bool, bool) {
	return false, false
}

// GetPlatformParent returns the parent path using standard filepath.Dir on Unix
func GetPlatformParent(path string) string {
	if fileinfo.IsSMBDisplay(path) {
		return fileinfo.ParentPath(path)
	}
	return filepath.Dir(path)
}
