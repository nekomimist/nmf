package jobs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCopyPreservesExistingPartFile(t *testing.T) {
	for _, cancelCopy := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "canceled"}[cancelCopy], func(t *testing.T) {
			srcDir, dstDir := t.TempDir(), t.TempDir()
			src := filepath.Join(srcDir, "file.txt")
			part := filepath.Join(dstDir, "file.txt.part")
			if err := os.WriteFile(src, []byte("source"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(part, []byte("unrelated"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			job := &Job{Type: TypeCopy, ctx: ctx}
			if cancelCopy {
				job.progressNotify = cancel
			}
			err := transferOneSource(job, src, dstDir)
			if cancelCopy && err != errCanceled {
				t.Fatalf("copy error = %v, want canceled", err)
			}
			if !cancelCopy && err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(part); err != nil || string(data) != "unrelated" {
				t.Fatalf("existing part = %q, %v", data, err)
			}
			entries, err := os.ReadDir(dstDir)
			if err != nil {
				t.Fatal(err)
			}
			want := 2
			if cancelCopy {
				want = 1
			}
			if len(entries) != want {
				t.Fatalf("destination entries = %v, want %d (no temporary output)", entries, want)
			}
		})
	}
}

func TestCopyDoesNotFollowExistingPartSymlink(t *testing.T) {
	srcDir, dstDir := t.TempDir(), t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	victim := filepath.Join(srcDir, "victim")
	if err := os.WriteFile(src, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(victim, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	part := filepath.Join(dstDir, "file.txt.part")
	if err := os.Symlink(victim, part); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := transferOneSource(&Job{Type: TypeCopy, ctx: t.Context()}, src, dstDir); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "keep" {
		t.Fatalf("symlink target = %q, %v", data, err)
	}
	if target, err := os.Readlink(part); err != nil || target != victim {
		t.Fatalf("existing symlink = %q, %v", target, err)
	}
}

type exclusiveCreateSMB struct {
	fakeSMBOps
	flags int
}

func (s *exclusiveCreateSMB) OpenFile(_ string, flags int, _ os.FileMode) (io.ReadWriteCloser, error) {
	s.flags = flags
	return nopReadWriteCloser{}, nil
}

func TestTransferTempUsesExclusiveSMBCreate(t *testing.T) {
	ops := &exclusiveCreateSMB{}
	dst := executionPath{backend: backendSMB, path: "/file.txt", smb: ops}
	tmp, out, err := createTransferTemp(newExecutionContext(), dst, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if ops.flags&os.O_EXCL == 0 || ops.flags&os.O_TRUNC != 0 {
		t.Fatalf("unsafe SMB create flags: %d", ops.flags)
	}
	if tmp.path == dst.path+".part" {
		t.Fatal("predictable temporary path")
	}
}

func TestCopyDirectoryIntoDescendantIsRejected(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(fmt.Sprint("alias=", alias), func(t *testing.T) {
			root := t.TempDir()
			src := filepath.Join(root, "source")
			dst := filepath.Join(src, "child")
			if err := os.MkdirAll(dst, 0700); err != nil {
				t.Fatal(err)
			}
			if alias {
				link := filepath.Join(root, "alias")
				if err := os.Symlink(dst, link); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				dst = link
			}
			err := transferOneSource(&Job{Type: TypeCopy, ctx: t.Context()}, src, dst)
			if err == nil || !strings.Contains(err.Error(), "cannot copy a directory into itself") {
				t.Fatalf("copy error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(dst, "source")); !os.IsNotExist(err) {
				t.Fatalf("copy created a destination: %v", err)
			}
		})
	}
}

func TestCopyPreservesExistingDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	srcDir, dstDir := t.TempDir(), t.TempDir()
	if err := os.Chmod(dstDir, 0700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(srcDir, "file.txt")
	if err := os.WriteFile(src, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := transferOneSource(&Job{Type: TypeCopy, ctx: t.Context()}, src, dstDir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dstDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("directory permissions = %04o, want 0700", info.Mode().Perm())
	}
}
