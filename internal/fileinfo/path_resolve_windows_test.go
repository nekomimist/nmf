//go:build windows
// +build windows

package fileinfo

import "testing"

func TestResolveDirectoryPath_UNCCanonicalDisplay(t *testing.T) {
	input := `\\wsl.localhost\Ubuntu\home\neko\src\nmf`

	resolved, parsed, err := ResolveDirectoryPath(input)
	if err != nil {
		t.Fatalf("expected UNC path parse to succeed: %v", err)
	}
	if parsed.Scheme != SchemeSMB {
		t.Fatalf("expected smb scheme, got %q", parsed.Scheme)
	}
	want := "smb://wsl.localhost/Ubuntu/home/neko/src/nmf"
	if resolved != want {
		t.Fatalf("resolved got %q, want %q", resolved, want)
	}
}
