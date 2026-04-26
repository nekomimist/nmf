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
		{name: "multiple", in: "pp", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeDirectoryJumpShortcut(tt.in); got != tt.want {
				t.Fatalf("NormalizeDirectoryJumpShortcut(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildDirectoryJumpShortcutTargetsKeepsFirstNonEmptyShortcut(t *testing.T) {
	entries := []config.DirectoryJumpEntry{
		{Shortcut: "p", Directory: "/projects"},
		{Shortcut: "", Directory: "/tmp"},
		{Shortcut: "", Directory: "/var/tmp"},
		{Shortcut: "P", Directory: "/duplicate"},
		{Shortcut: "d", Directory: "/downloads"},
	}

	targets := buildDirectoryJumpShortcutTargets(entries)

	if got := targets["p"]; got != "/projects" {
		t.Fatalf("shortcut p target = %q, want /projects", got)
	}
	if got := targets["d"]; got != "/downloads" {
		t.Fatalf("shortcut d target = %q, want /downloads", got)
	}
	if _, ok := targets[""]; ok {
		t.Fatal("empty shortcut should not be registered")
	}
}

func TestFilterDirectoryJumpEntriesMatchesDirectoryOnly(t *testing.T) {
	entries := []config.DirectoryJumpEntry{
		{Shortcut: "p", Directory: "/work/projects"},
		{Shortcut: "x", Directory: "/tmp"},
	}

	filtered := filterDirectoryJumpEntries(entries, "proj")
	if len(filtered) != 1 || filtered[0].Directory != "/work/projects" {
		t.Fatalf("directory query result = %+v, want only /work/projects", filtered)
	}

	filtered = filterDirectoryJumpEntries(entries, "x")
	if len(filtered) != 0 {
		t.Fatalf("shortcut-only query result = %+v, want empty", filtered)
	}
}
