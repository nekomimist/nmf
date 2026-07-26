package fileinfo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestIsNotExist(t *testing.T) {
	_, statErr := os.Stat(filepath.Join(t.TempDir(), "missing"))

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "PathError from Stat", err: statErr, want: true},
		{name: "bare fs.ErrNotExist", err: fs.ErrNotExist, want: true},
		{name: "wrapped with %w", err: fmt.Errorf("resolving target: %w", statErr), want: true},
		{name: "unrelated", err: fs.ErrPermission, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotExist(tt.err); got != tt.want {
				t.Fatalf("IsNotExist(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}
