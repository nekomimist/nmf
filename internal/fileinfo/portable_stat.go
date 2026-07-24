package fileinfo

import (
	"context"
	"os"
)

// StatPortable resolves the input path to a suitable provider and performs Stat.
// For SMB providers, it uses the provider-native path from the resolver.
func StatPortable(p string) (os.FileInfo, error) {
	return StatPortableContext(context.Background(), p)
}

// StatPortableContext resolves the input path and performs Stat while
// propagating cancellation to providers that support it.
func StatPortableContext(ctx context.Context, p string) (os.FileInfo, error) {
	if ctx == nil {
		ctx = context.Background()
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
	if statter, ok := vfs.(interface {
		StatContext(context.Context, string) (os.FileInfo, error)
	}); ok {
		return statter.StatContext(ctx, native)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := vfs.Stat(native)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return info, nil
}
