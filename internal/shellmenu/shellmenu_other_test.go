//go:build !windows

package shellmenu

import (
	"errors"
	"testing"
)

func TestShowReturnsUnsupportedOffWindows(t *testing.T) {
	err := Show(0, []string{"/tmp/example"})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Show error = %v, want ErrUnsupported", err)
	}
}

func TestShowAtClientPositionReturnsUnsupportedOffWindows(t *testing.T) {
	err := ShowAtClientPosition(0, []string{"/tmp/example"}, 10, 20)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("ShowAtClientPosition error = %v, want ErrUnsupported", err)
	}
}
