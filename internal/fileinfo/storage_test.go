package fileinfo

import "testing"

func TestStorageInfoFromBlocks(t *testing.T) {
	got := storageInfoFromBlocks(10, 4, 1024)
	if got.Total != 10240 {
		t.Fatalf("Total got %d, want 10240", got.Total)
	}
	if got.Free != 4096 {
		t.Fatalf("Free got %d, want 4096", got.Free)
	}
	if got.Used != 6144 {
		t.Fatalf("Used got %d, want 6144", got.Used)
	}
}

func TestStatStoragePortableLocalPath(t *testing.T) {
	info, err := StatStoragePortable(t.TempDir())
	if err != nil {
		t.Fatalf("StatStoragePortable returned error: %v", err)
	}
	if info.Total == 0 {
		t.Fatal("Total should be greater than zero")
	}
	if info.Free > info.Total {
		t.Fatalf("Free %d should not exceed Total %d", info.Free, info.Total)
	}
	if info.Used != info.Total-info.Free {
		t.Fatalf("Used got %d, want %d", info.Used, info.Total-info.Free)
	}
}
