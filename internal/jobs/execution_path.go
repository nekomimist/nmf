package jobs

import (
	"errors"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strings"

	"nmf/internal/fileinfo"
)

type executionBackend int

const (
	backendLocal executionBackend = iota
	backendSMB
	backendArchive
)

type executionPath struct {
	localRoot      *os.Root
	rootDisplay    string
	raw            string
	path           string
	backend        executionBackend
	smb            fileinfo.SMBPathOps
	smbOpener      fileinfo.SMBSessionOpener
	smbDisplayRoot string
	archivePath    string
}

type executionContext struct {
	smbSessions map[string]fileinfo.SMBSession
	archiveVFSs map[string]*fileinfo.ArchiveVFS
	copyBuffer  []byte // reused by the serial transfers within one job
}

func newExecutionContext() *executionContext {
	return &executionContext{
		smbSessions: make(map[string]fileinfo.SMBSession),
		archiveVFSs: make(map[string]*fileinfo.ArchiveVFS),
	}
}

func (ctx *executionContext) close() error {
	if ctx == nil {
		return nil
	}
	ctx.copyBuffer = nil
	var closeErr error
	for key, session := range ctx.smbSessions {
		if session == nil {
			continue
		}
		if err := session.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("%s: %w", key, err))
		}
	}
	ctx.smbSessions = make(map[string]fileinfo.SMBSession)
	for key, vfs := range ctx.archiveVFSs {
		if err := vfs.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("%s: %w", key, err))
		}
	}
	ctx.archiveVFSs = make(map[string]*fileinfo.ArchiveVFS)
	return closeErr
}

func (ctx *executionContext) archiveVFSFor(p executionPath) (*fileinfo.ArchiveVFS, error) {
	if p.backend != backendArchive {
		return nil, fmt.Errorf("path is not archive-backed: %s", p.displayPath())
	}
	if ctx == nil {
		return fileinfo.NewArchiveVFS(p.archivePath)
	}
	if vfs, ok := ctx.archiveVFSs[p.archivePath]; ok && vfs != nil {
		return vfs, nil
	}
	vfs, err := fileinfo.NewArchiveVFS(p.archivePath)
	if err != nil {
		return nil, err
	}
	ctx.archiveVFSs[p.archivePath] = vfs
	return vfs, nil
}

func (ctx *executionContext) smbOpsFor(p executionPath) (fileinfo.SMBPathOps, error) {
	if p.backend != backendSMB {
		return nil, fmt.Errorf("path is not SMB-backed: %s", p.displayPath())
	}
	if p.smb == nil {
		return nil, fmt.Errorf("SMB backend is unavailable: %s", p.displayPath())
	}
	if p.smbOpener == nil || ctx == nil {
		return p.smb, nil
	}

	key := p.smbDisplayRoot
	if key == "" {
		key = "smb://"
	}
	if session, ok := ctx.smbSessions[key]; ok && session != nil {
		return session, nil
	}

	session, err := p.smbOpener.OpenSession()
	if err != nil {
		return nil, err
	}
	ctx.smbSessions[key] = session
	return session, nil
}

func (p executionPath) displayPath() string {
	if p.localRoot != nil {
		return filepath.Join(p.rootDisplay, p.path)
	}
	switch p.backend {
	case backendArchive:
		return fileinfo.ArchiveDisplayPath(p.archivePath, p.path)
	case backendSMB:
		root := p.smbDisplayRoot
		if root == "" {
			root = "smb://"
		}
		rel := strings.TrimPrefix(strings.ReplaceAll(p.path, "\\", "/"), "/")
		if rel == "" {
			return root
		}
		return root + "/" + rel
	default:
		return p.path
	}
}

func sameExecutionPath(a, b executionPath) bool {
	if a.backend != b.backend {
		return false
	}
	if a.backend == backendArchive {
		return a.archivePath == b.archivePath && normalizeSMBExecutionPath(a.path) == normalizeSMBExecutionPath(b.path)
	}
	if a.backend == backendSMB {
		return normalizeSMBRoot(a.smbDisplayRoot) == normalizeSMBRoot(b.smbDisplayRoot) &&
			normalizeSMBExecutionPath(a.path) == normalizeSMBExecutionPath(b.path)
	}

	ap := filepath.Clean(a.path)
	bp := filepath.Clean(b.path)
	if runtime.GOOS == "windows" {
		ap = strings.ToLower(ap)
		bp = strings.ToLower(bp)
	}
	return ap == bp
}

func isDescendantExecutionPath(child, parent executionPath) bool {
	if child.backend != parent.backend {
		return false
	}
	switch child.backend {
	case backendArchive:
		if child.archivePath != parent.archivePath {
			return false
		}
		return isDescendantSlashPath(child.path, parent.path)
	case backendSMB:
		if normalizeSMBRoot(child.smbDisplayRoot) != normalizeSMBRoot(parent.smbDisplayRoot) {
			return false
		}
		return isDescendantSlashPath(child.path, parent.path)
	default:
		childPath := filepath.Clean(child.path)
		parentPath := filepath.Clean(parent.path)
		if runtime.GOOS == "windows" {
			childPath = strings.ToLower(childPath)
			parentPath = strings.ToLower(parentPath)
		}
		if childPath == parentPath {
			return false
		}
		rel, err := filepath.Rel(parentPath, childPath)
		if err != nil {
			return false
		}
		return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	}
}

