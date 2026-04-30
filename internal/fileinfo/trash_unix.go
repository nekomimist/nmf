//go:build !windows

package fileinfo

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

func trashPath(ctx context.Context, displayPath string) error {
	_, parsed, err := ResolveRead(displayPath)
	if err != nil {
		return err
	}
	if parsed.Scheme == SchemeSMB && parsed.Provider != "local" {
		return fmt.Errorf("%w: direct SMB paths cannot be trashed", ErrTrashUnsupported)
	}

	native := parsed.Native
	if native == "" {
		native = displayPath
	}
	if _, err := exec.LookPath("gio"); err != nil {
		return fmt.Errorf("%w: gio command not found", ErrTrashUnsupported)
	}
	cmd := exec.CommandContext(ctx, "gio", "trash", "--", native)
	if output, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("gio trash failed for %s: %w: %s", displayPath, err, string(output))
	}
	return nil
}
