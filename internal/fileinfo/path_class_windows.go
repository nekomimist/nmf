//go:build windows
// +build windows

package fileinfo

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

const (
	driveRemovable = 2
	driveRemote    = 4
)

func classifyLocalPath(p string) (PathClass, error) {
	root := filepath.VolumeName(p)
	if root == "" {
		return PathClass{}, nil
	}
	root += `\`
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return PathClass{}, err
	}
	switch windows.GetDriveType(ptr) {
	case driveRemote:
		return PathClass{Network: true}, nil
	case driveRemovable:
		return PathClass{Removable: true}, nil
	default:
		return PathClass{}, nil
	}
}
