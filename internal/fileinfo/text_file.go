package fileinfo

import (
	"fmt"
	"os"
	"path/filepath"
)

// CreateTextFilePortable creates a new text file inside parentPath.
// It returns the resulting display path. Existing targets are rejected.
func CreateTextFilePortable(parentPath, name, content string) (string, error) {
	fileName, err := ValidateRenameName(name)
	if err != nil {
		return "", err
	}
	if IsArchivePath(parentPath) {
		return "", fmt.Errorf("archive paths are read-only: %s", parentPath)
	}

	parentDisplay, parentParsed, err := ResolvePathDisplay(parentPath)
	if err != nil {
		return "", err
	}
	if parentParsed.Scheme == SchemeArchive {
		return "", fmt.Errorf("archive paths are read-only: %s", parentDisplay)
	}
	if parentParsed.Scheme == SchemeSMB && parentParsed.Provider != "local" {
		return "", fmt.Errorf("direct SMB paths do not support text file creation: %s", parentDisplay)
	}

	newDisplay := JoinPath(parentDisplay, fileName)
	if _, err := StatPortable(newDisplay); err == nil {
		return "", fmt.Errorf("target already exists: %s", newDisplay)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	vfs, newParsed, err := ResolveRead(newDisplay)
	if err != nil {
		return "", err
	}
	defer CloseVFS(vfs)
	if newParsed.Scheme == SchemeArchive {
		return "", fmt.Errorf("archive paths are read-only: %s", newDisplay)
	}
	if newParsed.Scheme == SchemeSMB && newParsed.Provider != "local" {
		return "", fmt.Errorf("direct SMB paths do not support text file creation: %s", newDisplay)
	}

	newNative := newParsed.Native
	if newNative == "" {
		newNative = newDisplay
	}
	out, err := os.OpenFile(newNative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	completed := false
	defer func() {
		if !completed {
			_ = out.Close()
			_ = os.Remove(newNative)
		}
	}()
	if _, err := out.WriteString(content); err != nil {
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	completed = true
	if newParsed.Scheme == SchemeFile && !filepath.IsAbs(newDisplay) {
		if abs, err := filepath.Abs(newDisplay); err == nil {
			return abs, nil
		}
	}
	return newDisplay, nil
}
