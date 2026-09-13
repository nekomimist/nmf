package jobs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestCopyFromArchiveToLocalDirectory(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{
		"dir/file.txt": "copied from archive",
	})
	dstDir := t.TempDir()
	job := &Job{Type: TypeCopy, ctx: t.Context()}

	src := fileinfo.JoinPath(fileinfo.ArchiveRootPath(archivePath), "dir")
	if err := transferOneSource(job, src, dstDir); err != nil {
		t.Fatalf("transferOneSource returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dstDir, "dir", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile copied file returned error: %v", err)
	}
	if string(data) != "copied from archive" {
		t.Fatalf("copied content = %q", string(data))
	}
}

func TestMoveFromArchiveIsRejected(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{"file.txt": "data"})
	dstDir := t.TempDir()
	job := &Job{Type: TypeMove, ctx: t.Context()}

	src := fileinfo.JoinPath(fileinfo.ArchiveRootPath(archivePath), "file.txt")
	if err := transferOneSource(job, src, dstDir); err == nil {
		t.Fatal("transferOneSource move from archive returned nil error")
	}
}

func TestCopyFromArchiveRejectsUnsafeEntryName(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{
		`dir\evil.txt`: "bad",
	})
	dstDir := t.TempDir()
	job := &Job{Type: TypeCopy, ctx: t.Context()}

	src := fileinfo.ArchiveRootPath(archivePath)
	err := transferOneSource(job, src, dstDir)
	if !errors.Is(err, fileinfo.ErrUnsafeArchiveEntry) {
		t.Fatalf("transferOneSource unsafe archive error = %v, want ErrUnsafeArchiveEntry", err)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "evil.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe destination was created or stat failed unexpectedly: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "dir", "evil.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe nested destination was created or stat failed unexpectedly: %v", statErr)
	}
}

func TestExtractZipToNamedDirectory(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{
		"dir/file.txt": "extracted",
	})
	dstDir := t.TempDir()
	job := &Job{Type: TypeExtract, ctx: t.Context()}

	if err := extractArchivePath(job, newExecutionContext(), archivePath, mustResolveExecutionPath(t, dstDir)); err != nil {
		t.Fatalf("extractArchivePath returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dstDir, "sample", "dir", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile extracted file returned error: %v", err)
	}
	if string(data) != "extracted" {
		t.Fatalf("extracted content = %q", string(data))
	}
}

func TestExtractTarGzToNamedDirectory(t *testing.T) {
	archivePath := writeJobTestTarGz(t, map[string]string{
		"dir/file.txt": "tar extracted",
	})
	dstDir := t.TempDir()
	job := &Job{Type: TypeExtract, ctx: t.Context()}

	if err := extractArchivePath(job, newExecutionContext(), archivePath, mustResolveExecutionPath(t, dstDir)); err != nil {
		t.Fatalf("extractArchivePath returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dstDir, "sample", "dir", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile extracted tar file returned error: %v", err)
	}
	if string(data) != "tar extracted" {
		t.Fatalf("extracted tar content = %q", string(data))
	}
}

func TestExtractArchiveRejectsUnsafeEntryName(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{
		"../evil.txt": "bad",
	})
	dstDir := t.TempDir()
	job := &Job{Type: TypeExtract, ctx: t.Context()}

	err := extractArchivePath(job, newExecutionContext(), archivePath, mustResolveExecutionPath(t, dstDir))
	if !errors.Is(err, fileinfo.ErrUnsafeArchiveEntry) {
		t.Fatalf("extractArchivePath unsafe archive error = %v, want ErrUnsafeArchiveEntry", err)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "evil.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe destination was created or stat failed unexpectedly: %v", statErr)
	}
}

func TestExtractArchiveCollisionRename(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{
		"file.txt": "archive",
	})
	dstDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dstDir, "sample"), 0755); err != nil {
		t.Fatalf("Mkdir sample returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "sample", "file.txt"), []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile existing returned error: %v", err)
	}
	job := &Job{
		Type: TypeExtract,
		ctx:  context.Background(),
		Resolver: func(context.Context, ConflictRequest) ConflictResolution {
			return ConflictResolution{Action: ConflictRename, NewName: "file-renamed.txt"}
		},
	}

	if err := extractArchivePath(job, newExecutionContext(), archivePath, mustResolveExecutionPath(t, dstDir)); err != nil {
		t.Fatalf("extractArchivePath returned error: %v", err)
	}
	existing, err := os.ReadFile(filepath.Join(dstDir, "sample", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile existing returned error: %v", err)
	}
	if string(existing) != "existing" {
		t.Fatalf("existing content = %q", string(existing))
	}
	renamed, err := os.ReadFile(filepath.Join(dstDir, "sample", "file-renamed.txt"))
	if err != nil {
		t.Fatalf("ReadFile renamed returned error: %v", err)
	}
	if string(renamed) != "archive" {
		t.Fatalf("renamed content = %q", string(renamed))
	}
}

