package main

import "testing"

func TestViewerLoadGenerationRejectsCanceledAndStaleResults(t *testing.T) {
	fm := &FileManager{}
	first, firstCtx := fm.beginViewerLoad()
	second, _ := fm.beginViewerLoad()
	if firstCtx.Err() == nil {
		t.Fatal("starting a replacement viewer load should cancel the previous context")
	}

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
