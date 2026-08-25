//go:build !windows

package fileinfo

import "context"

// IsShortcutNavigationCandidate is always false on non-Windows platforms.
func IsShortcutNavigationCandidate(string) bool {
	return false
}

// ResolveShortcutNavigationDirContext is a no-op on non-Windows platforms.
func ResolveShortcutNavigationDirContext(context.Context, string) (string, bool, error) {
	return "", false, nil
}
