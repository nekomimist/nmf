//go:build windows

package fileinfo

import "testing"

func TestPathClassForWindowsDriveType(t *testing.T) {
	tests := []struct {
		name      string
		driveType uint32
		want      PathClass
	}{
		{name: "unknown", driveType: driveUnknown, want: PathClass{Unavailable: true}},
		{name: "no root", driveType: driveNoRootDir, want: PathClass{Unavailable: true}},
		{name: "removable", driveType: driveRemovable, want: PathClass{Removable: true}},
		{name: "remote", driveType: driveRemote, want: PathClass{Network: true}},
		{name: "fixed", driveType: 3, want: PathClass{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathClassForWindowsDriveType(tt.driveType); got != tt.want {
				t.Fatalf("pathClassForWindowsDriveType(%d) = %#v, want %#v", tt.driveType, got, tt.want)
			}
		})
	}
}
