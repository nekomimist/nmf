package fileinfo

import (
	"path/filepath"
	"testing"
)

func TestJoinParentBaseWithSMB(t *testing.T) {
	base := "smb://host/share/dir"
	name := "file.txt"
	joined := JoinPath(base, name)
	if joined != "smb://host/share/dir/file.txt" {
		t.Fatalf("JoinPath(smb) got %q", joined)
	}
	parent := ParentPath(joined)
	if parent != base {
		t.Fatalf("ParentPath(smb) got %q, want %q", parent, base)
	}
	if last := BaseName(joined); last != "file.txt" {
		t.Fatalf("BaseName(smb) got %q", last)
	}
}

func TestJoinParentBaseWithLocal(t *testing.T) {
	base := "/tmp/dir"
	name := "file.txt"
	joined := JoinPath(base, name)
	// filepath.Join may collapse //, but here simple concat
	if joined != "/tmp/dir/file.txt" {
		t.Fatalf("JoinPath(local) got %q", joined)
	}
	parent := ParentPath(joined)
	if parent != "/tmp/dir" {
		t.Fatalf("ParentPath(local) got %q", parent)
	}
	if last := BaseName(joined); last != name {
		t.Fatalf("BaseName(local) got %q", last)
	}
}

func TestNavigationRootPathWithSMB(t *testing.T) {
	for _, input := range []string{
		"smb://server/share/dir/subdir",
		`\\server\share\dir\subdir`,
	} {
		root, err := NavigationRootPath(input)
		if err != nil {
			t.Fatalf("NavigationRootPath(%q) returned error: %v", input, err)
		}
		if root != "smb://server/share" {
			t.Fatalf("NavigationRootPath(%q) = %q, want smb://server/share", input, root)
		}
	}
}

func TestNavigationRootPathWithArchive(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "files.zip")
	input := ArchiveDisplayPath(archive, "docs/src")

	root, err := NavigationRootPath(input)
	if err != nil {
		t.Fatalf("NavigationRootPath(%q) returned error: %v", input, err)
	}
	if want := ArchiveRootPath(archive); root != want {
		t.Fatalf("NavigationRootPath(%q) = %q, want %q", input, root, want)
	}
}

func TestNavigationRootPathWithSMBArchive(t *testing.T) {
	input := "smb://server/share/files.zip!/docs/src"

	root, err := NavigationRootPath(input)
	if err != nil {
		t.Fatalf("NavigationRootPath(%q) returned error: %v", input, err)
	}
	if want := "smb://server/share/files.zip!/"; root != want {
		t.Fatalf("NavigationRootPath(%q) = %q, want %q", input, root, want)
	}
}
