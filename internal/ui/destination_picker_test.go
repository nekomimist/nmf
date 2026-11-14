package ui

import (
	"slices"
	"testing"
	"time"
)

func TestDestinationDialogsToggleHistoryOrder(t *testing.T) {
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	for _, op := range []Operation{OpCopy, OpMove, OpExtract, "compare"} {
		t.Run(string(op), func(t *testing.T) {
			candidates := []DestinationCandidate{
				{Path: "/tmp/unknown", OpenInWindow: true},
				{Path: "/tmp/project-open", OpenInWindow: true, LastUsed: now.Add(-time.Hour)},
				{Path: "/tmp/project-recent", LastUsed: now},
				{Path: "/tmp/tie", LastUsed: now.Add(-time.Hour)},
			}
			original := slices.Clone(candidates)
			var picker *destinationPicker
			if op == "compare" {
				d := NewCompareDialog("/src", 1, candidates, nil, func(string, ...interface{}) {})
				picker = &d.destinationPicker
			} else {
				d := NewCopyMoveDialog(op, []string{"file.txt"}, candidates, false, nil, func(string, ...interface{}) {})
				picker = &d.destinationPicker
			}
			if picker.orderRadio.Selected != navigationHistoryOrderFrecency || !picker.orderRadio.Required || !picker.orderRadio.Horizontal {
				t.Fatal("order radio should be horizontal, required, and default to Frecency")
			}
			if !slices.Equal(picker.filteredDest, original) {
				t.Fatalf("default destinations = %v, want original order", picker.filteredDest)
			}

			picker.ToggleHistoryOrder()
			wantRecent := []string{"/tmp/project-recent", "/tmp/project-open", "/tmp/tie", "/tmp/unknown"}
			if got := destinationPaths(picker.filteredDest); !slices.Equal(got, wantRecent) {
				t.Fatalf("last-access destinations = %v, want %v", got, wantRecent)
			}
			if picker.selectedPath != wantRecent[0] || !picker.filteredDest[1].OpenInWindow {
				t.Fatal("ordering should select the newest destination and retain open-window metadata")
			}
			if got, _ := picker.dataBinding.Get(); !slices.Equal(got, wantRecent) {
				t.Fatalf("displayed paths = %v, want %v", got, wantRecent)
			}

			picker.searchEntry.SetText("project")
			picker.ToggleHistoryOrder()
			wantFiltered := []string{"/tmp/project-open", "/tmp/project-recent"}
			if got := destinationPaths(picker.filteredDest); !slices.Equal(got, wantFiltered) {
				t.Fatalf("filtered frecency destinations = %v, want %v", got, wantFiltered)
			}
			if picker.GetSearchText() != "project" || picker.selectedPath != wantFiltered[0] {
				t.Fatal("toggle should keep the query and select the first reordered match")
			}
			picker.searchEntry.SetText("missing")
			picker.ToggleHistoryOrder()
			if len(picker.filteredDest) != 0 || picker.selectedPath != "" || picker.selectedIdx != -1 {
				t.Fatal("toggle with no matches should keep an empty selection")
			}
			picker.ClearSearch()
			picker.ToggleHistoryOrder()
			if !slices.Equal(picker.filteredDest, original) || !slices.Equal(candidates, original) {
				t.Fatal("returning to Frecency should restore the original candidate order")
			}
		})
	}
}

func TestDestinationPickerRefreshKeepsOrderSearchAndSelection(t *testing.T) {
	now := time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	d := NewCopyMoveDialog(OpCopy, nil, []DestinationCandidate{
		{Path: "/tmp/project-old", LastUsed: now.Add(-time.Hour)},
	}, false, nil, func(string, ...interface{}) {})
	d.searchEntry.SetText("project")
	d.ToggleHistoryOrder()

	updated := []DestinationCandidate{
		{Path: "/tmp/project-preferred", LastUsed: now.Add(-2 * time.Hour)},
		{Path: "/tmp/project-old", LastUsed: now},
		{Path: "/tmp/other", LastUsed: now},
	}
	d.SetDestinations(updated, "/tmp/project-preferred")
	if d.orderRadio.Selected != navigationHistoryOrderLastUsed || d.GetSearchText() != "project" {
		t.Fatal("refresh should keep the order mode and query")
	}
	want := []string{"/tmp/project-old", "/tmp/project-preferred"}
	if got := destinationPaths(d.filteredDest); !slices.Equal(got, want) {
		t.Fatalf("refreshed paths = %v, want %v", got, want)
	}
	if d.selectedPath != "/tmp/project-preferred" {
		t.Fatalf("selected path = %q, want preferred path", d.selectedPath)
	}

	d.SetDestinations(updated, "/tmp/other")
	if d.selectedPath != "/tmp/project-preferred" {
		t.Fatalf("selected path = %q, want previous matching selection", d.selectedPath)
	}
	d.SetDestinations(updated[1:], "")
	if d.selectedPath != "/tmp/project-old" {
		t.Fatalf("selected path = %q, want first remaining match", d.selectedPath)
	}
}
