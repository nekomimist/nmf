//go:build windows
// +build windows

package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"nmf/internal/fileinfo"
)

const (
	// Virtual root identifier for Windows drives
	WindowsVirtualRoot = "\\"
)

// GetSystemRoot returns the platform-specific root for tree navigation
func GetSystemRoot() string {
	return WindowsVirtualRoot
}

// IsVirtualRoot checks if the given path is the Windows virtual root
func IsVirtualRoot(path string) bool {
	return path == WindowsVirtualRoot
}

// getAvailableDrives returns a list of available Windows drives (C:\, D:\, etc.)
func getAvailableDrives() []string {
	var drives []string

	// Get drive bitmask using Windows API
	driveMask, err := getDrivesBitmask()
	if err != nil {
		// Fallback: try common drive letters
		for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			drive := string(letter) + ":\\"
			if _, err := os.Stat(drive); err == nil {
				drives = append(drives, drive)
			}
		}
		return drives
	}

	// Convert bitmask to drive letters
	for i := 0; i < 26; i++ {
		if driveMask&(1<<uint(i)) != 0 {
			driveLetter := string('A' + rune(i))
			drive := driveLetter + ":\\"
			drives = append(drives, drive)
		}
	}

	// Sort drives alphabetically
	sort.Strings(drives)
	return drives
}

// getDrivesBitmask uses Windows API to get available drives as a bitmask
func getDrivesBitmask() (uint32, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getLogicalDrives := kernel32.NewProc("GetLogicalDrives")

	ret, _, err := getLogicalDrives.Call()
	if ret == 0 {
		return 0, err
	}

	return uint32(ret), nil
}

// GetPlatformSpecificChildren returns children for platform-specific paths
func GetPlatformSpecificChildren(path string) ([]string, bool) {
	if IsVirtualRoot(path) {
		// Return available drives as children of virtual root
		return getAvailableDrives(), true
	}
	return nil, false
}

// GetPlatformDisplayName returns platform-specific display name for a path
func GetPlatformDisplayName(path string) (string, bool) {
	if IsVirtualRoot(path) {
		return "My Computer", true
	}

	// For drive roots like "C:\", display as "C:"
	if len(path) == 3 && path[1] == ':' && path[2] == '\\' {
		return path[:2], true
	}

	return "", false
}

// IsPlatformDirectory checks if a path is a directory on Windows
func IsPlatformDirectory(path string) (bool, bool) {
	if IsVirtualRoot(path) {
		return true, true // Virtual root is always a "directory"
	}

	// For drive letters, check if they exist
	if len(path) == 3 && path[1] == ':' && path[2] == '\\' {
		if _, err := os.Stat(path); err == nil {
			return true, true
		}
		return false, true
	}

	return false, false // Let default handling take over
}

// normalizeWindowsPath ensures Windows paths use backslashes
func normalizeWindowsPath(path string) string {
	if IsVirtualRoot(path) {
		return path
	}

	// Convert forward slashes to backslashes for Windows
	normalized := strings.ReplaceAll(path, "/", "\\")

	// Ensure drive letters end with backslash
	if len(normalized) == 2 && normalized[1] == ':' {
		normalized += "\\"
	}

	return normalized
}

// GetPlatformParent returns the parent path for Windows paths with proper drive handling
func GetPlatformParent(path string) string {
	if IsVirtualRoot(path) {
		return path // Virtual root has no parent
	}

	// For drive roots like "C:\", parent is virtual root
	if len(path) == 3 && path[1] == ':' && path[2] == '\\' {
		return WindowsVirtualRoot
	}

	// For SMB display paths, use fileinfo's logic
	if fileinfo.IsSMBDisplay(path) {
		return fileinfo.ParentPath(path)
	}

	// For other paths, use standard parent logic
	parent := filepath.Dir(path)
	if parent == "." || parent == path {
		// If we can't go up further, try going to drive root
		if len(path) >= 2 && path[1] == ':' {
			return path[:3] // Return drive root (e.g., "C:\")
		}
		return WindowsVirtualRoot
	}

	return parent
}
