//go:build windows
// +build windows

package fileinfo

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

const (
	driveUnknown   = 0
	driveNoRootDir = 1
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
	return pathClassForWindowsDriveType(windows.GetDriveType(ptr)), nil
}

func pathClassForWindowsDriveType(driveType uint32) PathClass {
	switch driveType {
	case driveUnknown, driveNoRootDir:
		return PathClass{Unavailable: true}
	case driveRemote:
		return PathClass{Network: true}
	case driveRemovable:
		return PathClass{Removable: true}
	default:
		return PathClass{}
	}
}
