//go:build windows

package fileinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
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

func TestWindowsHiddenAttributeOnListedDirectory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "folder")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	native, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := windows.GetFileAttributes(native)
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetFileAttributes(native, attrs|windows.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
	info, err := FileInfoFromDirEntry(root, readDirEntry(t, root, "folder"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsHidden() || !info.IsDir || info.FileType != FileTypeDirectory {
		t.Fatalf("listed hidden directory: %+v", info)
	}
}
