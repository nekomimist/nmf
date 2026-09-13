package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
		srcPath := extractSourcePath(src, entry.Name)
		if entry.Info.IsDir() {
			if err := ensureDir(execCtx, dst, entry.Info.Mode()); err != nil {
				return wrapPath(dst.displayPath(), err)
			}
			if shouldPreserveTimestamps(j) {
				if err := chtimesPath(execCtx, dst, entry.Info.ModTime(), entry.Info.ModTime()); err != nil {
					return wrapPath(dst.displayPath(), err)
				}
			}
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
	if err != nil {
		return wrapPath(src, err)
	}
	if rootCreated {
		j.addResult(Result{Source: src, Destination: root.displayPath(), DestinationCreated: true})
	}
	return nil
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
