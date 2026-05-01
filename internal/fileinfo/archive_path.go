package fileinfo

import (
	"path"
	"path/filepath"
	"strings"
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
	idx := strings.Index(p, archivePathSeparator)
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
