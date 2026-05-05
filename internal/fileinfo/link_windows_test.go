//go:build windows

package fileinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFileInfoFromDirEntryJunction(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "junction")
	createTestJunction(t, link, target)

	entry := readDirEntry(t, tmp, "junction")
	got, err := FileInfoFromDirEntry(tmp, entry)
	if err != nil {
		t.Fatalf("FileInfoFromDirEntry returned error: %v", err)
	}
	if got.FileType != FileTypeSymlink {
		t.Fatalf("FileType = %v, want FileTypeSymlink", got.FileType)
	}
	if !got.IsDir {
		t.Fatal("junction should be navigable")
	}
}

func createTestJunction(t *testing.T, link string, target string) {
	t.Helper()
	output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("junction unavailable: %v: %s", err, string(output))
	}
}
