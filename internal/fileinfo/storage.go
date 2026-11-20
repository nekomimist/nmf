package fileinfo

import (
	"context"
	"fmt"
)

// StorageInfo contains capacity information for the storage backing a path.
type StorageInfo struct {
	Free  uint64
	Used  uint64
	Total uint64
}

// ErrStorageUnsupported indicates that a provider cannot report storage capacity.
var ErrStorageUnsupported = fmt.Errorf("storage information is not supported")

type storageProvider interface {
	StorageInfoContext(context.Context, string) (StorageInfo, error)
}

// StatStoragePortable resolves the display path and returns storage capacity for
// the backing file system when the provider can report it.
func StatStoragePortable(p string) (StorageInfo, error) {
	return StatStoragePortableContext(context.Background(), p)
}

// StatStoragePortableContext uses the directory load context for network queries.
func StatStoragePortableContext(ctx context.Context, p string) (StorageInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return StorageInfo{}, err
	}
	if IsArchivePath(p) {
		return StorageInfo{}, ErrStorageUnsupported
	}
	vfs, parsed, err := ResolveReadContext(ctx, p)
	if err != nil {
		return StorageInfo{}, err
	}
	defer CloseVFS(vfs)
	if parsed.Scheme == SchemeArchive {
		return StorageInfo{}, ErrStorageUnsupported
	}

	native := parsed.Native
	if native == "" {
		native = p
	}
	if provider, ok := vfs.(storageProvider); ok {
		return provider.StorageInfoContext(ctx, native)
	}
	return localStorageInfo(native)
}

func storageInfoFromBlocks(totalBlocks, freeBlocks, blockSize uint64) StorageInfo {
	total := totalBlocks * blockSize
	free := freeBlocks * blockSize
	used := uint64(0)
	if total >= free {
		used = total - free
	}
	return StorageInfo{Free: free, Used: used, Total: total}
}