func validateDirectoryTransfer(src, dst executionPath, operation Type) error {
	if src.backend == backendLocal && dst.backend == backendLocal {
		// The destination parent exists. Resolve aliases before adding the final
		// name, which may not exist yet, so a symlink/junction cannot hide a
		// destination beneath the source.
		source, err := filepath.EvalSymlinks(src.path)
		if err != nil {
			return wrapPath(src.displayPath(), err)
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(dst.path))
		if err != nil {
			return wrapPath(dst.displayPath(), err)
		}
		src.path = source
		dst.path = filepath.Join(parent, filepath.Base(dst.path))
		if resolved, err := filepath.EvalSymlinks(dst.path); err == nil {
			dst.path = resolved
		} else if !os.IsNotExist(err) {
			return wrapPath(dst.displayPath(), err)
		}
	}
	if sameExecutionPath(src, dst) || isDescendantExecutionPath(dst, src) {
		return wrapPath(dst.displayPath(), fmt.Errorf("cannot %s a directory into itself", operation))
	}
	return nil
}

func isDescendantSlashPath(child, parent string) bool {
	childPath := normalizeSMBExecutionPath(child)
	parentPath := normalizeSMBExecutionPath(parent)
	if childPath == parentPath {
		return false
	}
	if parentPath == "/" {
		return strings.HasPrefix(childPath, "/") && childPath != "/"
	}
	return strings.HasPrefix(childPath, strings.TrimRight(parentPath, "/")+"/")
}

func pathExists(execCtx *executionContext, p executionPath) (bool, error) {
	if _, err := lstatPath(execCtx, p); err == nil {
		return true, nil
	} else if fileinfo.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func nextAvailablePath(execCtx *executionContext, dst executionPath) (executionPath, error) {
	dir := dirPath(dst)
	stem, ext := splitCopyName(baseName(dst))
	for i := 1; ; i++ {
		candidate := joinPath(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		exists, err := pathExists(execCtx, candidate)
		if err != nil {
			return candidate, wrapPath(candidate.displayPath(), err)
		}
		if !exists {
			return candidate, nil
		}
	}
}

func splitCopyName(name string) (string, string) {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return name, ""
	}
	return name[:dot], name[dot:]
}

func normalizeSMBRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "smb://"
	}
	root = strings.ReplaceAll(root, "\\", "/")
	root = strings.TrimRight(root, "/")
	return strings.ToLower(root)
}

func normalizeSMBExecutionPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = pathpkg.Clean("/" + strings.TrimPrefix(p, "/"))
	if p == "." {
		return "/"
	}
	return p
}

// resolveExecutionPath maps display paths to backend-specific execution paths.
func resolveExecutionPath(p string) (executionPath, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return executionPath{}, errors.New("path is empty")
	}

	vfs, parsed, err := fileinfo.ResolveRead(p)
	if err != nil {
		return executionPath{}, err
	}

	native := parsed.Native
	if native == "" {
		native = p
	}

	if parsed.Scheme == fileinfo.SchemeArchive {
		_ = fileinfo.CloseVFS(vfs)
		if native == "" {
			native = "."
		}
		return executionPath{
			raw:         p,
			path:        native,
			backend:     backendArchive,
			archivePath: parsed.Archive,
		}, nil
	}

	if parsed.Scheme == fileinfo.SchemeSMB && parsed.Provider != "local" {
		smb, ok := vfs.(fileinfo.SMBPathOps)
		if !ok {
			_ = fileinfo.CloseVFS(vfs)
			return executionPath{}, fmt.Errorf("direct SMB provider is unavailable on this platform: %s", p)
		}
		opener, _ := vfs.(fileinfo.SMBSessionOpener)
		root := "smb://"
		if parsed.Host != "" && parsed.Share != "" {
			root = "smb://" + pathpkg.Join(parsed.Host, parsed.Share)
		}
		if native == "" {
			native = "/"
		}
		return executionPath{
			raw:            p,
			path:           native,
			backend:        backendSMB,
			smb:            smb,
			smbOpener:      opener,
			smbDisplayRoot: root,
		}, nil
	}

	_ = fileinfo.CloseVFS(vfs)
	return executionPath{
		raw:     p,
		path:    native,
		backend: backendLocal,
	}, nil
}

func baseName(p executionPath) string {
	if p.backend == backendArchive {
		if p.path == "." {
			return fileinfo.BaseName(p.archivePath)
		}
		return pathpkg.Base(p.path)
	}
	if p.backend == backendSMB {
		return p.smb.Base(p.path)
	}
	return filepath.Base(p.path)
}

func joinPath(base executionPath, name string) executionPath {
	out := base
	if base.backend == backendArchive {
		if base.path == "." {
			out.path = pathpkg.Clean(name)
		} else {
			out.path = pathpkg.Join(base.path, name)
		}
	} else if base.backend == backendSMB {
		out.path = base.smb.Join(base.path, name)
	} else {
		out.path = filepath.Join(base.path, name)
	}
	out.raw = out.path
	return out
}

func dirPath(p executionPath) executionPath {
	out := p
	if p.backend == backendArchive {
		parent := pathpkg.Dir(strings.TrimPrefix(p.path, "/"))
		if parent == "." || parent == "/" {
			parent = "."
		}
		out.path = parent
		out.raw = out.path
		return out
	}
	if p.backend == backendSMB {
		clean := strings.ReplaceAll(p.path, "\\", "/")
		parent := pathpkg.Dir(clean)
		if parent == "." {
			parent = ""
		}
		out.path = parent
		out.raw = out.path
		return out
	}
	out.path = filepath.Dir(p.path)
	out.raw = out.path
	return out
}
