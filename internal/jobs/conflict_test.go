package jobs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConflictRenameRetriesInvalidAndOccupiedNames(t *testing.T) {
	srcDir, dstDir := t.TempDir(), t.TempDir()
	src := filepath.Join(srcDir, "file.txt")
	dst := filepath.Join(dstDir, "file.txt")
	occupied := filepath.Join(dstDir, "occupied.txt")
	for _, path := range []string{src, dst, occupied} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	steps := []struct {
		destination   string
		defaultAction ConflictAction
		newName       string
	}{
		{dst, ConflictOverwriteIfNewer, "../invalid"},
		{dst, ConflictRename, "occupied.txt"},
		{occupied, ConflictRename, "copied.txt"},
	}
	calls := 0
	job := &Job{Type: TypeCopy, ctx: context.Background()}
	job.Resolver = func(_ context.Context, req ConflictRequest) ConflictResolution {
		if calls >= len(steps) {
			t.Fatal("unexpected additional conflict prompt")
		}
		step := steps[calls]
		calls++
		if req.Destination != step.destination || req.DefaultAction != step.defaultAction {
			t.Fatalf("prompt destination=%q default=%q, want %q %q", req.Destination, req.DefaultAction, step.destination, step.defaultAction)
		}
		return ConflictResolution{Action: ConflictRename, NewName: step.newName}
	}
	if err := transferOneSource(job, src, dstDir); err != nil {
		t.Fatal(err)
	}
	if calls != len(steps) {
		t.Fatalf("prompt count = %d, want %d", calls, len(steps))
	}
	for _, path := range []string{src, dst, occupied, filepath.Join(dstDir, "copied.txt")} {
		want := "file.txt"
		if path == occupied {
			want = "occupied.txt"
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("file %q = %q, %v; want %q", path, got, err, want)
		}
	}
}

func TestConflictApplyToRestPreservesActionSemantics(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	dstPath, err := resolveExecutionPath(dst)
	if err != nil {
		t.Fatal(err)
	}
	execCtx := newExecutionContext()
	defer execCtx.close()
	for _, tt := range []struct {
		action        ConflictAction
		wantDefault   ConflictAction
		wantSkip      bool
		wantOverwrite bool
		wantErr       bool
	}{
		{ConflictSkip, ConflictSkip, true, false, false},
		{ConflictOverwrite, ConflictOverwrite, false, true, false},
		{ConflictOverwriteIfNewer, ConflictOverwriteIfNewer, true, false, false},
		{ConflictAutoSuffix, ConflictAutoSuffix, false, false, false},
		{"", ConflictAutoSuffix, false, false, false},
		{ConflictRename, ConflictRename, false, false, false},
		{ConflictCancelJob, ConflictRename, false, false, true},
		{"unknown", ConflictRename, false, false, true},
	} {
		t.Run(string(tt.action), func(t *testing.T) {
			job := &Job{Type: TypeCopy, ctx: context.Background(), conflictDefault: ConflictRename}
			job.Resolver = func(context.Context, ConflictRequest) ConflictResolution {
				return ConflictResolution{Action: tt.action, ApplyToRest: true, NewName: "renamed.txt"}
			}
			_, skipped, overwrite, err := askDestinationConflict(job, execCtx, dstPath, dstPath, info, info)
			if (err != nil) != tt.wantErr || skipped != tt.wantSkip || overwrite != tt.wantOverwrite {
				t.Fatalf("result skip=%t overwrite=%t err=%v, want %t %t error=%t", skipped, overwrite, err, tt.wantSkip, tt.wantOverwrite, tt.wantErr)
			}
			if job.conflictDefault != tt.wantDefault {
				t.Fatalf("default = %q, want %q", job.conflictDefault, tt.wantDefault)
			}
		})
	}
}
