package main

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugHeapProfilesCanBeCompared(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	seen := make(map[string]bool)
	for _, trigger := range []string{"startup", "manual", "manual"} {
		path, err := writeDebugHeapProfile(dir, trigger)
		if err != nil {
			t.Fatal(err)
		}
		if seen[path] {
			t.Fatalf("capture overwrote %q", path)
		}
		seen[path] = true
		if filepath.Dir(path) != dir || !strings.HasPrefix(filepath.Base(path), "nmf-heap-"+trigger+"-") || filepath.Ext(path) != ".pprof" {
			t.Fatalf("unexpected profile path %q", path)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		n, err := io.Copy(io.Discard, r)
		r.Close()
		if err != nil || n == 0 {
			t.Fatalf("reading profile %q: bytes=%d err=%v", path, n, err)
		}
	}
}
