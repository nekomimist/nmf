package fileinfo

import (
	"fmt"
	"os"
	"path/filepath"
)

// CreateDirectoryPortable creates a single directory inside parentPath.
// It returns the resulting display path. Existing targets are rejected.
func CreateDirectoryPortable(parentPath, name string) (string, error) {
	dirName, err := ValidateRenameName(name)
	if err != nil {
		return "", err
	}

	parentDisplay, parentParsed, err := ResolvePathDisplay(parentPath)
	if err != nil {
		return "", err
	}
	if parentParsed.Scheme == SchemeArchive {
		return "", fmt.Errorf("archive paths are read-only: %s", parentDisplay)
	}

	newDisplay := JoinPath(parentDisplay, dirName)
	if _, err := StatPortable(newDisplay); err == nil {
		return "", fmt.Errorf("target already exists: %s", newDisplay)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	vfs, newParsed, err := ResolveRead(newDisplay)
	if err != nil {
		return "", err
	}
	newNative := newParsed.Native
	if newNative == "" {
		newNative = newDisplay
	}

	if newParsed.Scheme == SchemeSMB && newParsed.Provider != "local" {
		ops, ok := vfs.(SMBPathOps)
		if !ok {
			return "", fmt.Errorf("direct SMB provider is unavailable: %s", newDisplay)
		}
		if err := ops.MkdirAll(newNative, 0755); err != nil {
			return "", err
		}
		return newDisplay, nil
	}

	if err := os.Mkdir(newNative, 0755); err != nil {
		return "", err
	}
	if newParsed.Scheme == SchemeFile && !filepath.IsAbs(newDisplay) {
		if abs, err := filepath.Abs(newDisplay); err == nil {
			return abs, nil
		}
	}
	return newDisplay, nil
}
