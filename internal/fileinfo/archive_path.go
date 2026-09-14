package fileinfo

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
)

const archivePathSeparator = "!/"

// IsArchivePath reports whether p is an nmf archive virtual path.
func IsArchivePath(p string) bool {
	_, _, ok := SplitArchivePath(p)
	return ok
}

// SplitArchivePath splits "archive.ext!/inner/path" into the archive display path
// and an archive-internal path. The root inside the archive is returned as ".".
func SplitArchivePath(p string) (archiveFile, inner string, ok bool) {
	p = strings.TrimSpace(p)
	idx := archivePathSeparatorIndex(p)
	if idx < 0 {
		return "", "", false
	}
	archiveFile = p[:idx]
	inner = p[idx+len(archivePathSeparator):]
	if archiveFile == "" {
		return "", "", false
	}
	inner = cleanArchiveInnerPath(inner)
	return archiveFile, inner, true
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
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"))
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
	format, _, err := archives.Identify(context.Background(), name, nil)
	if err != nil {
		return false
	}
	_, ok := format.(archives.Extractor)
	return ok
}

// ArchiveRootPath returns the display path for the root of an archive.
func ArchiveRootPath(archiveFile string) string {
	return ArchiveDisplayPath(archiveFile, ".")
}

// ArchiveDisplayPath joins an archive display path and an internal path.
func ArchiveDisplayPath(archiveFile, inner string) string {
	inner = cleanArchiveInnerPath(inner)
	if inner == "." {
		return archiveFile + archivePathSeparator
	}
	return archiveFile + archivePathSeparator + inner
}

func cleanArchiveInnerPath(inner string) string {
	inner = strings.ReplaceAll(strings.TrimSpace(inner), "\\", "/")
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

func archiveJoinPath(base, name string) string {
	archiveFile, inner, ok := SplitArchivePath(base)
	if !ok {
		return filepath.Join(base, name)
	}
	if inner == "." {
		return ArchiveDisplayPath(archiveFile, name)
	}
	return ArchiveDisplayPath(archiveFile, path.Join(inner, name))
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
