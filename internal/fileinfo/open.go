package fileinfo

import (
	"context"
	"os"
	"path/filepath"
)

// OpenWithDefaultApp opens the given path with the OS-associated application.
// Archive entries are extracted to a temporary file first.
func OpenWithDefaultApp(p string) error {
	return OpenWithDefaultAppContext(context.Background(), p)
}

// OpenWithDefaultAppContext cancels extraction before handing a file to the OS.
func OpenWithDefaultAppContext(ctx context.Context, p string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if IsArchivePath(p) {
		tmpPath, err := extractArchiveEntryToTemp(ctx, p)
		if err != nil {
			return err
		}
		if err = ctx.Err(); err == nil {
			err = openNativeWithDefaultApp(tmpPath)
		}
		if err != nil {
			_ = os.RemoveAll(filepath.Dir(tmpPath))
		}
		return err
	}
	return openNativeWithDefaultApp(p)
}
