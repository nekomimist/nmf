package fileinfo

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type stagingVFS struct {
	LocalFS
	open func(context.Context) (io.ReadCloser, error)
}

func (v stagingVFS) Open(string) (io.ReadCloser, error) {
	return nil, errors.New("staging dropped the read context")
}

func (v stagingVFS) OpenContext(ctx context.Context, _ string) (io.ReadCloser, error) {
	return v.open(ctx)
}

type stalledArchiveReader struct {
	ctx     context.Context
	started chan struct{}
	closed  chan struct{}
	read    bool
}

func (r *stalledArchiveReader) Read(p []byte) (int, error) {
	if !r.read {
		r.read = true
		return copy(p, "partial archive"), nil
	}
	close(r.started)
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

func (r *stalledArchiveReader) Close() error { close(r.closed); return nil }

func TestRemoteArchiveStagingCancelsWithoutBlockingOtherDownloads(t *testing.T) {
	tmp := t.TempDir()
	for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
		t.Setenv(key, tmp)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader := &stalledArchiveReader{started: make(chan struct{}), closed: make(chan struct{})}
	vfs := stagingVFS{open: func(ctx context.Context) (io.ReadCloser, error) {
		reader.ctx = ctx
		return reader, nil
	}}
	first := make(chan error, 1)
	go func() {
		_, _, err := stageArchiveSource(ctx, vfs, "first.zip", ".zip")
		first <- err
	}()
	select {
	case <-reader.started:
	case <-time.After(5 * time.Second):
		t.Fatal("first download did not start")
	}
	second := make(chan error, 1)
	go func() {
		vfs := stagingVFS{open: func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("second archive")), nil
		}}
		path, _, err := stageArchiveSource(ctx, vfs, "second.zip", ".zip")
		if err == nil {
			var data []byte
			data, err = os.ReadFile(path)
			if err == nil && string(data) != "second archive" {
				err = errors.New("second download content mismatch")
			}
			_ = os.Remove(path)
		}
		second <- err
	}()
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second download blocked behind the stalled download")
	}
	cancel()
	select {
	case err := <-first:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("staging error = %v, want cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("staging did not stop after cancellation")
	}
	select {
	case <-reader.closed:
	default:
		t.Fatal("canceled download reader was not closed")
	}
	entries, err := os.ReadDir(tmp)
	if err != nil || len(entries) != 0 {
		t.Fatalf("staging left temporary output: %v, %v", entries, err)
	}
}

func TestArchiveStagingHonorsCanceledContextBeforeOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := NewArchiveVFSContext(ctx, "smb://unused/share/archive.zip")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("archive open error = %v, want cancellation", err)
	}
	err = ExtractArchive(ctx, "smb://unused/share/archive.zip", func(context.Context, ArchiveEntry) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("archive extraction error = %v, want cancellation", err)
	}
}
