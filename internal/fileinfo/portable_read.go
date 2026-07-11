package fileinfo

import (
	"context"
	"os"
	"runtime"
)

type readDirContextProvider interface {
	ReadDirContext(context.Context, string) ([]os.DirEntry, error)
}

// ReadDirPortable resolves the input path to a suitable provider and performs ReadDir.
func ReadDirPortable(p string) ([]os.DirEntry, error) {
	return ReadDirPortableContext(context.Background(), p)
}

// ReadDirPortableContext resolves the input path and performs ReadDir with
// best-effort cancellation for providers that support it.
func ReadDirPortableContext(ctx context.Context, p string) ([]os.DirEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vfs, parsed, err := ResolveReadContext(ctx, p)
	if err != nil {
		return nil, err
	}
	defer CloseVFS(vfs)
	native := parsed.Native
	if native == "" {
		native = p
	}
	entries, rerr := readDirWithContext(ctx, vfs, native)
	// Windows: if UNC access denied, try establishing a connection via keyring/UI then retry
	if rerr != nil && runtime.GOOS == "windows" && (isUNC(native) || parsed.Scheme == SchemeSMB) && isWinAccessError(rerr) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err2 := ensureWindowsConnection(parsed, native); err2 == nil {
			return readDirWithContext(ctx, vfs, native)
		} else if IsWindowsCredentialConflict(err2) {
			// propagate credential conflict so caller can present guidance
			return nil, err2
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return entries, rerr
}

func readDirWithContext(ctx context.Context, vfs VFS, native string) ([]os.DirEntry, error) {
	if provider, ok := vfs.(readDirContextProvider); ok {
		return provider.ReadDirContext(ctx, native)
	}
	return vfs.ReadDir(native)
}
