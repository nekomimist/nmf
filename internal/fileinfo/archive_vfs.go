package fileinfo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mholt/archives"
)

// ArchiveVFS exposes a read-only archive as a VFS.
type ArchiveVFS struct {
	archivePath string
	localPath   string
	tempPath    string
	fsys        fs.FS
}

var archiveTempMu sync.Mutex

// ErrUnsafeArchiveEntry reports an archive entry name that could escape or
// confuse a destination filesystem if materialized.
var ErrUnsafeArchiveEntry = errors.New("unsafe archive entry")

// NewArchiveVFS opens archivePath as a read-only virtual file system.
func NewArchiveVFS(archivePath string) (*ArchiveVFS, error) {
	return NewArchiveVFSContext(context.Background(), archivePath)
}

// NewArchiveVFSContext opens archivePath as a read-only virtual file system.
func NewArchiveVFSContext(ctx context.Context, archivePath string) (*ArchiveVFS, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if IsArchivePath(archivePath) {
		return nil, fmt.Errorf("nested archive paths are not supported: %s", archivePath)
	}

	localPath, tempPath, err := archiveLocalPath(archivePath)
	if err != nil {
		return nil, err
	}
	fsys, err := archiveFileSystem(ctx, localPath, currentArchiveOptions())
	if err != nil {
		if tempPath != "" {
			_ = os.Remove(tempPath)
		}
		return nil, err
	}
	return &ArchiveVFS{
		archivePath: archivePath,
		localPath:   localPath,
		tempPath:    tempPath,
		fsys:        fsys,
	}, nil
}

func archiveFileSystem(ctx context.Context, localPath string, opts ArchiveOptions) (fs.FS, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	format, _, err := archives.Identify(ctx, filepath.Base(localPath), file)
	if err != nil {
		return nil, fmt.Errorf("identify format: %w", err)
	}
	if _, ok := format.(archives.Zip); ok {
		format = archives.Zip{TextEncoding: archiveZipNameEncoding(opts)}
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return nil, fmt.Errorf("format is not extractable: %s", localPath)
	}
	if err := validateArchiveFileSystem(ctx, localPath, extractor); err != nil {
		return nil, err
	}
	if zipFormat, ok := format.(archives.Zip); ok {
		return &archives.ArchiveFS{
			Path:    localPath,
			Format:  zipFormat,
			Context: ctx,
		}, nil
	}
	return archives.FileSystem(ctx, localPath, nil)
}

func validateArchiveFileSystem(ctx context.Context, localPath string, extractor archives.Extractor) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = extractor.Extract(ctx, file, func(ctx context.Context, entry archives.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := ValidateArchiveEntryPath(entry.NameInArchive, entry.IsDir()); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("validate archive entries: %w", err)
	}
	return nil
}

// IsSupportedArchive reports whether p can be opened as an archive-like FS.
func IsSupportedArchive(p string) bool {
	if strings.TrimSpace(p) == "" || IsArchivePath(p) {
		return false
	}
	localPath, tempPath, err := archiveLocalPath(p)
	if err != nil {
		return false
	}
	if tempPath != "" {
		defer os.Remove(tempPath)
	}
	file, err := os.Open(localPath)
	if err != nil {
		return false
	}
	defer file.Close()
	format, _, err := archives.Identify(context.Background(), filepath.Base(localPath), file)
	if err != nil {
		return false
	}
	_, ok := format.(archives.Extractor)
	return ok
}

