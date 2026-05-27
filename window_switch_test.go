package main

import "testing"

func TestSelectWindowSwitchCandidateUsesNearestHorizontalRect(t *testing.T) {
	candidates := []windowSwitchCandidate{
		{rect: windowSwitchRect{Left: 0, Top: 0, Right: 100, Bottom: 100}, hasRect: true},
		{rect: windowSwitchRect{Left: 250, Top: 300, Right: 350, Bottom: 400}, hasRect: true},
		{rect: windowSwitchRect{Left: 120, Top: 1000, Right: 220, Bottom: 1100}, hasRect: true},
		{rect: windowSwitchRect{Left: -140, Top: 0, Right: -40, Bottom: 100}, hasRect: true},
	}

	right, ok := selectWindowSwitchCandidate(candidates, 0, windowSwitchRight)
	if !ok || right != 2 {
		t.Fatalf("right selection = %d, %t, want 2, true", right, ok)
	}

	left, ok := selectWindowSwitchCandidate(candidates, 0, windowSwitchLeft)
	if !ok || left != 3 {
		t.Fatalf("left selection = %d, %t, want 3, true", left, ok)
	}
}

func TestSelectWindowSwitchCandidateSupportsNegativeMonitorCoordinates(t *testing.T) {
	candidates := []windowSwitchCandidate{
		{rect: windowSwitchRect{Left: -1900, Top: 0, Right: -1500, Bottom: 400}, hasRect: true},
		{rect: windowSwitchRect{Left: -1200, Top: 0, Right: -800, Bottom: 400}, hasRect: true},
		{rect: windowSwitchRect{Left: 100, Top: 0, Right: 500, Bottom: 400}, hasRect: true},
	}

	left, ok := selectWindowSwitchCandidate(candidates, 1, windowSwitchLeft)
	if !ok || left != 0 {
		t.Fatalf("left selection = %d, %t, want 0, true", left, ok)
	}

	right, ok := selectWindowSwitchCandidate(candidates, 1, windowSwitchRight)
	if !ok || right != 2 {
		t.Fatalf("right selection = %d, %t, want 2, true", right, ok)
	}
}

func TestSelectWindowSwitchCandidateDoesNothingAtEdge(t *testing.T) {
	candidates := []windowSwitchCandidate{
		{rect: windowSwitchRect{Left: 0, Top: 0, Right: 100, Bottom: 100}, hasRect: true},
		{rect: windowSwitchRect{Left: 200, Top: 0, Right: 300, Bottom: 100}, hasRect: true},
	}

	index, ok := selectWindowSwitchCandidate(candidates, 0, windowSwitchLeft)
	if ok {
		t.Fatalf("left edge selection = %d, %t, want no selection", index, ok)
	}

	index, ok = selectWindowSwitchCandidate(candidates, 1, windowSwitchRight)
	if ok {
		t.Fatalf("right edge selection = %d, %t, want no selection", index, ok)
	}
}

func TestSelectWindowSwitchCandidateFallsBackToOrderWithoutCurrentRect(t *testing.T) {
	candidates := []windowSwitchCandidate{
		{rect: windowSwitchRect{Left: 0, Top: 0, Right: 100, Bottom: 100}, hasRect: true},
		{},
		{rect: windowSwitchRect{Left: 200, Top: 0, Right: 300, Bottom: 100}, hasRect: true},
	}

	left, ok := selectWindowSwitchCandidate(candidates, 1, windowSwitchLeft)
	if !ok || left != 0 {
		t.Fatalf("left fallback selection = %d, %t, want 0, true", left, ok)
	}

	right, ok := selectWindowSwitchCandidate(candidates, 1, windowSwitchRight)
	if !ok || right != 2 {
		t.Fatalf("right fallback selection = %d, %t, want 2, true", right, ok)
	}
}

func TestSelectWindowByOrderDoesNotWrap(t *testing.T) {
	candidates := []windowSwitchCandidate{{}, {}, {}}

	left, ok := selectWindowByOrder(candidates, 1, windowSwitchLeft)
	if !ok || left != 0 {
		t.Fatalf("left order selection = %d, %t, want 0, true", left, ok)
	}

	right, ok := selectWindowByOrder(candidates, 1, windowSwitchRight)
	if !ok || right != 2 {
		t.Fatalf("right order selection = %d, %t, want 2, true", right, ok)
	}

	if index, ok := selectWindowByOrder(candidates, 0, windowSwitchLeft); ok {
		t.Fatalf("left edge order selection = %d, %t, want no selection", index, ok)
	}
	if index, ok := selectWindowByOrder(candidates, 2, windowSwitchRight); ok {
		t.Fatalf("right edge order selection = %d, %t, want no selection", index, ok)
	}
}

func TestReopenPathStackUsesMostRecentlyClosedPath(t *testing.T) {
	resetFileManagerWindowTestRegistry(t)

	recordReopenPath("/first")
	recordReopenPath("")
	recordReopenPath("/second")

	path, ok := nextReopenPath()
	if !ok || path != "/second" {
		t.Fatalf("first reopen path = %q, %t, want /second, true", path, ok)
	}

	path, ok = nextReopenPath()
	if !ok || path != "/first" {
		t.Fatalf("second reopen path = %q, %t, want /first, true", path, ok)
	}

	path, ok = nextReopenPath()
	if ok || path != "" {
		t.Fatalf("empty reopen path = %q, %t, want empty, false", path, ok)
	}
}
