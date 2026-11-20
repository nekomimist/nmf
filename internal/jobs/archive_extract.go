package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"nmf/internal/fileinfo"
)

type virtualFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (v virtualFileInfo) Name() string { return v.name }

func (v virtualFileInfo) Size() int64 { return v.size }

func (v virtualFileInfo) Mode() os.FileMode { return v.mode }

func (v virtualFileInfo) ModTime() time.Time { return v.modTime }

func (v virtualFileInfo) IsDir() bool { return v.mode.IsDir() }

func (v virtualFileInfo) Sys() any { return nil }

func extractArchivePath(j *Job, execCtx *executionContext, src string, destDir executionPath) error {
	if fileinfo.IsArchivePath(src) {
		return wrapPath(src, errors.New("nested archive extraction is not supported"))
	}
	rootName := extractRootName(fileinfo.BaseName(src))
	if rootName == "" || rootName == "." {
		return wrapPath(src, errors.New("archive name is not usable as an extract directory"))
	}
	if err := fileinfo.ValidateArchiveEntryBaseName(rootName); err != nil {
		return wrapPath(src, err)
	}

	if destDir.backend == backendLocal {
		anchor, err := os.OpenRoot(destDir.path)
		if err != nil {
			return wrapPath(destDir.displayPath(), err)
		}
		defer anchor.Close()
		destDir.rootDisplay = destDir.path
		destDir.path = "."
		destDir.localRoot = anchor
	}

	root := joinPath(destDir, rootName)
	rootInfo := virtualFileInfo{name: rootName, mode: os.ModeDir | 0755, modTime: time.Now()}
	root, skipped, _, err := resolveDestinationConflict(j, execCtx, extractSourcePath(src, "."), root, rootInfo)
	if err != nil {
		return err
	}
	if skipped {
		return errSkipped
	}
	rootCreated, err := createDirectoryIfMissing(execCtx, root, rootInfo.Mode())
	if err != nil {
		return wrapPath(root.displayPath(), err)
	}

	if root.localRoot != nil {
		anchored, err := anchorExtractionRoot(root)
		if err != nil {
			return wrapPath(root.displayPath(), err)
		}
		defer anchored.localRoot.Close()
		root = anchored
	}

	// Headers may precede or follow children. Remember newly created
	// directories, including implicit parents, until all entries are present.
	directories := make(map[string]*archiveDirectory)
	err = fileinfo.ExtractArchive(j.ctx, src, func(ctx context.Context, entry fileinfo.ArchiveEntry) error {
		if canceled(j) {
			return errCanceled
		}
		if entry.Name == "." {
			return nil
		}
		if err := fileinfo.ValidateArchiveEntryPath(entry.Name, entry.Info.IsDir()); err != nil {
			return wrapPath(src, err)
		}
		if entry.LinkTarget != "" || fileinfo.IsLinkModeCandidate(entry.Info.Mode()) {
			return wrapPath(fileinfo.ArchiveDisplayPath(src, entry.Name), errors.New("archive links are not supported for extraction"))
		}

		dst, err := archiveEntryDestination(root, entry.Name)
		if err != nil {
			return wrapPath(fileinfo.ArchiveDisplayPath(src, entry.Name), err)
		}
		parent := dirPath(dst)
		if entry.Info.IsDir() {
			parent = dst
		}
		if err := ensureSMBArchiveParents(execCtx, root, parent); err != nil {
			return err
		}
		if root.localRoot != nil {
			workingMode := os.FileMode(0755)
			if entry.Info.IsDir() {
				workingMode = entry.Info.Mode().Perm() | 0700
			}
			if err := prepareLocalArchiveDirectories(execCtx, parent, workingMode, directories); err != nil {
				return err
			}
		}
		srcPath := extractSourcePath(src, entry.Name)
		if entry.Info.IsDir() {
			record := directories[dst.path]
			if record == nil { // SMB parents were created and checked above.
				record = &archiveDirectory{path: dst}
				directories[dst.path] = record
			}
			record.info = entry.Info
			return nil
		}

		dst, skipped, overwrite, err := resolveDestinationConflict(j, execCtx, srcPath, dst, entry.Info)
		if err != nil {
			return err
		}
		if skipped {
			return nil
		}
		if err := ensureSMBArchiveParents(execCtx, root, dirPath(dst)); err != nil {
			return err
		}
		in, err := entry.Open()
		if err != nil {
			return wrapPath(srcPath.displayPath(), err)
		}
		defer in.Close()
		return copyReaderWithCancel(j, execCtx, in, srcPath.displayPath(), dst, entry.Info, overwrite)
	})
	finishErr := finishArchiveDirectories(j, execCtx, root, directories, err == nil && shouldPreserveTimestamps(j))
	if err != nil || finishErr != nil {
		return errors.Join(wrapPath(src, err), finishErr)
	}
	if rootCreated {
		j.addResult(Result{Source: src, Destination: root.displayPath(), DestinationCreated: true})
	}
	return nil
}

