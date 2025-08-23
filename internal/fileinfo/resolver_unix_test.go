//go:build !windows
// +build !windows

package fileinfo

import "testing"

func TestNormalizeInputPath_Unix(t *testing.T) {
	inp := "smb://server/share/dir"
	got := NormalizeInputPath(inp)
	if got != inp {
		t.Fatalf("normalize should be no-op on unix: %q", got)
	}
}
