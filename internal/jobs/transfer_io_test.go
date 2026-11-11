package jobs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCopyReaderCleanupAndPublicationOrder(t *testing.T) {
	failure := errors.New("injected transfer failure")
	for _, tt := range []struct {
		name          string
		readErr       error
		writeErr      error
		closeErr      error
		publishErr    error
		timestampErr  error
		cancel        bool
		wantErr       error
		wantErrorPath string
		wantEvents    []string
	}{
		{name: "success", wantEvents: []string{"open", "write", "close", "publish", "timestamp"}},
		{name: "data with EOF", readErr: io.EOF, wantEvents: []string{"open", "write", "close", "publish", "timestamp"}},
		{name: "cancel", cancel: true, wantErr: errCanceled, wantEvents: []string{"open", "close", "remove"}},
		{name: "read failure after data", readErr: failure, wantErr: failure, wantErrorPath: "source", wantEvents: []string{"open", "write", "close", "remove"}},
		{name: "write failure", writeErr: failure, wantErr: failure, wantErrorPath: "temporary", wantEvents: []string{"open", "write", "close", "remove"}},
		{name: "close failure", closeErr: failure, wantErr: failure, wantErrorPath: "temporary", wantEvents: []string{"open", "write", "close", "remove"}},
		{name: "publish failure", publishErr: failure, wantErr: failure, wantErrorPath: "destination", wantEvents: []string{"open", "write", "close", "publish", "remove"}},
		{name: "timestamp failure retains published file", timestampErr: failure, wantErr: failure, wantErrorPath: "destination", wantEvents: []string{"open", "write", "close", "publish", "timestamp"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ops := &transferOutputSMB{writeErr: tt.writeErr, closeErr: tt.closeErr, publishErr: tt.publishErr, timestampErr: tt.timestampErr}
			dst := executionPath{backend: backendSMB, path: "/file.txt", smb: ops, smbDisplayRoot: "smb://host/share"}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			job := &Job{Type: TypeCopy, ctx: ctx, Options: TransferOptions{PreserveTimestamps: true}}
			if tt.cancel {
				job.progressNotify = cancel
			}
			var in io.Reader = strings.NewReader("data")
			if tt.readErr != nil {
				in = transferReadFunc(func(p []byte) (int, error) { return copy(p, "data"), tt.readErr })
			}
			execCtx := newExecutionContext()
			defer execCtx.close()
			err := copyReaderWithCancel(job, execCtx, in, "source", dst, virtualFileInfo{size: 4, mode: 0644}, false)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("copy error = %v, want %v", err, tt.wantErr)
			}
			wantPath := tt.wantErrorPath
			switch wantPath {
			case "temporary":
				tmp := dst
				tmp.path = ops.tempPath
				wantPath = tmp.displayPath()
			case "destination":
				wantPath = dst.displayPath()
			}
			if got := failingPath(err); got != wantPath {
				t.Fatalf("error path = %q, want %q", got, wantPath)
			}
			if !reflect.DeepEqual(ops.events, tt.wantEvents) {
				t.Fatalf("output operations = %v, want %v", ops.events, tt.wantEvents)
			}
			if tt.wantErr == nil && (ops.output.String() != "data" || job.Snapshot().CurrentBytes != 4) {
				t.Fatal("successful transfer lost data or progress")
			}
		})
	}
}

type transferReadFunc func([]byte) (int, error)

func (f transferReadFunc) Read(p []byte) (int, error) { return f(p) }

// transferOutputSMB records output lifecycle operations without a remote server.
type transferOutputSMB struct {
	fakeSMBOps
	output       bytes.Buffer
	tempPath     string
	events       []string
	writeErr     error
	closeErr     error
	publishErr   error
	timestampErr error
}

func (s *transferOutputSMB) OpenFile(path string, _ int, _ os.FileMode) (io.ReadWriteCloser, error) {
	s.tempPath = path
	s.events = append(s.events, "open")
	return &transferOutputHandle{backend: s}, nil
}

func (s *transferOutputSMB) Rename(_, _ string) error {
	s.events = append(s.events, "publish")
	return s.publishErr
}

func (s *transferOutputSMB) Chtimes(_ string, _, _ time.Time) error {
	s.events = append(s.events, "timestamp")
	return s.timestampErr
}

func (s *transferOutputSMB) Remove(path string) error {
	if path != s.tempPath {
		s.events = append(s.events, "removed unrelated path: "+path)
	}
	s.events = append(s.events, "remove")
	// Cleanup failure must not replace the original transfer error.
	return errors.New("injected cleanup failure")
}

type transferOutputHandle struct{ backend *transferOutputSMB }

func (h *transferOutputHandle) Read([]byte) (int, error) { return 0, io.EOF }

func (h *transferOutputHandle) Write(p []byte) (int, error) {
	h.backend.events = append(h.backend.events, "write")
	if h.backend.writeErr != nil {
		return 0, h.backend.writeErr
	}
	return h.backend.output.Write(p)
}

func (h *transferOutputHandle) Close() error {
	h.backend.events = append(h.backend.events, "close")
	return h.backend.closeErr
}
