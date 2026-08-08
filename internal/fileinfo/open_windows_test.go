//go:build windows

package fileinfo

import (
	"testing"
)

func TestOpenWorkingDirectory(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "local unicode path",
			input: `D:\マーブルCandy\IHS.exe`,
			want:  `D:\マーブルCandy`,
		},
		{
			name:  "path with spaces",
			input: `C:\Program Files\Example\app.exe`,
			want:  `C:\Program Files\Example`,
		},
		{
			name:  "UNC path",
			input: `\\server\share\apps\app.exe`,
			want:  `\\server\share\apps`,
		},
		{
			name:  "SMB display path",
			input: "smb://server/share/apps/app.exe",
			want:  `\\server\share\apps`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			native := NormalizeInputPath(tt.input)
			got := nativeOpenWorkingDirectory(native)
			if got != tt.want {
				t.Fatalf("nativeOpenWorkingDirectory(NormalizeInputPath(%q)) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
