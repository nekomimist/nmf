//go:build windows

package fileinfo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nziu/lnk"
)

// IsShortcutNavigationCandidate reports whether p should be resolved as a
// Windows shortcut before default-app delegation.
func IsShortcutNavigationCandidate(p string) bool {
	return strings.EqualFold(filepath.Ext(p), ".lnk")
}

// ResolveShortcutNavigationDirContext resolves a Windows shortcut while
// propagating logical cancellation around COM and filesystem calls. Windows
// local/UNC filesystem calls themselves may remain blocked until the OS
// returns, so callers must also discard results from canceled operations.
func ResolveShortcutNavigationDirContext(ctx context.Context, p string) (string, bool, error) {
	if !IsShortcutNavigationCandidate(p) {
		return "", false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}

	shortcut, err := lnk.Read(NormalizeInputPath(p))
	if err != nil {
		return "", false, &ShortcutNavigationError{
			Stage: ShortcutNavigationRead,
			Path:  p,
			Err:   err,
		}
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}

	target := strings.TrimSpace(shortcut.TargetPath)
	if target == "" {
		return "", false, nil
	}

	info, err := StatPortableContext(ctx, target)
	if err != nil {
		return "", false, &ShortcutNavigationError{
			Stage:  ShortcutNavigationTarget,
			Path:   p,
			Target: target,
			Err:    fmt.Errorf("stat: %w", err),
		}
	}

	dir := target
	if !info.IsDir() {
		dir = filepath.Dir(target)
	}

	resolved, _, err := ResolveDirectoryPathContext(ctx, dir)
	if err != nil {
		return "", false, &ShortcutNavigationError{
			Stage:  ShortcutNavigationTarget,
			Path:   p,
			Target: target,
			Err:    err,
		}
	}
	return resolved, true, nil
}
