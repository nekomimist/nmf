package fileinfo

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveVFSReadDirStatAndOpen(t *testing.T) {
	archivePath := writeTestZip(t, map[string]string{
		"docs/readme.txt": "hello archive",
		"root.txt":        "at root",
	})

	if !IsSupportedArchive(archivePath) {
		t.Fatalf("IsSupportedArchive(%q) = false, want true", archivePath)
	}

	root := ArchiveRootPath(archivePath)
	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 2 {
		t.Fatalf("root entries len = %d, want 2", len(entries))
	}

	innerPath := JoinPath(root, "docs/readme.txt")
	info, err := StatPortable(innerPath)
	if err != nil {
		t.Fatalf("StatPortable(%q) returned error: %v", innerPath, err)
	}
	if info.IsDir() || info.Size() != int64(len("hello archive")) {
		t.Fatalf("unexpected info: isDir=%t size=%d", info.IsDir(), info.Size())
	}

	vfs, parsed, err := ResolveRead(innerPath)
	if err != nil {
		t.Fatalf("ResolveRead(%q) returned error: %v", innerPath, err)
	}
	if parsed.Scheme != SchemeArchive || parsed.Native != "docs/readme.txt" {
		t.Fatalf("unexpected parsed archive path: %+v", parsed)
	}
	rc, err := vfs.Open(parsed.Native)
	if err != nil {
		t.Fatalf("Open(%q) returned error: %v", parsed.Native, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != "hello archive" {
		t.Fatalf("archive file content = %q, want %q", string(data), "hello archive")
	}
}

func TestArchivePathHelpers(t *testing.T) {
	root := ArchiveRootPath("/tmp/sample.zip")
	if root != "/tmp/sample.zip!/" {
		t.Fatalf("ArchiveRootPath got %q", root)
	}
	inner := JoinPath(root, "dir/file.txt")
	if inner != "/tmp/sample.zip!/dir/file.txt" {
		t.Fatalf("JoinPath archive got %q", inner)
	}
	if parent := ParentPath(inner); parent != "/tmp/sample.zip!/dir" {
		t.Fatalf("ParentPath archive got %q", parent)
	}
	if parent := ParentPath(root); parent != "/tmp" {
		t.Fatalf("ParentPath archive root got %q", parent)
	}
	if base := BaseName(root); base != "sample.zip" {
		t.Fatalf("BaseName archive root got %q", base)
	}
}

func writeTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "sample.zip")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create zip returned error: %v", err)
	}
	zw := zip.NewWriter(out)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) returned error: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) returned error: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close returned error: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("file Close returned error: %v", err)
	}
	return archivePath
}
