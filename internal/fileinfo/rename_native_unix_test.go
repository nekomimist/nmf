//go:build linux || darwin

package fileinfo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestRenameFlagsUnsupported(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "EINVAL (9p/drvfs, vfat, exfat)", err: unix.EINVAL, want: true},
		{name: "ENOSYS (old kernel)", err: unix.ENOSYS, want: true},
		{name: "ENOTSUP (darwin non-APFS)", err: unix.ENOTSUP, want: true},
		{name: "EOPNOTSUPP", err: unix.EOPNOTSUPP, want: true},
		{name: "wrapped in LinkError", err: &os.LinkError{Err: unix.EINVAL}, want: true},
		{name: "EEXIST is a real conflict", err: unix.EEXIST, want: false},
		{name: "ENOENT is a real failure", err: unix.ENOENT, want: false},
		{name: "EACCES is a real failure", err: unix.EACCES, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renameFlagsUnsupported(tt.err); got != tt.want {
				t.Fatalf("renameFlagsUnsupported(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}

// stubRenameNoReplace makes the no-clobber syscall report err for the duration
// of the test, simulating a filesystem that does not implement the flag.
func stubRenameNoReplace(t *testing.T, err error) {
	t.Helper()
	previous := renameNoReplace
	renameNoReplace = func(string, string) error { return err }
	t.Cleanup(func() { renameNoReplace = previous })
}

// A filesystem that rejects the no-replace flag (9p/drvfs under WSL, vfat,
// exfat, several network mounts) must still be renamable.
func TestRenameNativeSameDirFallsBackWhenFlagUnsupported(t *testing.T) {
	for _, unsupported := range []error{unix.EINVAL, unix.ENOSYS, unix.ENOTSUP} {
		t.Run(unsupported.Error(), func(t *testing.T) {
			dir := t.TempDir()
			oldPath := filepath.Join(dir, "old.txt")
			newPath := filepath.Join(dir, "new.txt")
			if err := os.WriteFile(oldPath, []byte("data"), 0o644); err != nil {
				t.Fatal(err)
			}
			stubRenameNoReplace(t, unsupported)

			if err := renameNativeSameDir(oldPath, newPath, false); err != nil {
				t.Fatalf("renameNativeSameDir() = %v, want nil", err)
			}
			if _, err := os.Lstat(oldPath); !os.IsNotExist(err) {
				t.Fatalf("source still present: %v", err)
			}
			data, err := os.ReadFile(newPath)
			if err != nil || string(data) != "data" {
				t.Fatalf("target = %q, %v", data, err)
			}
		})
	}
}

// The fallback must not silently overwrite an unrelated existing target.
func TestRenameNativeSameDirFallbackKeepsNoClobber(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldPath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("victim"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubRenameNoReplace(t, unix.EINVAL)

	err := renameNativeSameDir(oldPath, newPath, false)
	if err == nil {
		t.Fatal("renameNativeSameDir() = nil, want an error")
	}
	if !errors.Is(err, unix.EEXIST) {
		t.Fatalf("renameNativeSameDir() = %v, want EEXIST", err)
	}
	if data, readErr := os.ReadFile(newPath); readErr != nil || string(data) != "victim" {
		t.Fatalf("target was clobbered: %q, %v", data, readErr)
	}
	if data, readErr := os.ReadFile(oldPath); readErr != nil || string(data) != "source" {
		t.Fatalf("source was consumed: %q, %v", data, readErr)
	}
}

// A case-only rename on a case-insensitive filesystem surfaces as EEXIST from
// the flagged syscall, because the destination resolves to the source itself.
func TestRenameNativeSameDirCaseOnlyRenameFallsBackOnEEXIST(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "notes.txt")
	newPath := filepath.Join(dir, "Notes.txt")
	if err := os.WriteFile(oldPath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubRenameNoReplace(t, unix.EEXIST)

	if err := renameNativeSameDir(oldPath, newPath, true); err != nil {
		t.Fatalf("case-only rename = %v, want nil", err)
	}
	if data, err := os.ReadFile(newPath); err != nil || string(data) != "data" {
		t.Fatalf("target = %q, %v", data, err)
	}
}

// EEXIST without a case-only rename is a genuine conflict and must be reported.
func TestRenameNativeSameDirReportsRealEEXIST(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldPath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubRenameNoReplace(t, unix.EEXIST)

	err := renameNativeSameDir(oldPath, newPath, false)
	if !errors.Is(err, unix.EEXIST) {
		t.Fatalf("renameNativeSameDir() = %v, want EEXIST", err)
	}
	if _, statErr := os.Lstat(oldPath); statErr != nil {
		t.Fatalf("source missing after refused rename: %v", statErr)
	}
}

// A failure that is neither "flag unsupported" nor EEXIST must propagate.
func TestRenameNativeSameDirPropagatesOtherErrors(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(oldPath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	stubRenameNoReplace(t, unix.EACCES)

	err := renameNativeSameDir(oldPath, newPath, false)
	if !errors.Is(err, unix.EACCES) {
		t.Fatalf("renameNativeSameDir() = %v, want EACCES", err)
	}
}
