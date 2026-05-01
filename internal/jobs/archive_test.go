package jobs

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"nmf/internal/fileinfo"
)

func TestCopyFromArchiveToLocalDirectory(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{
		"dir/file.txt": "copied from archive",
	})
	dstDir := t.TempDir()
	job := &Job{Type: TypeCopy, ctx: t.Context()}

	src := fileinfo.JoinPath(fileinfo.ArchiveRootPath(archivePath), "dir")
	if err := copyOrMovePath(job, src, dstDir); err != nil {
		t.Fatalf("copyOrMovePath returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "dir", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile copied file returned error: %v", err)
	}
	if string(data) != "copied from archive" {
		t.Fatalf("copied content = %q", string(data))
	}
}

func TestMoveFromArchiveIsRejected(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{"file.txt": "data"})
	dstDir := t.TempDir()
	job := &Job{Type: TypeMove, ctx: t.Context()}

	src := fileinfo.JoinPath(fileinfo.ArchiveRootPath(archivePath), "file.txt")
	if err := copyOrMovePath(job, src, dstDir); err == nil {
		t.Fatal("copyOrMovePath move from archive returned nil error")
	}
}

func writeJobTestZip(t *testing.T, files map[string]string) string {
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
