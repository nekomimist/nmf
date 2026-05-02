//go:build !windows
// +build !windows

package fileinfo

import "golang.org/x/sys/unix"

func localStorageInfo(p string) (StorageInfo, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(p, &st); err != nil {
		return StorageInfo{}, err
	}
	blockSize := uint64(st.Bsize)
	return storageInfoFromBlocks(st.Blocks, st.Bavail, blockSize), nil
}
