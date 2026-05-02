//go:build windows
// +build windows

package fileinfo

import "golang.org/x/sys/windows"

func localStorageInfo(p string) (StorageInfo, error) {
	pathPtr, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return StorageInfo{}, err
	}
	var freeAvailable, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeAvailable, &total, &totalFree); err != nil {
		return StorageInfo{}, err
	}
	used := uint64(0)
	if total >= freeAvailable {
		used = total - freeAvailable
	}
	return StorageInfo{Free: freeAvailable, Used: used, Total: total}, nil
}
