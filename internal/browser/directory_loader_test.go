package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"nmf/internal/config"
)

func TestDirectoryLoaderBuildsSortedWidgetFreeResult(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	for _, name := range []string{"zeta.txt", "alpha.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) returned error: %v", name, err)
		}
	}

	loader := NewDirectoryLoader()
	handle := loader.Begin()
	result, err := loader.Load(handle, DirectoryLoadRequest{
		Path: dir,
		Sort: config.SortConfig{
			SortBy:           "name",
			SortOrder:        "asc",
			DirectoriesFirst: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if result.RequestedPath != dir || result.Path != dir || result.UsedParentFallback {
		t.Fatalf("load paths = requested %q opened %q fallback %t", result.RequestedPath, result.Path, result.UsedParentFallback)
	}
	if got, want := fileNames(result.Files), []string{"..", "docs", "alpha.txt", "zeta.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("result names = %v, want %v", got, want)
	}
	if result.Files[0].Path != filepath.Dir(dir) || !result.Files[0].IsDir {
		t.Fatalf("parent entry = %#v, want directory %q", result.Files[0], filepath.Dir(dir))
	}
	if !loader.Finish(handle.ID) {
		t.Fatal("Finish rejected active result")
	}
	if !errors.Is(handle.Context.Err(), context.Canceled) {
		t.Fatalf("handle context after Finish = %v, want context.Canceled", handle.Context.Err())
	}
	if loader.Active(handle.ID) {
		t.Fatal("finished load remained active")
	}
}

func TestDirectoryLoaderReportsFallbackCandidate(t *testing.T) {
	parent := t.TempDir()
	requested := filepath.Join(parent, "missing", "child")
	loader := NewDirectoryLoader()
	handle := loader.Begin()
	defer loader.Cancel(handle.ID)
	var candidates []string

	result, err := loader.Load(handle, DirectoryLoadRequest{
		Path:                requested,
		AllowParentFallback: true,
		Sort:                nameSort(),
	}, func(candidate string) {
		candidates = append(candidates, candidate)
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !result.UsedParentFallback || result.Path != parent {
		t.Fatalf("fallback result = path %q used %t, want %q/true", result.Path, result.UsedParentFallback, parent)
	}
	if got, want := candidates, []string{filepath.Dir(requested), parent}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fallback candidates = %#v, want %#v", got, want)
	}
}
