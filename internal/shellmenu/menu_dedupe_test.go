package shellmenu

import (
	"reflect"
	"testing"
)

func TestNormalizeMenuLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "mnemonic", in: "開く(&O)", want: "開く"},
		{name: "ampersand mnemonic", in: "&Open", want: "open"},
		{name: "escaped ampersand", in: "A && B", want: "a & b"},
		{name: "ellipsis", in: "Microsoft Defender でスキャンする...", want: "microsoft defender でスキャンする"},
		{name: "unicode ellipsis", in: "コピーを転送…", want: "コピーを転送"},
		{name: "plain accelerator", in: "プロパティ(R)", want: "プロパティ"},
		{name: "keeps non accelerator suffix", in: "OneDrive (Personal)", want: "onedrive (personal)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMenuLabel(tt.in); got != tt.want {
				t.Fatalf("normalizeMenuLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDuplicateMenuPositionsPrefersFirstEnabledEntry(t *testing.T) {
	entries := []menuDedupeEntry{
		{position: 0, label: "OneDrive に移動(M)", enabled: false},
		{position: 1, label: "OneDrive に移動(A)", enabled: true},
		{position: 2, label: "PowerRename で名前を変更", enabled: true},
		{position: 3, label: "PowerRename で名前を変更(E)", enabled: true},
		{position: 4, label: "7-Zip", enabled: true},
	}

	got := duplicateMenuPositions(entries)
	want := []int{3, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicateMenuPositions = %#v, want %#v", got, want)
	}
}

func TestDuplicateMenuPositionsUsesVerbAndLabel(t *testing.T) {
	entries := []menuDedupeEntry{
		{position: 0, label: "共有(S)", verb: "share", enabled: true},
		{position: 1, label: "Share", verb: "share", enabled: true},
		{position: 2, label: "OneDrive に移動(M)", verb: "onedrive.move", enabled: true},
		{position: 3, label: "OneDrive に移動(A)", verb: "cloud.keep", enabled: true},
	}

	got := duplicateMenuPositions(entries)
	want := []int{3, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("duplicateMenuPositions = %#v, want %#v", got, want)
	}
}

func TestSeparatorCleanupPositions(t *testing.T) {
	entries := []menuDedupeEntry{
		{position: 0, separator: true},
		{position: 1, label: "Open"},
		{position: 2, separator: true},
		{position: 3, separator: true},
		{position: 4, label: "Copy"},
		{position: 5, separator: true},
	}
	removed := map[int]struct{}{}

	got := separatorCleanupPositions(entries, removed)
	want := []int{5, 3, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("separatorCleanupPositions = %#v, want %#v", got, want)
	}
}
