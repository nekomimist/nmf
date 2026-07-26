package shellmenu

import (
	"strings"
	"testing"
)

func TestExceedsLegacyShellPathLimit(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "259 UTF-16 characters", path: strings.Repeat("a", 259), want: false},
		{name: "260 UTF-16 characters", path: strings.Repeat("a", 260), want: true},
		{name: "supplementary rune counts as two UTF-16 characters", path: strings.Repeat("a", 258) + "😀", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exceedsLegacyShellPathLimit(tt.path); got != tt.want {
				t.Fatalf("exceedsLegacyShellPathLimit length=%d = %t, want %t", len([]rune(tt.path)), got, tt.want)
			}
		})
	}
}