func (a *ArchiveVFS) ReadDir(p string) ([]os.DirEntry, error) {
	native, err := safeArchiveNativePath(p)
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(a.fsys, native)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if err := ValidateArchiveEntryBaseName(entry.Name()); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func (a *ArchiveVFS) Stat(p string) (os.FileInfo, error) {
	native, err := safeArchiveNativePath(p)
	if err != nil {
		return nil, err
	}
	return fs.Stat(a.fsys, native)
}

func (a *ArchiveVFS) Capabilities() Capabilities {
	return Capabilities{FastList: false, Watch: false}
}

func (a *ArchiveVFS) Join(elem ...string) string {
	return pathJoin(elem...)
}

func (a *ArchiveVFS) Base(p string) string {
	return pathBase(p)
}

func (a *ArchiveVFS) Open(p string) (io.ReadCloser, error) {
	native, err := safeArchiveNativePath(p)
	if err != nil {
		return nil, err
	}
	f, err := a.fsys.Open(native)
	if err != nil {
		return nil, err
	}
	if rc, ok := f.(io.ReadCloser); ok {
		return rc, nil
	}
	_ = f.Close()
	return nil, fmt.Errorf("archive entry is not readable: %s", p)
}

func (a *ArchiveVFS) Close() error {
	if a == nil || a.tempPath == "" {
		return nil
	}
	return os.Remove(a.tempPath)
}

func archiveLocalPath(displayPath string) (localPath, tempPath string, err error) {
	vfs, parsed, err := ResolveRead(displayPath)
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme == SchemeArchive {
		return "", "", fmt.Errorf("nested archive paths are not supported: %s", displayPath)
	}
	if parsed.Provider == "local" {
		native := parsed.Native
		if native == "" {
			native = displayPath
		}
		return native, "", nil
	}

	native := parsed.Native
	if native == "" {
		native = displayPath
	}
	in, err := vfs.Open(native)
	if err != nil {
		return "", "", err
	}
	defer in.Close()

	archiveTempMu.Lock()
	defer archiveTempMu.Unlock()
	tmp, err := os.CreateTemp("", "nmf-archive-source-*"+filepath.Ext(displayPath))
	if err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = io.Copy(tmp, in); err != nil {
		return "", "", err
	}
	if err = tmp.Close(); err != nil {
		return "", "", err
	}
	return tmp.Name(), tmp.Name(), nil
}

func ExtractArchiveEntryToTemp(displayPath string) (string, error) {
	archiveFile, inner, ok := SplitArchivePath(displayPath)
	if !ok || inner == "." {
		return "", fmt.Errorf("not an archive file entry: %s", displayPath)
	}
	vfs, err := NewArchiveVFS(archiveFile)
	if err != nil {
		return "", err
	}
	defer vfs.Close()

	info, err := vfs.Stat(inner)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("archive entry is a directory: %s", displayPath)
	}
	if err := ValidateArchiveEntryPath(inner, false); err != nil {
		return "", err
	}

	in, err := vfs.Open(inner)
	if err != nil {
		return "", err
	}
	defer in.Close()

	dir, err := os.MkdirTemp("", "nmf-archive-open-*")
	if err != nil {
		return "", err
	}
	outPath := filepath.Join(dir, filepath.Base(inner))
	perm := info.Mode().Perm()
	if perm == 0 {
		perm = 0600
	}
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err = out.Close(); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return outPath, nil
}

func CleanupOldArchiveOpenTemps() {
	cutoff := time.Now().Add(-24 * time.Hour)
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "nmf-archive-open-") {
			continue
		}
		p := filepath.Join(os.TempDir(), entry.Name())
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(p)
	}
}

func safeArchiveNativePath(p string) (string, error) {
	native := archiveNativePath(p)
	if native == "." {
		return native, nil
	}
	if err := ValidateArchiveEntryPath(native, false); err != nil {
		return "", err
	}
	return native, nil
}

// ValidateArchiveEntryPath rejects archive paths that could escape or be
// reinterpreted when copied to local or SMB filesystems.
func ValidateArchiveEntryPath(p string, isDir bool) error {
	raw := strings.TrimSpace(p)
	if raw == "" || strings.ContainsRune(raw, 0) {
		return unsafeArchiveEntryError(p)
	}
	if strings.ContainsAny(raw, `\:`) {
		return unsafeArchiveEntryError(p)
	}
	if path.IsAbs(raw) || filepath.IsAbs(raw) || hasWindowsVolumePrefix(raw) {
		return unsafeArchiveEntryError(p)
	}

	cleaned := path.Clean(raw)
	if cleaned == "." {
		if isDir {
			return nil
		}
		return unsafeArchiveEntryError(p)
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == "" || part == "." || part == ".." {
			return unsafeArchiveEntryError(p)
		}
	}
	return nil
}

// ValidateArchiveEntryBaseName rejects names returned by ReadDir that are not
// single path elements on every supported platform.
func ValidateArchiveEntryBaseName(name string) error {
	if err := ValidateArchiveEntryPath(name, false); err != nil {
		return err
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return unsafeArchiveEntryError(name)
	}
	return nil
}

func unsafeArchiveEntryError(p string) error {
	return fmt.Errorf("%w: %q", ErrUnsafeArchiveEntry, p)
}

func hasWindowsVolumePrefix(p string) bool {
	if len(p) >= 2 && p[1] == ':' {
		c := p[0]
		return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
	}
	if runtime.GOOS != "windows" {
		return false
	}
	return filepath.VolumeName(p) != ""
}

func pathJoin(elem ...string) string {
	parts := make([]string, 0, len(elem))
	for _, part := range elem {
		if part == "" {
			continue
		}
		parts = append(parts, strings.Trim(part, "/"))
	}
	if len(parts) == 0 {
		return "."
	}
	joined := strings.Join(parts, "/")
	if joined == "" {
		return "."
	}
	return cleanArchiveInnerPath(joined)
}

func pathBase(p string) string {
	p = cleanArchiveInnerPath(p)
	if p == "." {
		return "."
	}
	if base := path.Base(p); base != "." && base != "/" {
		return base
	}
	return p
}

func IsUnsupportedArchiveWritePath(p string) error {
	if IsArchivePath(p) {
		return errors.New("archive paths are read-only")
	}
	return nil
}
