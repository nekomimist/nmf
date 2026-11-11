//go:build !linux && !windows

package main

import (
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestDragSourceNativePathRejectsDirectSMB(t *testing.T) {
	fi := fileinfo.FileInfo{Name: "remote.txt", Path: "smb://example.invalid/share/remote.txt"}
	_, err := dragSourceNativePath(fi)
	if err == nil || !strings.Contains(err.Error(), "not supported on this platform") {
		t.Fatalf("error = %v, want unsupported-platform rejection", err)
	}
}

func TestDropDestinationRejectsDirectSMB(t *testing.T) {
	path := "smb://example.invalid/share"
	_, err := dropDestination(path)
	if err == nil || !strings.Contains(err.Error(), "not supported on this platform") {
		t.Fatalf("dropDestination(%q) error = %v, want unsupported-platform rejection", path, err)
	}
}
