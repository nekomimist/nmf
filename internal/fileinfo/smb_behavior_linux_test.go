//go:build linux

package fileinfo

import (
	"strings"
	"testing"
)

func TestResolveDirectoryPath_SMBCanonicalDisplay(t *testing.T) {
	resolved, parsed, err := ResolveDirectoryPath("smb://example.local/share/path/to/dir")
	if err != nil {
		t.Fatalf("expected SMB path parse to succeed: %v", err)
	}
	if parsed.Scheme != SchemeSMB {
		t.Fatalf("expected smb scheme, got %q", parsed.Scheme)
	}
	if !strings.HasPrefix(strings.ToLower(resolved), "smb://example.local/share") {
		t.Fatalf("unexpected SMB resolved path: %q", resolved)
	}
}

func TestCreateTextFilePortableRejectsDirectSMBPath(t *testing.T) {
	_, err := CreateTextFilePortable("smb://example.invalid/share", "note.txt", "hello")
	if err == nil || !strings.Contains(err.Error(), "direct SMB") {
		t.Fatalf("CreateTextFilePortable error = %v, want direct SMB rejection", err)
	}
}
