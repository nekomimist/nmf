package browser

import (
	"context"
	"os"
	"sync"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

// DirectoryLoadHandle identifies one directory read. Starting a newer read
// cancels the context held by every older handle.
type DirectoryLoadHandle struct {
	ID      uint64
	Context context.Context
}

// DirectoryLoadRequest contains the immutable inputs captured by the UI
// coordinator before directory work moves to a background goroutine.
type DirectoryLoadRequest struct {
	Path                string
	AllowParentFallback bool
	Sort                config.SortConfig
}

// DirectoryLoadResult is widget-free data ready to apply to a browsing model.
type DirectoryLoadResult struct {
	RequestedPath      string
	Path               string
	Files              []fileinfo.FileInfo
	Storage            fileinfo.StorageInfo
	StorageErr         error
	UsedParentFallback bool
}

type DirectoryReadFunc func(context.Context, string) ([]os.DirEntry, error)
type storageStatFunc func(string) (fileinfo.StorageInfo, error)
type dirEntryInfoFunc func(string, os.DirEntry) (fileinfo.FileInfo, error)

// DirectoryLoader owns directory-read cancellation/generation state and the
// filesystem work needed to build a sorted listing. It never touches widgets.
type DirectoryLoader struct {
	mu sync.Mutex

	nextID   uint64
	activeID uint64
	cancel   context.CancelFunc

	readDir     DirectoryReadFunc
	statStorage storageStatFunc
	entryInfo   dirEntryInfoFunc
}

func NewDirectoryLoader() *DirectoryLoader {
	return newDirectoryLoader(
		fileinfo.ReadDirPortableContext,
		fileinfo.StatStoragePortable,
		fileinfo.FileInfoFromDirEntry,
	)
}

func newDirectoryLoader(readDir DirectoryReadFunc, statStorage storageStatFunc, entryInfo dirEntryInfoFunc) *DirectoryLoader {
	return &DirectoryLoader{
		readDir:     readDir,
		statStorage: statStorage,
		entryInfo:   entryInfo,
	}
}

// Begin starts a new load generation and cancels any generation it replaces.
func (l *DirectoryLoader) Begin() DirectoryLoadHandle {
	if l == nil {
		return DirectoryLoadHandle{}
	}
	ctx, cancel := context.WithCancel(context.Background())

	l.mu.Lock()
	previousCancel := l.cancel
	l.nextID++
	handle := DirectoryLoadHandle{ID: l.nextID, Context: ctx}
	l.activeID = handle.ID
	l.cancel = cancel
	l.mu.Unlock()

	if previousCancel != nil {
		previousCancel()
	}
	return handle
}

// Load performs one directory read synchronously. Callers normally invoke it
// from a background goroutine and then use Finish on the UI thread before
// applying the result.
func (l *DirectoryLoader) Load(handle DirectoryLoadHandle, request DirectoryLoadRequest, onCandidate func(string)) (DirectoryLoadResult, error) {
	result := DirectoryLoadResult{RequestedPath: request.Path}
	ctx := handle.Context
	if ctx == nil {
		ctx = context.Background()
	}

	entries, loadedPath, usedFallback, err := ReadDirectoryWithParentFallback(
		ctx,
		request.Path,
		request.AllowParentFallback,
		l.readDir,
		onCandidate,
	)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	result.Path = loadedPath
	result.UsedParentFallback = usedFallback
	result.Storage, result.StorageErr = l.statStorage(loadedPath)
	if err := ctx.Err(); err != nil {
		return result, err
	}

	files := make([]fileinfo.FileInfo, 0, len(entries)+1)
	parent := fileinfo.ParentPath(loadedPath)
	if parent != loadedPath {
		files = append(files, fileinfo.FileInfo{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			Modified: time.Time{},
			FileType: fileinfo.FileTypeDirectory,
			Status:   fileinfo.StatusNormal,
		})
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		info, err := l.entryInfo(loadedPath, entry)
		if err != nil {
			continue
		}
		files = append(files, info)
	}
	result.Files = SortFiles(files, request.Sort)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// Finish claims a completed result if it still belongs to the active load.
// A stale completion returns false and cannot replace newer browsing state.
func (l *DirectoryLoader) Finish(id uint64) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	if l.activeID != id {
		l.mu.Unlock()
		return false
	}
	cancel := l.cancel
	l.activeID = 0
	l.cancel = nil
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return true
}

func (l *DirectoryLoader) Active(id uint64) bool {
	if l == nil || id == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.activeID == id
}

// Cancel invalidates the matching load. It leaves a newer active load alone.
func (l *DirectoryLoader) Cancel(id uint64) bool {
	if l == nil || id == 0 {
		return false
	}
	l.mu.Lock()
	if l.activeID != id {
		l.mu.Unlock()
		return false
	}
	cancel := l.cancel
	l.activeID = 0
	l.cancel = nil
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return true
}

// CancelActive invalidates whichever generation is current. It is used when
// a window closes and must not restart any UI or watcher state.
func (l *DirectoryLoader) CancelActive() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	id := l.activeID
	l.mu.Unlock()
	return l.Cancel(id)
}

// ReadDirectoryWithParentFallback reads requestedPath and walks through
// portable parent paths only for confirmed not-exist failures. The optional
// callback receives each parent immediately before it is probed.
func ReadDirectoryWithParentFallback(
	ctx context.Context,
	requestedPath string,
	allowParentFallback bool,
	readDir DirectoryReadFunc,
	onCandidate ...func(string),
) ([]os.DirEntry, string, bool, error) {
	var notify func(string)
	if len(onCandidate) > 0 {
		notify = onCandidate[0]
	}
	path := requestedPath
	for {
		if err := ctx.Err(); err != nil {
			return nil, "", false, err
		}

		entries, err := readDir(ctx, path)
		if err == nil {
			return entries, path, path != requestedPath, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", false, ctxErr
		}
		if !allowParentFallback || !fileinfo.IsNotExist(err) {
			return nil, "", false, err
		}

		parent := fileinfo.ParentPath(path)
		if parent == path {
			return nil, "", false, err
		}
		path = parent
		if notify != nil {
			notify(path)
		}
	}
}
