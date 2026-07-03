package keymanager

import (
	"fmt"
	"strings"
	"testing"

	"nmf/internal/config"
)

// TestWarnUnknownKeyBindingTargetsWarnsOnlyForUnknownTargets is a regression
// test for a gap where targetKeyBindingEntries filters config.json entries by
// target before validation, so a mistyped target (e.g. "fileviewr") silently
// vanishes from every target's binding list without any diagnostic.
// WarnUnknownKeyBindingTargets should log exactly one warning per entry whose
// normalized target is not one of main/lineEdit/fileViewer, and stay silent
// for recognized targets (including aliases and the empty/default target).
func TestWarnUnknownKeyBindingTargetsWarnsOnlyForUnknownTargets(t *testing.T) {
	entries := []config.KeyBindingEntry{
		{Target: "", Key: "A", Command: "cmd.a"},
		{Target: "main", Key: "B", Command: "cmd.b"},
		{Target: "lineEdit", Key: "C", Command: "cmd.c"},
		{Target: "line-edit", Key: "D", Command: "cmd.d"},
		{Target: "fileViewer", Key: "E", Command: "cmd.e"},
		{Target: "fileviewr", Key: "F", Command: "cmd.f"},
		{Target: "bogus", Key: "G", Command: "cmd.g"},
	}

	var warnings []string
	WarnUnknownKeyBindingTargets(entries, func(format string, args ...interface{}) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	})

	if len(warnings) != 2 {
		t.Fatalf("warnings = %#v, want exactly 2 (for target=fileviewr and target=bogus)", warnings)
	}
	for _, target := range []string{"fileviewr", "bogus"} {
		found := false
		for _, w := range warnings {
			if strings.Contains(w, target) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("warnings = %#v, want a warning mentioning target=%q", warnings, target)
		}
	}
}

func TestWarnUnknownKeyBindingTargetsNilDebugPrintIsNoop(t *testing.T) {
	// Must not panic when no debugPrint is supplied.
	WarnUnknownKeyBindingTargets([]config.KeyBindingEntry{
		{Target: "bogus", Key: "G", Command: "cmd.g"},
	}, nil)
}
