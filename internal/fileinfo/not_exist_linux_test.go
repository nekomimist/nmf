//go:build linux

package fileinfo

import (
	"fmt"
	"os"
	"testing"

	"github.com/hirochachacha/go-smb2"
)

func TestIsNotExistRecognizesSMBResponseErrors(t *testing.T) {
	tests := []struct {
		name string
		code uint32
		want bool
	}{
		{name: "no such file", code: smbStatusNoSuchFile, want: true},
		{name: "object name missing", code: smbStatusObjectNameMissing, want: true},
		{name: "object path missing", code: smbStatusObjectPathMissing, want: true},
		{name: "access denied", code: 0xC0000022, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &os.PathError{Op: "stat", Path: "missing", Err: &smb2.ResponseError{Code: tt.code}}
			if got := IsNotExist(fmt.Errorf("portable stat: %w", err)); got != tt.want {
				t.Fatalf("IsNotExist() = %t, want %t", got, tt.want)
			}
		})
	}
}
