//go:build !windows

package fileinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrashPassesExactListedPathToGIO(t *testing.T) {
	dir := t.TempDir()
	sibling := filepath.Join(dir, "report")
	selected := sibling + " "
	target := filepath.Join(dir, "trashed")
	for path, data := range map[string]string{sibling: "sibling", selected: "selected"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Exercise the native command boundary using a trash destination in the fixture.
	const script = "#!/bin/sh\n[ \"$#\" = 3 ] && [ \"$1\" = trash ] && [ \"$2\" = -- ] && mv \"$3\" \"$NMF_TEST_TRASH\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gio"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("NMF_TEST_TRASH", target)
	if err := TrashPath(t.Context(), selected); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{sibling: "sibling", target: "selected"} {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("read %q = %q, %v", path, data, err)
		}
	}
}
