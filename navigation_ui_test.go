package main

import "testing"

func TestBuildEnhancedNavigationHistoryPathsPlacesPinnedBeforeHistory(t *testing.T) {
	got, unpinRemoves := buildEnhancedNavigationHistoryPaths(
		"/current",
		[]string{"/open"},
		[]string{"/pinned", "/also-history"},
		[]string{"/history", "/also-history"},
	)
	want := []string{"/current", "/open", "/pinned", "/also-history", "/history"}
	if len(got) != len(want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", got, want)
		}
	}
	if !unpinRemoves["/pinned"] {
		t.Fatalf("unpinRemoves = %#v, want pinned-only path removable", unpinRemoves)
	}
	if unpinRemoves["/also-history"] {
		t.Fatalf("unpinRemoves = %#v, history-backed pinned path should remain visible", unpinRemoves)
	}
}

func TestBuildEnhancedNavigationHistoryPathsDeduplicatesEarlierSources(t *testing.T) {
	got, unpinRemoves := buildEnhancedNavigationHistoryPaths(
		"/same",
		[]string{"/same", "/open"},
		[]string{"/open", "/pinned"},
		[]string{"/same", "/pinned", "/history"},
	)
	want := []string{"/same", "/open", "/pinned", "/history"}
	if len(got) != len(want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %#v, want %#v", got, want)
		}
	}
	if len(unpinRemoves) != 0 {
		t.Fatalf("unpinRemoves = %#v, want none removable", unpinRemoves)
	}
}
