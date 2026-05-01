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
