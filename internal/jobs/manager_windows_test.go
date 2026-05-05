//go:build windows

package jobs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPermanentDeleteDoesNotFollowJunction(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(targetDir, "keep.txt")
	if err := os.WriteFile(targetFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "junction")
	createTestJunction(t, link, targetDir)

	j := &Job{Type: TypeDelete, DeleteMode: DeleteModePermanent, ctx: context.Background()}
	if err := deletePermanentPath(j, newExecutionContext(), link); err != nil {
		t.Fatalf("deletePermanentPath returned error: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("junction still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Lstat(targetFile); err != nil {
		t.Fatalf("junction target should remain, got stat error: %v", err)
	}
}

func createTestJunction(t *testing.T, link string, target string) {
	t.Helper()
	output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("junction unavailable: %v: %s", err, string(output))
	}
}
