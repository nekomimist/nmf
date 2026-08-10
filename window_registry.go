package main

import "sync"

// fileManagerWindowRegistry owns the application-wide ordered set of file
// manager windows and the paths available for reopen. It belongs to
// ApplicationRuntime; keeping it out of package globals lets independent app
// runtimes and tests manage their own window lifecycle.
type fileManagerWindowRegistry struct {
	mu          sync.RWMutex
	windows     []*FileManager
	reopenPaths []string
}

func newFileManagerWindowRegistry() *fileManagerWindowRegistry {
	return &fileManagerWindowRegistry{}
}

func (r *fileManagerWindowRegistry) register(fm *FileManager) int {
	if r == nil || fm == nil || fm.window == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, candidate := range r.windows {
		if candidate == fm {
			return len(r.windows)
		}
	}
	r.windows = append(r.windows, fm)
	return len(r.windows)
}

func (r *fileManagerWindowRegistry) unregister(fm *FileManager) int {
	if r == nil || fm == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, candidate := range r.windows {
		if candidate == fm {
			r.windows = append(r.windows[:i], r.windows[i+1:]...)
			break
		}
	}
	return len(r.windows)
}

func (r *fileManagerWindowRegistry) count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.windows)
}

func (r *fileManagerWindowRegistry) recordReopenPath(path string) {
	if r == nil || path == "" {
		return
	}
	r.mu.Lock()
	r.reopenPaths = append(r.reopenPaths, path)
	r.mu.Unlock()
}

func (r *fileManagerWindowRegistry) nextReopenPath() (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reopenPaths) == 0 {
		return "", false
	}
	last := len(r.reopenPaths) - 1
	path := r.reopenPaths[last]
	r.reopenPaths = r.reopenPaths[:last]
	return path, true
}

func (r *fileManagerWindowRegistry) snapshot() []*FileManager {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := make([]*FileManager, 0, len(r.windows))
	for _, fm := range r.windows {
		if fm != nil && fm.window != nil {
			snapshot = append(snapshot, fm)
		}
	}
	return snapshot
}

func (fm *FileManager) windowRegistry() *fileManagerWindowRegistry {
	if fm == nil || fm.runtime == nil {
		return nil
	}
	return fm.runtime.windows
}

func (fm *FileManager) registeredWindows() []*FileManager {
	if fm == nil {
		return nil
	}
	// Isolated FileManager values in unit tests intentionally have no runtime;
	// treat only that explicit case as a one-window registry. A production
	// runtime with a missing/empty registry must remain visible as an invariant
	// violation instead of being silently replaced with fm.
	if fm.runtime == nil {
		return []*FileManager{fm}
	}
	if registry := fm.windowRegistry(); registry != nil {
		return registry.snapshot()
	}
	return nil
}

func registerFileManagerWindow(fm *FileManager) int {
	if fm == nil || fm.window == nil {
		return 0
	}
	registry := fm.windowRegistry()
	if registry == nil {
		return 0
	}
	return registry.register(fm)
}

func unregisterFileManagerWindow(fm *FileManager) int {
	if fm == nil {
		return 0
	}
	registry := fm.windowRegistry()
	if registry == nil {
		return 0
	}
	return registry.unregister(fm)
}
