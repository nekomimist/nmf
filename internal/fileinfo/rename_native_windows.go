//go:build windows
// +build windows

package fileinfo

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func renameNativeSameDir(oldNative, newNative string, caseOnlyRename bool) error {
	if !caseOnlyRename {
		return moveFileNoReplace(oldNative, newNative)
	}

	err := moveFileNoReplace(oldNative, newNative)
	if err == nil && nativeBaseMatches(newNative) {
		return nil
	}

	sourceNative := existingRenameSource(oldNative, newNative)
	tempNative, err := unusedRenameTempPath(filepath.Dir(oldNative))
	if err != nil {
		return err
	}
	if err := moveFileNoReplace(sourceNative, tempNative); err != nil {
		return err
	}
	if err := moveFileNoReplace(tempNative, newNative); err != nil {
		if rollbackErr := moveFileNoReplace(tempNative, sourceNative); rollbackErr != nil {
			return fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

func moveFileNoReplace(oldNative, newNative string) error {
	oldPtr, err := windows.UTF16PtrFromString(oldNative)
	if err != nil {
		return err
	}
	newPtr, err := windows.UTF16PtrFromString(newNative)
	if err != nil {
		return err
	}
	if err := windows.MoveFile(oldPtr, newPtr); err != nil {
		return &os.LinkError{Op: "rename", Old: oldNative, New: newNative, Err: err}
	}
	return nil
}

func nativeBaseMatches(native string) bool {
	targetInfo, err := os.Stat(native)
	if err != nil {
		return false
	}
	parent := filepath.Dir(native)
	want := filepath.Base(native)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() != want {
			continue
		}
		candidate := filepath.Join(parent, entry.Name())
		candidateInfo, err := os.Stat(candidate)
		if err == nil && os.SameFile(targetInfo, candidateInfo) {
			return true
		}
	}
	return false
}

func existingRenameSource(oldNative, newNative string) string {
	if _, err := os.Stat(oldNative); err == nil {
		return oldNative
	}
	if _, err := os.Stat(newNative); err == nil {
		return newNative
	}
	return oldNative
}

func unusedRenameTempPath(parent string) (string, error) {
	for i := 0; i < 100; i++ {
		candidate := filepath.Join(parent, fmt.Sprintf(".nmf-rename-%d-%d.tmp", os.Getpid(), i))
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find temporary rename path in %s", parent)
}
