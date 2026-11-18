package fileinfo

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

const archivePathSeparator = "!/"

// An explicit scheme disambiguates content-detected archives from real paths
// with directories ending in "!". The escaped outer path contains no "/".
const explicitArchivePrefix = "nmf-archive:"

// IsArchivePath reports whether p is an nmf archive virtual path.
func IsArchivePath(p string) bool {
	_, _, ok := SplitArchivePath(p)
	return ok
}

// SplitArchivePath splits "archive.ext!/inner/path" into the archive display path
// and an archive-internal path. The root inside the archive is returned as ".".
func SplitArchivePath(p string) (archiveFile, inner string, ok bool) {
	archiveFile, offset := archiveBoundary(p)
	if offset < 0 {
		return "", "", false
	}
	return archiveFile, cleanArchiveInnerPath(p[offset:]), true
}

// archiveBoundary returns the outer path and the byte offset of the member.
// A negative offset denotes an ordinary filesystem path.
func archiveBoundary(p string) (string, int) {
	if encoded, explicit := strings.CutPrefix(p, explicitArchivePrefix); explicit {
		outer, _, found := strings.Cut(encoded, archivePathSeparator)
		if !found || outer == "" || strings.Contains(outer, "/") {
			return "", -1
		}
		decoded, err := url.PathUnescape(outer)
		if err != nil || decoded == "" {
			return "", -1
		}
		return decoded, len(explicitArchivePrefix) + len(outer) + len(archivePathSeparator)
	}
	idx := archivePathSeparatorIndex(p)
	if idx < 0 {
		return "", -1
	}
	return p[:idx], idx + len(archivePathSeparator)
}

// archivePathSeparatorIndex finds an archive virtual-path boundary while
// ignoring ordinary directories whose names end in "!". A boundary is valid
// only when the preceding filename identifies an archive or a known ZIP-based
// application format. Scanning all candidates also allows such directories to
// contain an actual archive virtual path.
func archivePathSeparatorIndex(p string) int {
	searchFrom := 0
	for searchFrom < len(p) {
		relative := strings.Index(p[searchFrom:], archivePathSeparator)
		if relative < 0 {
			return -1
		}
		idx := searchFrom + relative
		if isArchiveFileName(p[:idx]) {
			return idx
		}
		searchFrom = idx + len(archivePathSeparator)
	}
	return -1
}

func isArchiveFileName(p string) bool {
	name := path.Base(strings.ReplaceAll(p, "\\", "/"))
	if name == "" || name == "." || name == "/" {
		return false
	}
	// Identify recognizes these ZIP containers by their contents, but does
	// not match their names without a stream. Path parsing must also work for
	// offline history and remote paths, so it cannot inspect file contents.
	// Opening the archive still validates its actual format independently.
	switch strings.ToLower(path.Ext(name)) {
	case ".xlsx", ".xlsm", ".xlsb", ".xltx", ".xltm", ".xlam",
		".docx", ".docm", ".dotx", ".dotm",
		".pptx", ".pptm", ".potx", ".potm", ".ppsx", ".ppsm", ".ppam", ".sldx", ".sldm",
		".odt", ".ods", ".odp", ".odg", ".odf", ".odb", ".ott", ".ots", ".otp", ".otg",
		".jar", ".war", ".ear", ".apk", ".aab", ".epub", ".cbz":
		return true
	}
	// These four archive formats also occur inside compound names such as
	// .tar.gz. Keep the name-only matching semantics of archives.Identify
	// without running every registered format matcher for each list entry.
	name = strings.ToLower(name)
	return strings.Contains(name, ".zip") || strings.Contains(name, ".tar") ||
		strings.Contains(name, ".rar") || strings.Contains(name, ".7z")
}

// ArchiveRootPath returns the display path for the root of an archive.
func ArchiveRootPath(archiveFile string) string {
	return ArchiveDisplayPath(archiveFile, ".")
}

// ArchiveDisplayPath joins an archive display path and an internal path.
func ArchiveDisplayPath(archiveFile, inner string) string {
	if !isArchiveFileName(archiveFile) {
		archiveFile = explicitArchivePrefix + url.PathEscape(archiveFile)
	}
	inner = cleanArchiveInnerPath(inner)
	if inner == "." {
		return archiveFile + archivePathSeparator
	}
	return archiveFile + archivePathSeparator + inner
}

func cleanArchiveInnerPath(inner string) string {
	inner = strings.ReplaceAll(inner, "\\", "/")
	inner = strings.TrimPrefix(inner, "/")
	if inner == "" || inner == "." {
		return "."
	}
	cleaned := path.Clean(inner)
	if cleaned == "." || cleaned == "/" {
		return "."
	}
	return strings.TrimPrefix(cleaned, "/")
}

func archiveNativePath(inner string) string {
	inner = cleanArchiveInnerPath(inner)
	if inner == "." {
		return "."
	}
	return inner
}

func archiveParentPath(p string) string {
	archiveFile, inner, ok := SplitArchivePath(p)
	if !ok {
		return filepath.Dir(p)
	}
	if inner == "." {
		return ParentPath(archiveFile)
	}
	parent := path.Dir(inner)
	if parent == "." || parent == "/" {
		return ArchiveRootPath(archiveFile)
	}
	return ArchiveDisplayPath(archiveFile, parent)
}

func archiveBaseName(p string) string {
	archiveFile, inner, ok := SplitArchivePath(p)
	if !ok {
		return filepath.Base(p)
	}
	if inner == "." {
		return BaseName(archiveFile)
	}
	return path.Base(inner)
}
