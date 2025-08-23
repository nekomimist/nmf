//go:build windows
// +build windows

package fileinfo

import "testing"

func TestNormalizeInputPath_Windows(t *testing.T) {
	got := NormalizeInputPath("smb://server/share/dir")
	if !isUNC(got) {
		t.Fatalf("expected UNC on windows, got %q", got)
	}
}
