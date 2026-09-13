//go:build linux
// +build linux

package jobs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nmf/internal/fileinfo"
)

// TestSMBCopyRoundtrip requires an SMB test directory URL in:
//
//	NMF_SMB_TEST_DIR=smb://host/share/path
//
// The test is skipped when the env var is unset.
func TestSMBCopyRoundtrip(t *testing.T) {
	root, _ := smbTestDirectory(t)
	smbDir := strings.TrimRight(os.Getenv("NMF_SMB_TEST_DIR"), "/") + "/" + baseName(root)

	localTmp := t.TempDir()
	sourceName := "nmf_smb_roundtrip_src.txt"
	sourcePath := filepath.Join(localTmp, sourceName)
	payload := []byte("nmf-smb-roundtrip-" + time.Now().Format(time.RFC3339Nano))
	if err := os.WriteFile(sourcePath, payload, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	job := &Job{Type: TypeCopy}
	job.ctx, job.cancel = context.WithCancel(t.Context())
	defer job.cancel()

	// local -> smb
	if err := transferOneSource(job, sourcePath, smbDir); err != nil {
		t.Fatalf("copy local->smb failed: %v", err)
	}

	smbFileDisplay := strings.TrimRight(smbDir, "/") + "/" + sourceName
	smbInfo, err := fileinfo.StatPortable(smbFileDisplay)
	if err != nil {
		t.Fatalf("expected copied SMB file to exist: %v", err)
	}
	if smbInfo.IsDir() {
		t.Fatalf("expected copied SMB path to be a file")
	}

	// smb -> local
	restoreDir := filepath.Join(localTmp, "restore")
	if err := os.MkdirAll(restoreDir, 0755); err != nil {
		t.Fatalf("failed to create restore dir: %v", err)
	}
	if err := transferOneSource(job, smbFileDisplay, restoreDir); err != nil {
		t.Fatalf("copy smb->local failed: %v", err)
	}

	restoredPath := filepath.Join(restoreDir, sourceName)
	got, err := os.ReadFile(restoredPath)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("restored payload mismatch: got=%q want=%q", string(got), string(payload))
	}

}
