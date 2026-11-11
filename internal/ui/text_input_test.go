package ui

import "testing"

func TestTrimLastRuneHandlesInvalidUTF8Tail(t *testing.T) {
	got := trimLastRune("日本語\xff")
	if got != "日本語" {
		t.Fatalf("trimLastRune got %q, want invalid tail byte removed", got)
	}
}
