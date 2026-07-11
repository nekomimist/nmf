package fileinfo

import (
	"os"
	"runtime"
	"strings"
	"time"
)

type lstatProvider interface {
	Lstat(path string) (os.FileInfo, error)
}

type readlinkProvider interface {
	Readlink(path string) (string, error)
}

// PathMetadata is the normalized file metadata nmf uses for list rendering and navigation.
type PathMetadata struct {
	Info     os.FileInfo
	IsDir    bool
	IsLink   bool
	FileType FileType
	Size     int64
	Modified time.Time
}

// LstatPortable resolves the path to its backend and stats the path without following links.
func LstatPortable(p string) (os.FileInfo, error) {
	vfs, parsed, err := ResolveRead(p)
	if err != nil {
		return nil, err
	}
	defer CloseVFS(vfs)
	native := parsed.Native
	if native == "" {
		native = p
	}
	if parsed.Provider == "local" || parsed.Scheme == SchemeFile {
		return os.Lstat(native)
	}
	if provider, ok := vfs.(lstatProvider); ok {
		return provider.Lstat(native)
	}
	return vfs.Stat(native)
}

// ReadlinkPortable resolves the path to its backend and reads the link target.
func ReadlinkPortable(p string) (string, error) {
	vfs, parsed, err := ResolveRead(p)
	if err != nil {
		return "", err
	}
	defer CloseVFS(vfs)
	native := parsed.Native
	if native == "" {
		native = p
	}
	if parsed.Provider == "local" || parsed.Scheme == SchemeFile {
		return os.Readlink(native)
	}
	if provider, ok := vfs.(readlinkProvider); ok {
		return provider.Readlink(native)
	}
	return "", os.ErrInvalid
}

// IsLinkModeCandidate reports whether a mode may describe a link-like object.
// Windows directory junctions are reparse points that Go exposes as irregular
// files on current releases, so callers must confirm them with Readlink.
func IsLinkModeCandidate(mode os.FileMode) bool {
	if mode&os.ModeSymlink != 0 {
		return true
	}
	return runtime.GOOS == "windows" && mode&os.ModeIrregular != 0
}

// InspectPath returns nmf's normalized view of a filesystem path.
// IsDir means "can be navigated into", so directory links are true here while
// still retaining FileTypeSymlink for rendering and copy/move/delete policy.
func InspectPath(path string, name string, entry os.DirEntry) (PathMetadata, error) {
	info, err := pathInfo(path, entry)
	if err != nil {
		return PathMetadata{}, err
	}

	isLink := info.Mode()&os.ModeSymlink != 0
	if !isLink && runtime.GOOS == "windows" && info.Mode()&os.ModeIrregular != 0 {
		if _, err := ReadlinkPortable(path); err == nil {
			isLink = true
		}
	}

	isDir := info.IsDir()
	if isLink {
		isDir = false
		if targetInfo, err := StatPortable(path); err == nil && targetInfo.IsDir() {
			isDir = true
		}
	}

	return PathMetadata{
		Info:     info,
		IsDir:    isDir,
		IsLink:   isLink,
		FileType: determineFileType(path, name, isDir, isLink),
		Size:     info.Size(),
		Modified: info.ModTime(),
	}, nil
}

// FileInfoFromDirEntry builds a FileInfo from a directory entry using nmf link semantics.
func FileInfoFromDirEntry(parent string, entry os.DirEntry) (FileInfo, error) {
	fullPath := JoinPath(parent, entry.Name())
	metadata, err := InspectPath(fullPath, entry.Name(), entry)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name:     entry.Name(),
		Path:     fullPath,
		IsDir:    metadata.IsDir,
		Size:     metadata.Size,
		Modified: metadata.Modified,
		FileType: metadata.FileType,
		Status:   StatusNormal,
	}, nil
}

// IsNavigableDirectory reports whether p can be opened as a directory in nmf.
func IsNavigableDirectory(p string) bool {
	metadata, err := InspectPath(p, BaseName(p), nil)
	return err == nil && metadata.IsDir
}

func pathInfo(path string, entry os.DirEntry) (os.FileInfo, error) {
	if entry != nil {
		if info, err := entry.Info(); err == nil {
			return info, nil
		}
	}
	return LstatPortable(path)
}

func determineFileType(path string, name string, isDir bool, isLink bool) FileType {
	if isLink {
		return FileTypeSymlink
	}
	if isDir {
		return FileTypeDirectory
	}
	if strings.HasPrefix(name, ".") {
		return FileTypeHidden
	}
	if runtime.GOOS == "windows" && IsWindowsHidden(path) {
		return FileTypeHidden
	}
	return FileTypeRegular
}
