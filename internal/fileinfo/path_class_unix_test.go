//go:build !windows
// +build !windows

package fileinfo

import "testing"

func TestParseMountInfoLine(t *testing.T) {
	line := "35 24 0:32 / /mnt/share rw,relatime - nfs4 server:/share rw"
	entry, ok := parseMountInfoLine(line)
	if !ok {
		t.Fatal("parseMountInfoLine returned false")
	}
	if entry.mountPoint != "/mnt/share" {
		t.Fatalf("mountPoint = %q, want /mnt/share", entry.mountPoint)
	}
	if entry.fsType != "nfs4" {
		t.Fatalf("fsType = %q, want nfs4", entry.fsType)
	}
	if entry.majorMinor != "0:32" {
		t.Fatalf("majorMinor = %q, want 0:32", entry.majorMinor)
	}
}

func TestBestMountInfoEntryChoosesLongestPrefix(t *testing.T) {
	entry, ok := bestMountInfoEntry("/mnt/share/docs/file.txt", func() ([]mountInfoEntry, error) {
		return []mountInfoEntry{
			{mountPoint: "/", fsType: "ext4"},
			{mountPoint: "/mnt/share", fsType: "nfs4"},
			{mountPoint: "/mnt/share/docs", fsType: "ext4"},
		}, nil
	})
	if !ok {
		t.Fatal("bestMountInfoEntry returned false")
	}
	if entry.mountPoint != "/mnt/share/docs" {
		t.Fatalf("mountPoint = %q, want /mnt/share/docs", entry.mountPoint)
	}
}

func TestNetworkFilesystemTypes(t *testing.T) {
	if !isNetworkFilesystemType("nfs4") {
		t.Fatal("nfs4 should be classified as network")
	}
	if !isNetworkFilesystemType("cifs") {
		t.Fatal("cifs should be classified as network")
	}
	if isNetworkFilesystemType("ext4") {
		t.Fatal("ext4 should not be classified as network")
	}
}
