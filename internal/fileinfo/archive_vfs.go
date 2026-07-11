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

	"github.com/bodgit/sevenzip"
	"github.com/mholt/archives"
	"github.com/nwaples/rardecode/v2"
)

// ArchiveVFS exposes a read-only archive as a VFS.
type ArchiveVFS struct {
	archivePath string
	localPath   string
	tempPath    string
	fsys        fs.FS
	mu          sync.Mutex
}

// ArchiveEntry describes one entry streamed from an archive.
type ArchiveEntry struct {
	Name       string
	Info       os.FileInfo
	LinkTarget string
	Open       func() (io.ReadCloser, error)
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
	fsys, err := archiveFileSystem(ctx, archivePath, localPath, currentArchiveOptions())
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

func archiveFileSystem(ctx context.Context, archivePath, localPath string, opts ArchiveOptions) (fs.FS, error) {
	var fsys fs.FS
	err := withArchivePasswordRetry(ctx, archivePath, localPath, opts, true, func(format archives.Format) error {
		extractor, ok := format.(archives.Extractor)
		if !ok {
			return fmt.Errorf("format is not extractable: %s", localPath)
		}
		if err := validateArchiveFileSystem(ctx, localPath, extractor, false); err != nil {
			return err
		}
		fsys = &archives.ArchiveFS{
			Path:    localPath,
			Format:  extractor,
			Context: ctx,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fsys, nil
}

func identifyArchiveFormat(ctx context.Context, localPath string) (archives.Format, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	format, _, err := archives.Identify(ctx, filepath.Base(localPath), file)
	if err != nil {
		return nil, fmt.Errorf("identify format: %w", err)
	}
	return format, nil
}

func withArchivePasswordRetry(ctx context.Context, archivePath, localPath string, opts ArchiveOptions, probeReadable bool, op func(archives.Format) error) error {
	format, err := identifyArchiveFormat(ctx, localPath)
	if err != nil {
		return err
	}
	if !archiveFormatSupportsPassword(format) {
		return op(applyArchiveFormatOptions(format, opts, ""))
	}

	var lastErr error
	password, cached := cachedArchivePassword(archivePath)
	retry := false
	for attempt := 0; attempt < 4; attempt++ {
		configured := applyArchiveFormatOptions(format, opts, password)
		if probeReadable {
			if extractor, ok := configured.(archives.Extractor); ok {
				if err := validateArchiveFileSystem(ctx, localPath, extractor, false); err != nil {
					lastErr = err
					if !isArchivePasswordError(err) {
						return err
					}
				} else if err := probeArchiveReadable(ctx, localPath, extractor); err != nil {
					lastErr = err
					if !isArchivePasswordError(err) {
						return err
					}
				} else if err := op(configured); err != nil {
					lastErr = err
					if !isArchivePasswordError(err) {
						return err
					}
				} else {
					if password != "" {
						putCachedArchivePassword(archivePath, password)
					}
					return nil
				}
			} else if err := op(configured); err != nil {
				return err
			} else {
				return nil
			}
		} else if err := op(configured); err != nil {
			lastErr = err
			if !isArchivePasswordError(err) {
				return err
			}
		} else {
			if password != "" {
				putCachedArchivePassword(archivePath, password)
			}
			return nil
		}

		if password != "" || cached {
			clearCachedArchivePassword(archivePath)
		}
		password, err = getArchivePassword(ctx, ArchivePasswordRequest{
			ArchivePath: archivePath,
			Format:      archivePasswordFormatName(format),
			Retry:       retry || password != "" || cached,
		})
		if err != nil {
			return fmt.Errorf("%w: %v", ErrArchivePasswordRequired, err)
		}
		cached = false
		retry = true
	}
	if lastErr == nil {
		lastErr = ErrArchivePasswordRequired
	}
	return lastErr
}

func applyArchiveFormatOptions(format archives.Format, opts ArchiveOptions, password string) archives.Format {
	switch f := format.(type) {
	case archives.Zip:
		return archives.Zip{
			SelectiveCompression: f.SelectiveCompression,
			Compression:          f.Compression,
			ContinueOnError:      f.ContinueOnError,
			TextEncoding:         archiveZipNameEncoding(opts),
		}
	case archives.SevenZip:
		f.Password = password
		return f
	case archives.Rar:
		f.Password = password
		return f
	case archives.CompressedArchive:
		if f.Extraction != nil {
			if extraction, ok := applyArchiveFormatOptions(f.Extraction, opts, password).(archives.Extraction); ok {
				f.Extraction = extraction
			}
		}
		if f.Archival != nil {
			if archival, ok := applyArchiveFormatOptions(f.Archival, opts, password).(archives.Archival); ok {
				f.Archival = archival
			}
		}
		return f
	default:
		return format
	}
}

func archiveFormatSupportsPassword(format archives.Format) bool {
	switch f := format.(type) {
	case archives.SevenZip, archives.Rar:
		return true
	case archives.CompressedArchive:
		if f.Extraction != nil && archiveFormatSupportsPassword(f.Extraction) {
			return true
		}
		if f.Archival != nil && archiveFormatSupportsPassword(f.Archival) {
			return true
		}
	}
	return false
}

func archivePasswordFormatName(format archives.Format) string {
	switch f := format.(type) {
	case archives.SevenZip:
		return "7z"
	case archives.Rar:
		return "RAR"
	case archives.CompressedArchive:
		if f.Extraction != nil {
			return archivePasswordFormatName(f.Extraction)
		}
		if f.Archival != nil {
			return archivePasswordFormatName(f.Archival)
		}
	}
	return "archive"
}

func isArchivePasswordError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrArchivePasswordRequired) ||
		errors.Is(err, rardecode.ErrArchiveEncrypted) ||
		errors.Is(err, rardecode.ErrArchivedFileEncrypted) ||
		errors.Is(err, rardecode.ErrBadPassword) {
		return true
	}
	var readErr *sevenzip.ReadError
	return errors.As(err, &readErr) && readErr.Encrypted
}

func validateArchiveFileSystem(ctx context.Context, localPath string, extractor archives.Extractor, probeReadable bool) error {
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
		if probeReadable && !entry.IsDir() {
			if err := probeArchiveEntry(entry); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("validate archive entries: %w", err)
	}
	return nil
}

func probeArchiveEntry(entry archives.FileInfo) error {
	if entry.Open == nil {
		return nil
	}
	f, err := entry.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	var buf [1]byte
	_, err = f.Read(buf[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func probeArchiveReadable(ctx context.Context, localPath string, extractor archives.Extractor) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	probed := false
	err = extractor.Extract(ctx, file, func(ctx context.Context, entry archives.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if err := ValidateArchiveEntryPath(entry.NameInArchive, false); err != nil {
			return err
		}
		probed = true
		if err := probeArchiveEntry(entry); err != nil {
			return err
		}
		return fs.SkipAll
	})
	if err != nil {
		return fmt.Errorf("probe archive readability: %w", err)
	}
	if !probed {
		return nil
	}
	return nil
}

// ExtractArchive streams entries from archivePath without building an archive
// filesystem index. The callback must consume file contents before returning.
func ExtractArchive(ctx context.Context, archivePath string, handler func(context.Context, ArchiveEntry) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if handler == nil {
		return errors.New("archive extract handler is nil")
	}
	if IsArchivePath(archivePath) {
		return fmt.Errorf("nested archive paths are not supported: %s", archivePath)
	}

	localPath, tempPath, err := archiveLocalPath(archivePath)
	if err != nil {
		return err
	}
	if tempPath != "" {
		defer os.Remove(tempPath)
	}

	err = withArchivePasswordRetry(ctx, archivePath, localPath, currentArchiveOptions(), true, func(format archives.Format) error {
		extractor, ok := format.(archives.Extractor)
		if !ok {
			return fmt.Errorf("format is not extractable: %s", archivePath)
		}

		file, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer file.Close()

		return extractor.Extract(ctx, file, func(ctx context.Context, entry archives.FileInfo) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := path.Clean(entry.NameInArchive)
			if err := ValidateArchiveEntryPath(entry.NameInArchive, entry.IsDir()); err != nil {
				return err
			}
			info := entry.FileInfo
			if info == nil {
				return fmt.Errorf("archive entry has no file info: %s", name)
			}
			out := ArchiveEntry{
				Name:       name,
				Info:       info,
				LinkTarget: entry.LinkTarget,
				Open: func() (io.ReadCloser, error) {
					if entry.Open == nil {
						return nil, fmt.Errorf("archive entry is not readable: %s", name)
					}
					f, err := entry.Open()
					if err != nil {
						return nil, err
					}
					return f, nil
				},
			}
			return handler(ctx, out)
		})
	})
	if err != nil {
		return fmt.Errorf("extract archive entries: %w", err)
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
	var f fs.File
	for attempt := 0; attempt < 4; attempt++ {
		f, err = a.fsys.Open(native)
		if err == nil || !isArchivePasswordError(err) {
			break
		}
		if retryErr := a.retryWithArchivePassword(context.Background(), attempt > 0); retryErr != nil {
			return nil, retryErr
		}
	}
	if err != nil {
		return nil, err
	}
	if rc, ok := f.(io.ReadCloser); ok {
		return rc, nil
	}
	_ = f.Close()
	return nil, fmt.Errorf("archive entry is not readable: %s", p)
}

func (a *ArchiveVFS) retryWithArchivePassword(ctx context.Context, retry bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	format, err := identifyArchiveFormat(ctx, a.localPath)
	if err != nil {
		return err
	}
	if retry {
		clearCachedArchivePassword(a.archivePath)
	}
	password, err := getArchivePassword(ctx, ArchivePasswordRequest{
		ArchivePath: a.archivePath,
		Format:      archivePasswordFormatName(format),
		Retry:       retry,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrArchivePasswordRequired, err)
	}
	format = applyArchiveFormatOptions(format, currentArchiveOptions(), password)
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("format is not extractable: %s", a.localPath)
	}
	if err := validateArchiveFileSystem(ctx, a.localPath, extractor, false); err != nil {
		return err
	}
	fsys := &archives.ArchiveFS{
		Path:    a.localPath,
		Format:  extractor,
		Context: ctx,
	}
	a.fsys = fsys
	putCachedArchivePassword(a.archivePath, password)
	return nil
}

func (a *ArchiveVFS) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	tempPath := a.tempPath
	a.tempPath = ""
	a.mu.Unlock()
	if tempPath == "" {
		return nil
	}
	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func archiveLocalPath(displayPath string) (localPath, tempPath string, err error) {
	vfs, parsed, err := ResolveRead(displayPath)
	if err != nil {
		return "", "", err
	}
	defer CloseVFS(vfs)
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
