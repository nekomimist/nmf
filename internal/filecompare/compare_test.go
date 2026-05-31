package filecompare

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"nmf/internal/fileinfo"
)

func TestCompareDirectFilesMethods(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name   string
		method Method
		want   []string
	}{
		{name: "missing or newer", method: MissingOrNewer, want: []string{"missing.txt", "newer.txt"}},
		{name: "missing", method: Missing, want: []string{"missing.txt"}},
		{name: "newer", method: Newer, want: []string{"newer.txt"}},
		{name: "size equal", method: SizeEqual, want: []string{"newer.txt", "same.txt", "same-content.txt"}},
		{name: "size time equal", method: SizeTimeEqual, want: []string{"same.txt", "same-content.txt"}},
		{name: "size content equal", method: SizeContentEqual, want: []string{"same-content.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir := t.TempDir()
			dstDir := t.TempDir()

			writeFile(t, srcDir, "missing.txt", "source", base)
			writeFile(t, srcDir, "newer.txt", "1234", base.Add(time.Hour))
			writeFile(t, dstDir, "newer.txt", "zzzz", base)
			writeFile(t, srcDir, "same.txt", "abcd", base)
			writeFile(t, dstDir, "same.txt", "wxyz", base)
			writeFile(t, srcDir, "same-content.txt", "equal", base)
			writeFile(t, dstDir, "same-content.txt", "equal", base)
			writeFile(t, srcDir, "different-size.txt", "long", base)
			writeFile(t, dstDir, "different-size.txt", "x", base)

			got, err := CompareDirectFiles(readFileInfos(t, srcDir), dstDir, tt.method)
			if err != nil {
				t.Fatalf("CompareDirectFiles returned error: %v", err)
			}
			if gotNames := fileNames(got.Matched); !sameNames(gotNames, tt.want) {
				t.Fatalf("matched = %#v, want %#v", gotNames, tt.want)
			}
		})
	}
}

func TestCompareDirectFilesIgnoresDirectories(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dstDir, "sub"), 0755); err != nil {
		t.Fatalf("mkdir destination: %v", err)
	}
	writeFile(t, srcDir, "file.txt", "source", time.Unix(2, 0))

	got, err := CompareDirectFiles(readFileInfos(t, srcDir), dstDir, Missing)
	if err != nil {
		t.Fatalf("CompareDirectFiles returned error: %v", err)
	}
	if gotNames := fileNames(got.Matched); !sameNames(gotNames, []string{"file.txt"}) {
		t.Fatalf("matched = %#v, want only file.txt", gotNames)
	}
	if got.SourceCount != 1 || got.TargetCount != 0 {
		t.Fatalf("counts = source %d target %d, want 1/0", got.SourceCount, got.TargetCount)
	}
}

func writeFile(t *testing.T, dir, name, content string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes %s: %v", name, err)
	}
}

func readFileInfos(t *testing.T, dir string) []fileinfo.FileInfo {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	files := make([]fileinfo.FileInfo, 0, len(entries))
	for _, entry := range entries {
		fi, err := fileinfo.FileInfoFromDirEntry(dir, entry)
		if err != nil {
			t.Fatalf("FileInfoFromDirEntry(%s): %v", entry.Name(), err)
		}
		files = append(files, fi)
	}
	return files
}

func fileNames(files []fileinfo.FileInfo) []string {
	names := make([]string, len(files))
	for i, fi := range files {
		names[i] = fi.Name
	}
	return names
}

func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, name := range got {
		seen[name]++
	}
	for _, name := range want {
		seen[name]--
		if seen[name] < 0 {
			return false
		}
	}
	return true
}
