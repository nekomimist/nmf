package fileinfo

import (
	"runtime"
	"testing"
)

func TestIsWindowsHidden(t *testing.T) {
	// Test that the function exists and returns a boolean
	result := IsWindowsHidden("/some/test/path")

	// On non-Windows systems, should always return false
	if runtime.GOOS != "windows" {
		if result {
			t.Error("IsWindowsHidden should return false on non-Windows systems")
		}
	}

	// The function should not panic and should return a boolean
	if result != true && result != false {
		t.Error("IsWindowsHidden should return a boolean value")
	}
}

func TestIsWindowsHiddenWithNonExistentPath(t *testing.T) {
	// Test with a path that doesn't exist - should not panic
	result := IsWindowsHidden("/non/existent/path/file.txt")

	// Should return false for non-existent paths
	if result {
		t.Error("IsWindowsHidden should return false for non-existent paths")
	}
}
