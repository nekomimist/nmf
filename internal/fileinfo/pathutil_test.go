package fileinfo

import "testing"

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
