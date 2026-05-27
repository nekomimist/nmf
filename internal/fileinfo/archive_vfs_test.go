package fileinfo

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
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

func TestArchiveVFSShiftJISZipNames(t *testing.T) {
	archivePath := writeShiftJISZip(t)
	setArchiveOptionsForTest(t, ArchiveOptions{ZipNameEncoding: "shift_jis"})
	root := ArchiveRootPath(archivePath)

	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 1 || entries[0].Name() != "てすと" {
		t.Fatalf("root entries = %v, want てすと", entryNames(entries))
	}

	innerPath := JoinPath(root, "てすと/UNICODE Text-test2006.txt")
	vfs, parsed, err := ResolveRead(innerPath)
	if err != nil {
		t.Fatalf("ResolveRead(%q) returned error: %v", innerPath, err)
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
	if string(data) != "hello zip\n" {
		t.Fatalf("archive file content = %q, want %q", string(data), "hello zip\n")
	}
}

func TestArchiveVFSConfiguredCP437ZipNames(t *testing.T) {
	archivePath := writeEncodedZip(t, charmap.CodePage437, map[string]string{
		"café/":           "",
		"café/readme.txt": "hello cp437\n",
	})
	setArchiveOptionsForTest(t, ArchiveOptions{ZipNameEncoding: "cp437"})
	root := ArchiveRootPath(archivePath)

	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 1 || entries[0].Name() != "café" {
		t.Fatalf("root entries = %v, want café", entryNames(entries))
	}
}

func TestArchiveVFSKeepsValidUTF8ZipNamesWithoutUTF8Flag(t *testing.T) {
	archivePath := writeNonUTF8FlaggedZip(t, map[string]string{
		"てすと/":                          "",
		"てすと/UNICODE Text-test2006.txt": "hello utf8\n",
	})
	setArchiveOptionsForTest(t, ArchiveOptions{ZipNameEncoding: "shift_jis"})
	root := ArchiveRootPath(archivePath)

	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 1 || entries[0].Name() != "てすと" {
		t.Fatalf("root entries = %v, want てすと", entryNames(entries))
	}
}

func TestResolveArchiveZipNameEncoding(t *testing.T) {
	tests := []string{"", "shift_jis", "sjis", "cp932", "cp437", "ibm437", "utf-8", "UTF8"}
	for _, tt := range tests {
		if _, err := ResolveArchiveZipNameEncoding(tt); err != nil {
			t.Fatalf("ResolveArchiveZipNameEncoding(%q) returned error: %v", tt, err)
		}
	}
	if _, err := ResolveArchiveZipNameEncoding("not-a-real-encoding"); err == nil {
		t.Fatal("ResolveArchiveZipNameEncoding should reject unknown encoding")
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

func writeShiftJISZip(t *testing.T) string {
	return writeEncodedZip(t, japanese.ShiftJIS, map[string]string{
		"てすと/":                          "",
		"てすと/UNICODE Text-test2006.txt": "hello zip\n",
	})
}

func writeEncodedZip(t *testing.T, enc encoding.Encoding, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "encoded.zip")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create zip returned error: %v", err)
	}
	zw := zip.NewWriter(out)
	for name, content := range files {
		encodedName, err := enc.NewEncoder().String(name)
		if err != nil {
			t.Fatalf("encode %q returned error: %v", name, err)
		}
		hdr := &zip.FileHeader{Name: encodedName, NonUTF8: true, Method: zip.Store}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("CreateHeader(%q) returned error: %v", name, err)
		}
		if content != "" {
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatalf("Write(%q) returned error: %v", name, err)
			}
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

func writeNonUTF8FlaggedZip(t *testing.T, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "utf8-no-flag.zip")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create zip returned error: %v", err)
	}
	zw := zip.NewWriter(out)
	for name, content := range files {
		hdr := &zip.FileHeader{Name: name, NonUTF8: true, Method: zip.Store}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("CreateHeader(%q) returned error: %v", name, err)
		}
		if content != "" {
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatalf("Write(%q) returned error: %v", name, err)
			}
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

func setArchiveOptionsForTest(t *testing.T, opts ArchiveOptions) {
	t.Helper()
	previous := currentArchiveOptions()
	if err := SetArchiveOptions(opts); err != nil {
		t.Fatalf("SetArchiveOptions(%+v) returned error: %v", opts, err)
	}
	t.Cleanup(func() {
		if err := SetArchiveOptions(previous); err != nil {
			t.Fatalf("restore archive options returned error: %v", err)
		}
	})
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
