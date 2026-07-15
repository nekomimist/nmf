//go:build linux

package main

import (
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestDragSourceNativePathRejectsDirectSMB(t *testing.T) {
	fi := fileinfo.FileInfo{Name: "remote.txt", Path: "smb://example.invalid/share/remote.txt"}
	_, err := dragSourceNativePath(fi)
	if err == nil || !strings.Contains(err.Error(), "direct SMB item") {
		t.Fatalf("error = %v, want direct SMB rejection", err)
	}
}

func TestDropDestinationRejectsDirectSMB(t *testing.T) {
	path := "smb://example.invalid/share"
	_, err := dropDestination(path)
	if err == nil || !strings.Contains(err.Error(), "direct SMB views") {
		t.Fatalf("dropDestination(%q) error = %v, want direct SMB rejection", path, err)
	}
}

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
