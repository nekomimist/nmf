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

func TestMountedSMBNativePathStaysWithinRoot(t *testing.T) {
	got, err := mountedSMBNativePath("/mnt/share", []string{"dir", "file"})
	if err != nil {
		t.Fatalf("mountedSMBNativePath returned error: %v", err)
	}
	if got != "/mnt/share/dir/file" {
		t.Fatalf("mounted path = %q, want /mnt/share/dir/file", got)
	}
	if _, err := mountedSMBNativePath("/mnt/share", []string{"..", "etc"}); err == nil {
		t.Fatal("mountedSMBNativePath should reject a path outside the mount root")
	}
	if _, err := mountedSMBNativePath("relative/share", nil); err == nil {
		t.Fatal("mountedSMBNativePath should reject a relative mount point")
	}
}
