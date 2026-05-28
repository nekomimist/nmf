//go:build !windows

package fileinfo

// ResolveShortcutNavigationDir is a no-op on non-Windows platforms.
func ResolveShortcutNavigationDir(p string) (string, bool, error) {
	return "", false, nil
}
