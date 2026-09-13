//go:build linux

package jobs

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

// Each run owns a fresh directory so failures cannot overwrite another run's data.
func smbTestDirectory(t *testing.T) (executionPath, *executionContext) {
	t.Helper()
	url := strings.TrimSpace(os.Getenv("NMF_SMB_TEST_DIR"))
	if url == "" {
		t.Skip("set NMF_SMB_TEST_DIR to run SMB integration tests")
	}
	root, err := resolveExecutionPath(strings.TrimRight(url, "/") + "/nmf-test-" + rand.Text())
	if err != nil {
		t.Fatal(err)
	}
	if root.backend != backendSMB {
		t.Fatal("direct SMB backend required")
	}
	ctx := newExecutionContext()
	if err := ensureDir(ctx, root, 0700); err != nil {
		ctx.close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := deletePermanentResolved(&Job{ctx: context.Background()}, ctx, root); err != nil {
			t.Errorf("remove test directory: %v", err)
		}
		_ = ctx.close()
	})
	return root, ctx
}

func smbTestWrite(t *testing.T, ctx *executionContext, path executionPath, data string) {
	t.Helper()
	out, err := createExclusivePath(ctx, path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := io.WriteString(out, data)
	if err := errors.Join(writeErr, out.Close()); err != nil {
		t.Fatal(err)
	}
}

func smbTestRead(t *testing.T, ctx *executionContext, path executionPath) string {
	t.Helper()
	in, err := openReadPath(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	data, err := io.ReadAll(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type interruptedSMBReplacement struct {
	fileinfo.SMBSession
	tmp, dst string
	mode     string
	backup   string
}

func (s *interruptedSMBReplacement) Rename(from, to string) error {
	// Inject transport failure after the real backup rename. Verification
	// uses an independent live session to check the retained server-side data.
	if s.backup != "" && s.mode == "disconnect" {
		return net.ErrClosed
	}
	if from == s.tmp && to == s.dst && s.backup != "" && s.mode == "restore" {
		return os.ErrPermission
	}
	err := s.SMBSession.Rename(from, to)
	if err == nil && from == s.dst {
		s.backup = to
	}
	return err
}

func TestSMBReplacementIntegration(t *testing.T) {
	for _, mode := range []string{"no overwrite", "success", "restore", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			root, verifyCtx := smbTestDirectory(t)
			tmp, dst := joinPath(root, "temporary"), joinPath(root, "destination")
			smbTestWrite(t, verifyCtx, tmp, "new")
			smbTestWrite(t, verifyCtx, dst, "old")
			session, err := root.smbOpener.OpenSession()
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			ops := &interruptedSMBReplacement{SMBSession: session, tmp: tmp.path, dst: dst.path, mode: mode}
			publishDst := dst
			publishDst.smb, publishDst.smbOpener = ops, nil
			err = replacePath(&Job{ctx: t.Context()}, newExecutionContext(), tmp, publishDst, mode != "no overwrite")
			switch mode {
			case "success":
				if err != nil || smbTestRead(t, verifyCtx, dst) != "new" {
					t.Fatalf("replacement failed: %v", err)
				}
			case "no overwrite", "restore":
				if err == nil || smbTestRead(t, verifyCtx, dst) != "old" {
					t.Fatalf("old destination lost: %v", err)
				}
			case "disconnect":
				backup := dst
				backup.path = ops.backup
				if err == nil || ops.backup == "" || !strings.Contains(err.Error(), backup.displayPath()) {
					t.Fatalf("backup path not reported: %v", err)
				}
				if smbTestRead(t, verifyCtx, backup) != "old" {
					t.Fatal("backup content lost after disconnect")
				}
			}
			entries, readErr := readDir(verifyCtx, root)
			want := 2
			if mode == "success" {
				want = 1
			}
			if readErr != nil || len(entries) != want {
				t.Fatalf("unexpected leftover files: %v, %v", entries, readErr)
			}
		})
	}
}
