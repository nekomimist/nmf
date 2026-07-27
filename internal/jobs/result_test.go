package jobs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTransferDirectoryRecordsOnlyTopLevelResult(t *testing.T) {
	srcParent := t.TempDir()
	src := filepath.Join(srcParent, "source")
	if err := os.MkdirAll(filepath.Join(src, "child"), 0755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "child", "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	dest := t.TempDir()
	job := &Job{Type: TypeCopy, ctx: t.Context()}

	if err := transferOneSource(job, src, dest); err != nil {
		t.Fatalf("transferOneSource: %v", err)
	}
	results := job.Snapshot().Results
	if len(results) != 1 {
		t.Fatalf("results = %#v, want exactly one top-level result", results)
	}
	if got, want := results[0].Destination, filepath.Join(dest, "source"); got != want {
		t.Fatalf("destination = %q, want %q", got, want)
	}
	if !results[0].SourceIsDir {
		t.Fatal("top-level copy result should be a directory")
	}
}

func TestExtractRecordsOnlyNewRootDirectory(t *testing.T) {
	archivePath := writeJobTestZip(t, map[string]string{"child/file.txt": "data"})
	dest := t.TempDir()
	job := &Job{Type: TypeExtract, ctx: t.Context()}

	if err := extractArchivePath(job, newExecutionContext(), archivePath, mustResolveExecutionPath(t, dest)); err != nil {
		t.Fatalf("extractArchivePath: %v", err)
	}
	results := job.Snapshot().Results
	if len(results) != 1 {
		t.Fatalf("results = %#v, want one extract-root result", results)
	}
	if got, want := results[0].Destination, filepath.Join(dest, "sample"); got != want {
		t.Fatalf("destination = %q, want %q", got, want)
	}
	if !results[0].DestinationCreated {
		t.Fatal("extract root should be marked newly created")
	}

	existingDest := t.TempDir()
	if err := os.Mkdir(filepath.Join(existingDest, "sample"), 0755); err != nil {
		t.Fatalf("Mkdir existing extract root: %v", err)
	}
	existingJob := &Job{Type: TypeExtract, ctx: t.Context()}
	if err := extractArchivePath(existingJob, newExecutionContext(), archivePath, mustResolveExecutionPath(t, existingDest)); err != nil {
		t.Fatalf("extract into existing root: %v", err)
	}
	if got := existingJob.Snapshot().Results; len(got) != 0 {
		t.Fatalf("existing extract root results = %#v, want none", got)
	}
}

func TestDeleteDirectoryRecordsTopLevelResult(t *testing.T) {
	root := filepath.Join(t.TempDir(), "delete-me")
	if err := os.MkdirAll(filepath.Join(root, "child"), 0755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	job := &Job{Type: TypeDelete, DeleteMode: DeleteModePermanent, Sources: []string{root}, ctx: t.Context(), TotalFiles: 1}

	if err := (&Manager{}).runDeleteJob(job); err != nil {
		t.Fatalf("runDeleteJob: %v", err)
	}
	results := job.Snapshot().Results
	if len(results) != 1 || !results[0].SourceIsDir || results[0].Source != root {
		t.Fatalf("results = %#v, want deleted top-level directory", results)
	}
}
