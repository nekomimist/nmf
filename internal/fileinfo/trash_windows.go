//go:build windows

package fileinfo

import (
	"context"
	"fmt"

	"nmf/internal/shellmenu"
)

func trashPath(ctx context.Context, displayPath string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if IsArchivePath(displayPath) {
		return ErrTrashUnsupported
	}
	if err := shellmenu.Trash(displayPath); err != nil {
		return fmt.Errorf("IFileOperation trash failed for %s: %w", displayPath, err)
	}
	return nil
}
