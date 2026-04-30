package fileinfo

import (
	"context"
	"errors"
)

var (
	// ErrTrashUnsupported is returned when the current backend cannot move a path to trash.
	ErrTrashUnsupported = errors.New("trash is unsupported for this path")
)

// TrashPath moves displayPath to the platform trash/recycle bin.
func TrashPath(ctx context.Context, displayPath string) error {
	return trashPath(ctx, displayPath)
}