type archiveDirectory struct {
	path     executionPath
	created  bool
	identity os.FileInfo // local directory when first seen
	info     os.FileInfo // explicit archive directory header, if present
}

// The local extraction root uses relative paths. Install each implicit parent
// once so a later explicit header can set the mode of a directory we created.
func prepareLocalArchiveDirectories(execCtx *executionContext, dir executionPath, workingMode os.FileMode, directories map[string]*archiveDirectory) error {
	if dir.path == "." || directories[dir.path] != nil {
		return nil
	}
	if err := prepareLocalArchiveDirectories(execCtx, dirPath(dir), 0755, directories); err != nil {
		return err
	}
	created, err := createDirectoryIfMissing(execCtx, dir, workingMode)
	if err != nil {
		return wrapPath(dir.displayPath(), err)
	}
	identity, err := dir.localRoot.Lstat(dir.path)
	if err != nil {
		return wrapPath(dir.displayPath(), err)
	}
	directories[dir.path] = &archiveDirectory{path: dir, created: created, identity: identity}
	return nil
}

func finishArchiveDirectories(j *Job, execCtx *executionContext, root executionPath, directories map[string]*archiveDirectory, preserveTimes bool) error {
	ordered := make([]*archiveDirectory, 0, len(directories))
	for _, dir := range directories {
		if dir.info != nil && (dir.created || preserveTimes) {
			ordered = append(ordered, dir)
		}
	}
	// A child's path is always longer than its parent's. Finish children
	// first while every ancestor is still writable and traversable.
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i].path.path) > len(ordered[j].path.path) })
	var finishErr error
	for _, dir := range ordered {
		// Restore temporary permissions even if extraction failed or was
		// canceled. Finish independent directories after an error.
		if err := dir.restoreMode(); err != nil {
			finishErr = errors.Join(finishErr, wrapPath(dir.path.displayPath(), err))
			continue
		}
		if preserveTimes && !canceled(j) {
			if err := ensureSMBArchiveParents(execCtx, root, dir.path); err != nil {
				return errors.Join(finishErr, err)
			}
			mtime := dir.info.ModTime()
			if err := chtimesPath(execCtx, dir.path, mtime, mtime); err != nil {
				finishErr = errors.Join(finishErr, wrapPath(dir.path.displayPath(), err))
			}
		}
	}
	if canceled(j) {
		return errors.Join(finishErr, errCanceled)
	}
	return finishErr
}

func (dir *archiveDirectory) restoreMode() error {
	if !dir.created || runtime.GOOS == "windows" {
		return nil
	}
	// Only remove permissions. mkdir already applied the destination umask.
	mode := dir.identity.Mode() &^ (os.ModePerm &^ dir.info.Mode().Perm())
	if mode.Perm() == dir.identity.Mode().Perm() {
		return nil
	}
	f, err := openSameLocalDirectory(dir.path, dir.identity)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Chmod(mode)
}

func extractSourcePath(archivePath, inner string) executionPath {
	return executionPath{
		raw:         fileinfo.ArchiveDisplayPath(archivePath, inner),
		path:        inner,
		backend:     backendArchive,
		archivePath: archivePath,
	}
}

func archiveEntryDestination(root executionPath, entryName string) (executionPath, error) {
	dst := root
	for _, part := range strings.Split(entryName, "/") {
		if err := fileinfo.ValidateArchiveEntryBaseName(part); err != nil {
			return executionPath{}, err
		}
		dst = joinPath(dst, part)
	}
	return dst, nil
}

func extractRootName(name string) string {
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	for _, ext := range []string{
		".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tar.zstd",
		".tgz", ".tbz2", ".txz", ".tzst",
		".zip", ".7z", ".rar", ".tar",
	} {
		if strings.HasSuffix(lower, ext) && len(name) > len(ext) {
			return name[:len(name)-len(ext)]
		}
	}
	if ext := filepath.Ext(name); ext != "" && len(name) > len(ext) {
		return strings.TrimSuffix(name, ext)
	}
	return name
}
