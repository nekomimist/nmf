package fileinfo

import "testing"

func TestSplitArchivePathIgnoresOrdinaryBangDirectories(t *testing.T) {
	paths := []string{
		"/tmp/bang!/file.txt",
		"smb://server/share/bang!/file.txt",
		"smb://server/share/parent/bang!/nested/file.txt",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			if archiveFile, inner, ok := SplitArchivePath(p); ok {
				t.Fatalf("SplitArchivePath(%q) = %q, %q, true; want ordinary path", p, archiveFile, inner)
			}
			if IsArchivePath(p) {
				t.Fatalf("IsArchivePath(%q) = true, want false", p)
			}
		})
	}
}

func TestSplitArchivePathFindsArchiveAfterBangDirectory(t *testing.T) {
	p := "smb://server/share/bang!/files.zip!/docs/readme.txt"
	archiveFile, inner, ok := SplitArchivePath(p)
	if !ok {
		t.Fatalf("SplitArchivePath(%q) did not find archive boundary", p)
	}
	if archiveFile != "smb://server/share/bang!/files.zip" || inner != "docs/readme.txt" {
		t.Fatalf("SplitArchivePath(%q) = %q, %q", p, archiveFile, inner)
	}
}

func TestSplitArchivePathKeepsBangInsideArchivePath(t *testing.T) {
	p := "smb://server/share/files.zip!/bang!/readme.txt"
	archiveFile, inner, ok := SplitArchivePath(p)
	if !ok {
		t.Fatalf("SplitArchivePath(%q) did not find archive boundary", p)
	}
	if archiveFile != "smb://server/share/files.zip" || inner != "bang!/readme.txt" {
		t.Fatalf("SplitArchivePath(%q) = %q, %q", p, archiveFile, inner)
	}
}
