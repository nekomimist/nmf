package ui

import (
	"testing"

	"nmf/internal/config"
)

func TestNormalizeDirectoryJumpShortcut(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "lower", in: "p", want: "p"},
		{name: "upper", in: "P", want: "p"},
		{name: "trim", in: " P ", want: "p"},
		{name: "multiple", in: "Proj", want: "proj"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeDirectoryJumpShortcut(tt.in); got != tt.want {
				t.Fatalf("NormalizeDirectoryJumpShortcut(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSortDirectoryJumpEntriesOrdersShortcutsBeforeEmpty(t *testing.T) {
	entries := []config.DirectoryJumpEntry{
		{Shortcut: "", Directory: "/tmp"},
		{Shortcut: "proj", Directory: "/projects"},
		{Shortcut: "d", Directory: "/downloads"},
		{Shortcut: "pa", Directory: "/archive"},
		{Shortcut: "", Directory: "/var/tmp"},
		{Shortcut: "b", Directory: "/backup"},
	}

	sorted := sortDirectoryJumpEntries(copyDirectoryJumpEntries(entries))
	got := make([]string, len(sorted))
	for i, entry := range sorted {
		got[i] = entry.Shortcut + ":" + entry.Directory
	}
	want := []string{
		"b:/backup",
		"d:/downloads",
		"pa:/archive",
		"proj:/projects",
		":/tmp",
		":/var/tmp",
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted entries = %v, want %v", got, want)
		}
	}
}

func TestFilterDirectoryJumpEntriesMatchesShortcutPrefixOnly(t *testing.T) {
	entries := []config.DirectoryJumpEntry{
		{Shortcut: "proj", Directory: "/work/projects"},
		{Shortcut: "pa", Directory: "/work/archive"},
		{Shortcut: "", Directory: "/plain"},
		{Shortcut: "x", Directory: "/tmp"},
	}

	filtered := filterDirectoryJumpEntries(entries, "p")
	if len(filtered) != 2 || filtered[0].Directory != "/work/projects" || filtered[1].Directory != "/work/archive" {
		t.Fatalf("shortcut prefix query result = %+v, want projects and archive", filtered)
	}

	filtered = filterDirectoryJumpEntries(entries, "work")
	if len(filtered) != 0 {
		t.Fatalf("directory-only query result = %+v, want empty", filtered)
	}

	filtered = filterDirectoryJumpEntries(entries, "pr")
	if len(filtered) != 1 || filtered[0].Directory != "/work/projects" {
		t.Fatalf("unique shortcut query result = %+v, want only /work/projects", filtered)
	}

	filtered = filterDirectoryJumpEntries(entries, "")
	if len(filtered) != len(entries) {
		t.Fatalf("empty query result length = %d, want %d", len(filtered), len(entries))
	}
}
