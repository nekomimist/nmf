package fileinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateRenameName validates a same-directory rename target name.
func ValidateRenameName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("name is empty")
	}
	if trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("name cannot be %q", trimmed)
	}
	if strings.ContainsRune(trimmed, 0) {
		return "", fmt.Errorf("name contains NUL")
	}
	if strings.ContainsAny(trimmed, `/\`) {
		return "", fmt.Errorf("name must not contain path separators")
	}
	return trimmed, nil
}

// RenamePortable renames oldPath to newName within the same parent directory.
// It returns the resulting display path. Existing targets are rejected.
func RenamePortable(oldPath, newName string) (string, error) {
	name, err := ValidateRenameName(newName)
	if err != nil {
		return "", err
	}

	oldDisplay, oldParsed, err := ResolvePathDisplay(oldPath)
	if err != nil {
		return "", err
	}
	if oldParsed.Scheme == SchemeArchive {
		return "", fmt.Errorf("archive paths are read-only: %s", oldDisplay)
	}
	oldVFS, _, err := ResolveRead(oldPath)
	if err != nil {
		return "", err
	}
	if BaseName(oldDisplay) == name {
		return oldDisplay, nil
	}

	newDisplay := JoinPath(ParentPath(oldDisplay), name)
	if _, err := StatPortable(newDisplay); err == nil {
		return "", fmt.Errorf("target already exists: %s", newDisplay)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	_, newParsed, err := ResolveRead(newDisplay)
	if err != nil {
		return "", err
	}

	oldNative := oldParsed.Native
	if oldNative == "" {
		oldNative = oldDisplay
	}
	newNative := newParsed.Native
	if newNative == "" {
		newNative = newDisplay
	}

	if oldParsed.Scheme == SchemeSMB && oldParsed.Provider != "local" {
		ops, ok := oldVFS.(SMBPathOps)
		if !ok {
			return "", fmt.Errorf("direct SMB provider is unavailable: %s", oldDisplay)
		}
		if err := ops.Rename(oldNative, newNative); err != nil {
			return "", err
		}
		return newDisplay, nil
	}

	if err := os.Rename(oldNative, newNative); err != nil {
		return "", err
	}
	if oldParsed.Scheme == SchemeFile && !filepath.IsAbs(newDisplay) {
		if abs, err := filepath.Abs(newDisplay); err == nil {
			return abs, nil
		}
	}
	return newDisplay, nil
}
