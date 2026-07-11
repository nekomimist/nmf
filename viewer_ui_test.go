package main

import "testing"

func TestViewerLoadGenerationRejectsCanceledAndStaleResults(t *testing.T) {
	fm := &FileManager{}
	first := fm.beginViewerLoad()
	second := fm.beginViewerLoad()

	if fm.invalidateViewerLoad(first) {
		t.Fatal("stale viewer cancellation should not cancel the active request")
	}
	if fm.finishViewerLoad(first) {
		t.Fatal("stale viewer result should not finish")
	}
	if !fm.invalidateViewerLoad(second) {
		t.Fatal("active viewer cancellation should succeed")
	}
	if fm.finishViewerLoad(second) {
		t.Fatal("canceled viewer result should not finish")
	}
}
