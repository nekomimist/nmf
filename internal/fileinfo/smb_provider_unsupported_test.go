//go:build !linux && !windows

package fileinfo

import (
	"strings"
	"testing"
)

func TestResolveReadRejectsDirectSMBOnUnsupportedPlatform(t *testing.T) {
	vfs, parsed, err := ResolveRead("smb://server/share/etc")
	if err == nil {
		t.Fatal("ResolveRead should reject direct SMB on this platform")
	}
	if vfs != nil {
		t.Fatalf("ResolveRead VFS = %T, want nil", vfs)
	}
	if parsed.Scheme != SchemeSMB || parsed.Provider != "smb" {
		t.Fatalf("ResolveRead parsed = %+v, want direct SMB metadata", parsed)
	}
}

func TestResolveDirectoryPathRejectsDirectSMBOnUnsupportedPlatform(t *testing.T) {
	_, parsed, err := ResolveDirectoryPath("smb://example.invalid/share/path/to/dir")
	if err == nil || !strings.Contains(err.Error(), "not supported on this platform") {
		t.Fatalf("ResolveDirectoryPath error = %v, want unsupported-platform rejection", err)
	}
	if parsed.Scheme != SchemeSMB || parsed.Provider != "smb" {
		t.Fatalf("ResolveDirectoryPath parsed = %+v, want direct SMB metadata", parsed)
	}
}

func TestCreateTextFilePortableRejectsDirectSMBOnUnsupportedPlatform(t *testing.T) {
	_, err := CreateTextFilePortable("smb://example.invalid/share", "note.txt", "hello")
	if err == nil || !strings.Contains(err.Error(), "not supported on this platform") {
		t.Fatalf("CreateTextFilePortable error = %v, want unsupported-platform rejection", err)
	}
}
