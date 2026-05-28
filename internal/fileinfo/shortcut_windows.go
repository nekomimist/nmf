//go:build windows

package fileinfo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nziu/lnk"
)

// ResolveShortcutNavigationDir resolves a Windows shortcut to the directory
// nmf should navigate to when the regular open command is used.
func ResolveShortcutNavigationDir(p string) (string, bool, error) {
	if !strings.EqualFold(filepath.Ext(p), ".lnk") {
		return "", false, nil
	}

	shortcut, err := lnk.Read(NormalizeInputPath(p))
	if err != nil {
		return "", false, err
	}

	target := strings.TrimSpace(shortcut.TargetPath)
	if target == "" {
		return "", false, nil
	}

	info, err := StatPortable(target)
	if err != nil {
		return "", false, fmt.Errorf("stat shortcut target %q: %w", target, err)
	}

	dir := target
	if !info.IsDir() {
		dir = filepath.Dir(target)
	}

	resolved, _, err := ResolveDirectoryPath(dir)
	if err != nil {
		return "", false, err
	}
	return resolved, true, nil
}
