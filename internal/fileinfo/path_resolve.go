package fileinfo

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// CanonicalDisplayPath normalizes a path string for UI display and durable
// runtime state without requiring the target to be accessible.
func CanonicalDisplayPath(input string) (string, Parsed, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", Parsed{}, fmt.Errorf("path is empty")
	}

	if display, parsed, ok, err := canonicalArchiveDisplayPath(input, trimmed); ok {
		return display, parsed, err
	}

	if isUNC(trimmed) {
		parsed := parseUNC(trimmed)
		parsed.Raw = input
		return parsed.Display, parsed, nil
	}

	if isSMBURL(trimmed) || strings.HasPrefix(trimmed, "//") {
		host, share, segments, user, pass, domain := parseSMBURL(trimmed)
		if host == "" || share == "" {
			return "", Parsed{Raw: input, Scheme: SchemeSMB, Display: canonicalizeSMB(trimmed), Provider: "smb"}, fmt.Errorf("invalid smb path: %s", input)
		}
		display := smbDisplayPath(host, share, segments)
		native := "/"
		if len(segments) > 0 {
			native = "/" + path.Join(segments...)
		}
		return display, Parsed{
			Scheme:   SchemeSMB,
			Host:     host,
			Share:    share,
			Segments: segments,
			Raw:      input,
			Display:  display,
			Native:   native,
			Provider: "smb",
			User:     user,
			Password: pass,
			Domain:   domain,
		}, nil
	}

	return ResolvePathDisplay(trimmed)
}

// ResolvePathDisplay resolves user input to a canonical display/navigation path.
// SMB inputs are returned as canonical smb:// paths. Local inputs are returned as absolute paths.
func ResolvePathDisplay(input string) (string, Parsed, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", Parsed{}, fmt.Errorf("path is empty")
	}

	if display, parsed, ok, err := canonicalArchiveDisplayPath(input, trimmed); ok {
		return display, parsed, err
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

func canonicalArchiveDisplayPath(input, trimmed string) (string, Parsed, bool, error) {
	archiveFile, inner, ok := SplitArchivePath(trimmed)
	if !ok {
		return "", Parsed{}, false, nil
	}
	archiveDisplay, _, err := CanonicalDisplayPath(archiveFile)
	if err != nil {
		return "", Parsed{Raw: input, Scheme: SchemeArchive, Display: trimmed, Provider: "archive", Archive: archiveFile, Inner: inner}, true, err
	}
	display := ArchiveDisplayPath(archiveDisplay, inner)
	return display, Parsed{
		Scheme:   SchemeArchive,
		Raw:      input,
		Display:  display,
		Native:   archiveNativePath(inner),
		Provider: "archive",
		Archive:  archiveDisplay,
		Inner:    inner,
	}, true, nil
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
