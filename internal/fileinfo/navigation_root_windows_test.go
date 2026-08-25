//go:build windows

package fileinfo

import "testing"

func TestNavigationRootPathWithWindowsLocalPath(t *testing.T) {
	for _, tt := range []struct {
		path string
		want string
	}{
		{path: `C:\Users\neko\src\nmf`, want: `C:\`},
		{path: `d:\work\project`, want: `d:\`},
	} {
		root, err := NavigationRootPath(tt.path)
		if err != nil {
			t.Fatalf("NavigationRootPath(%q) returned error: %v", tt.path, err)
		}
		if root != tt.want {
			t.Fatalf("NavigationRootPath(%q) = %q, want %q", tt.path, root, tt.want)
		}
	}
}
