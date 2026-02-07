package fileinfo

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolvePathDisplay resolves user input to a canonical display/navigation path.
// SMB inputs are returned as canonical smb:// paths. Local inputs are returned as absolute paths.
func ResolvePathDisplay(input string) (string, Parsed, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", Parsed{}, fmt.Errorf("path is empty")
	}

	normalized := NormalizeInputPath(trimmed)
	_, parsed, err := ResolveRead(normalized)
	if err != nil {
		return "", parsed, err
	}

	if parsed.Scheme == SchemeSMB {
		if parsed.Display != "" {
			return parsed.Display, parsed, nil
		}
		return normalized, parsed, nil
	}

	resolved := parsed.Native
	if resolved == "" {
		resolved = normalized
	}
	if !filepath.IsAbs(resolved) {
		absPath, err := filepath.Abs(resolved)
		if err != nil {
			return "", parsed, err
		}
		resolved = absPath
	}
	return resolved, parsed, nil
}

// ResolveDirectoryPath resolves input to a canonical path for navigation.
// Local paths are validated as directories; SMB paths are returned in canonical display form.
func ResolveDirectoryPath(input string) (string, Parsed, error) {
	resolved, parsed, err := ResolvePathDisplay(input)
	if err != nil {
		return "", parsed, err
	}
	if parsed.Scheme == SchemeSMB {
		return resolved, parsed, nil
	}
	info, err := StatPortable(resolved)
	if err != nil {
		return "", parsed, err
	}
	if !info.IsDir() {
		return "", parsed, fmt.Errorf("path is not a directory: %s", resolved)
	}
	return resolved, parsed, nil
}

// ResolveAccessibleDirectoryPath resolves input to a canonical path and validates accessibility.
// Both local and SMB paths must be stat'able directories.
func ResolveAccessibleDirectoryPath(input string) (string, Parsed, error) {
	resolved, parsed, err := ResolvePathDisplay(input)
	if err != nil {
		return "", parsed, err
	}
	info, err := StatPortable(resolved)
	if err != nil {
		return "", parsed, err
	}
	if !info.IsDir() {
		return "", parsed, fmt.Errorf("path is not a directory: %s", resolved)
	}
	return resolved, parsed, nil
}
