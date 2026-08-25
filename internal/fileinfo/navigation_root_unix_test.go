//go:build !windows

package fileinfo

import "testing"

func TestNavigationRootPathWithUnixLocalPath(t *testing.T) {
	root, err := NavigationRootPath("/home/neko/src/nmf")
	if err != nil {
		t.Fatalf("NavigationRootPath returned error: %v", err)
	}
	if root != "/" {
		t.Fatalf("NavigationRootPath = %q, want /", root)
	}
}
