//go:build linux

package fileinfo

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// This opt-in test pauses only the explicitly supplied disposable Samba
// container. It verifies blocked reads and cleanup without changing the host network.
func TestSMBReadCancellationWithPausedServer(t *testing.T) {
	url := strings.TrimSpace(os.Getenv("NMF_SMB_TEST_DIR"))
	container := strings.TrimSpace(os.Getenv("NMF_SMB_TEST_CONTAINER"))
	if url == "" || container == "" {
		t.Skip("set NMF_SMB_TEST_DIR and NMF_SMB_TEST_CONTAINER for the disposable Samba fixture")
	}
	vfs, parsed, err := ResolveRead(url)
	if err != nil {
		t.Fatal(err)
	}
	smb, ok := vfs.(SMBFS)
	if !ok {
		t.Fatal("direct SMB backend required")
	}
	addresses, err := exec.Command("docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}", container).Output()
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, address := range strings.Fields(string(addresses)) {
		matched = matched || address == parsed.Host
	}
	if !matched {
		t.Fatal("SMB host must be the explicitly selected container's IP address")
	}
	path := smb.Join(parsed.Native, "nmf-cancel-"+rand.Text())
	out, err := smb.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := io.WriteString(out, "read must wait for the server")
	closeErr := out.Close()
	t.Cleanup(func() {
		if err := smb.Remove(path); err != nil {
			t.Errorf("remove fixture: %v", err)
		}
	})
	if writeErr != nil || closeErr != nil {
		t.Fatalf("write fixture: %v, %v", writeErr, closeErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	in, err := smb.OpenContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if output, err := exec.Command("docker", "pause", container).CombinedOutput(); err != nil {
		t.Fatalf("pause server: %v: %s", err, output)
	}
	t.Cleanup(func() {
		if output, err := exec.Command("docker", "unpause", container).CombinedOutput(); err != nil {
			t.Errorf("unpause server: %v: %s", err, output)
		}
	})
	readDone := make(chan error, 1)
	go func() { _, err := in.Read(make([]byte, 64)); readDone <- err }()
	select {
	case err := <-readDone:
		t.Fatalf("read completed while server paused: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-readDone:
		if err == nil {
			t.Fatal("canceled read succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled read remained blocked")
	}
	closed := make(chan struct{})
	go func() { _ = in.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("reader cleanup remained blocked")
	}
}
