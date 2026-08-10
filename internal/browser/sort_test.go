package browser

import (
	"reflect"
	"testing"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

func TestSortSlice(t *testing.T) {
	times := []time.Time{
		time.Unix(1000, 0),
		time.Unix(2000, 0),
		time.Unix(3000, 0),
		time.Unix(4000, 0),
		time.Unix(5000, 0),
	}
	newFiles := func() []fileinfo.FileInfo {
		return []fileinfo.FileInfo{
			{Name: "Banana.txt", Size: 100, Modified: times[3]},
			{Name: "apple.TXT", Size: 50, Modified: times[1]},
			{Name: "README", Size: 10, Modified: times[4]},
			{Name: "notes", Size: 20, Modified: times[0]},
			{Name: "zeta.md", Size: 5, Modified: times[2]},
		}
	}

	tests := []struct {
		sortBy    string
		sortOrder string
		want      []string
	}{
		{"name", "asc", []string{"apple.TXT", "Banana.txt", "notes", "README", "zeta.md"}},
		{"name", "desc", []string{"zeta.md", "README", "notes", "Banana.txt", "apple.TXT"}},
		{"size", "asc", []string{"zeta.md", "README", "notes", "apple.TXT", "Banana.txt"}},
		{"size", "desc", []string{"Banana.txt", "apple.TXT", "notes", "README", "zeta.md"}},
		{"modified", "asc", []string{"notes", "apple.TXT", "zeta.md", "Banana.txt", "README"}},
		{"modified", "desc", []string{"README", "Banana.txt", "zeta.md", "apple.TXT", "notes"}},
		{"extension", "asc", []string{"notes", "README", "zeta.md", "apple.TXT", "Banana.txt"}},
		{"extension", "desc", []string{"Banana.txt", "apple.TXT", "zeta.md", "README", "notes"}},
	}

	for _, tt := range tests {
		t.Run(tt.sortBy+"_"+tt.sortOrder, func(t *testing.T) {
			files := newFiles()
			SortSlice(files, config.SortConfig{SortBy: tt.sortBy, SortOrder: tt.sortOrder})
			if got := fileNames(files); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SortSlice(SortBy=%s,SortOrder=%s) = %v, want %v", tt.sortBy, tt.sortOrder, got, tt.want)
			}
		})
	}
}

func TestSortFilesPinsParentAndGroupsDirectories(t *testing.T) {
	input := func() []fileinfo.FileInfo {
		return []fileinfo.FileInfo{
			{Name: "..", IsDir: true},
			{Name: "zeta", IsDir: true},
			{Name: "banana.txt"},
			{Name: "apple", IsDir: true},
			{Name: "cherry.txt"},
		}
	}

	t.Run("DirectoriesFirst", func(t *testing.T) {
		got := SortFiles(input(), config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true})
		if names, want := fileNames(got), []string{"..", "apple", "zeta", "banana.txt", "cherry.txt"}; !reflect.DeepEqual(names, want) {
			t.Fatalf("SortFiles(DirectoriesFirst=true) = %v, want %v", names, want)
		}
	})

	t.Run("FlatSort", func(t *testing.T) {
		got := SortFiles(input(), config.SortConfig{SortBy: "name", SortOrder: "asc"})
		if names, want := fileNames(got), []string{"..", "apple", "banana.txt", "cherry.txt", "zeta"}; !reflect.DeepEqual(names, want) {
			t.Fatalf("SortFiles(DirectoriesFirst=false) = %v, want %v", names, want)
		}
	})

	t.Run("NoParentEntry", func(t *testing.T) {
		got := SortFiles([]fileinfo.FileInfo{{Name: "b", IsDir: true}, {Name: "a.txt"}}, config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true})
		if names, want := fileNames(got), []string{"b", "a.txt"}; !reflect.DeepEqual(names, want) {
			t.Fatalf("SortFiles(no parent) = %v, want %v", names, want)
		}
	})

	t.Run("ShortInput", func(t *testing.T) {
		if got := SortFiles(nil, config.SortConfig{}); got != nil {
			t.Fatalf("SortFiles(nil) = %v, want nil", got)
		}
		single := []fileinfo.FileInfo{{Name: "only"}}
		if got := SortFiles(single, config.SortConfig{}); len(got) != 1 || got[0].Name != "only" {
			t.Fatalf("SortFiles(single) = %v, want unchanged single element", got)
		}
	})
}