func writeJobTestZip(t *testing.T, files map[string]string) string {
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

func writeJobTestTarGz(t *testing.T, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "sample.tar.gz")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create tar.gz returned error: %v", err)
	}
	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		dir := filepath.ToSlash(filepath.Dir(name))
		if dir != "." {
			parts := strings.Split(dir, "/")
			current := ""
			for _, part := range parts {
				if current == "" {
					current = part
				} else {
					current += "/" + part
				}
				if err := tw.WriteHeader(&tar.Header{Name: current + "/", Typeflag: tar.TypeDir, Mode: 0755}); err != nil {
					t.Fatalf("WriteHeader dir %q returned error: %v", current, err)
				}
			}
		}
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader(%q) returned error: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) returned error: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close returned error: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip Close returned error: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("file Close returned error: %v", err)
	}
	return archivePath
}

func mustResolveExecutionPath(t *testing.T, p string) executionPath {
	t.Helper()
	resolved, err := resolveExecutionPath(p)
	if err != nil {
		t.Fatalf("resolveExecutionPath(%q) returned error: %v", p, err)
	}
	return resolved
}

func TestExtractRejectsExistingDirectorySymlink(t *testing.T) {
	archive := writeJobTestZip(t, map[string]string{"dir/file.txt": "archive"})
	dst, outside := t.TempDir(), t.TempDir()
	root := filepath.Join(dst, "sample")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "dir")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	job := &Job{Type: TypeExtract, ctx: t.Context()}
	if err := extractArchivePath(job, newExecutionContext(), archive, mustResolveExecutionPath(t, dst)); err == nil {
		t.Fatal("extraction followed external symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside file = %v", err)
	}
}

func TestExtractConfinesWritesAfterDestinationChanges(t *testing.T) {
	for _, swapRoot := range []bool{false, true} {
		t.Run(fmt.Sprint("root=", swapRoot), func(t *testing.T) {
			archive := writeJobTestZip(t, map[string]string{"dir/file.txt": "archive"})
			dst, outside := t.TempDir(), t.TempDir()
			root := filepath.Join(dst, "sample")
			child := filepath.Join(root, "dir")
			if err := os.MkdirAll(child, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(child, "file.txt"), []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}
			// Check symlink support before the operation mutates the fixture.
			probe := filepath.Join(dst, "probe")
			if err := os.Symlink(outside, probe); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if err := os.Remove(probe); err != nil {
				t.Fatal(err)
			}
			job := &Job{Type: TypeExtract, ctx: t.Context(), Resolver: func(_ context.Context, _ ConflictRequest) ConflictResolution {
				target := child
				if swapRoot {
					target = root
				}
				if err := os.Rename(target, target+"-saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, target); err != nil {
					t.Fatal(err)
				}
				return ConflictResolution{Action: ConflictOverwrite}
			}}
			err := extractArchivePath(job, newExecutionContext(), archive, mustResolveExecutionPath(t, dst))
			if swapRoot {
				if err != nil {
					t.Fatal(err)
				}
				if data, err := os.ReadFile(filepath.Join(root+"-saved", "dir", "file.txt")); err != nil || string(data) != "archive" {
					t.Fatalf("anchored output = %q, %v", data, err)
				}
			} else if err == nil {
				t.Fatal("extraction accepted replaced intermediate directory")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatalf("outside directory changed: %v, %v", entries, err)
			}
		})
	}
}

type archiveParentSMB struct {
	fakeSMBOps
	directory string
}

func (s archiveParentSMB) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(filepath.Join(s.directory, strings.TrimPrefix(path, "/")))
}
func (s archiveParentSMB) Mkdir(path string, mode os.FileMode) error {
	return os.Mkdir(filepath.Join(s.directory, strings.TrimPrefix(path, "/")), mode)
}

func TestSMBArchiveParentsRejectExistingLinks(t *testing.T) {
	dst, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(dst, "sample"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dst, "sample", "dir")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	root := executionPath{backend: backendSMB, path: "/sample", smb: archiveParentSMB{directory: dst}}
	if err := ensureSMBArchiveParents(newExecutionContext(), root, joinPath(joinPath(root, "dir"), "nested")); err == nil {
		t.Fatal("SMB extraction accepted link parent")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside entries = %v, %v", entries, err)
	}
}
