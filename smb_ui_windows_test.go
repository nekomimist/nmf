//go:build windows

package main

import (
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestResolveDirectoryPath_SMBCanonicalDisplay(t *testing.T) {
	resolved, parsed, err := resolveDirectoryPath("smb://example.local/share/path/to/dir")
	if err != nil {
		t.Fatalf("expected SMB path parse to succeed: %v", err)
	}
	if parsed.Scheme != fileinfo.SchemeSMB {
		t.Fatalf("expected smb scheme, got %q", parsed.Scheme)
	}
	if !strings.HasPrefix(strings.ToLower(resolved), "smb://example.local/share") {
		t.Fatalf("unexpected SMB resolved path: %q", resolved)
	}
}
