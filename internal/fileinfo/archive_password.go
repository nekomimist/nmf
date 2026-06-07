package fileinfo

import (
	"context"
	"errors"
	"sync"
)

// ErrArchivePasswordRequired reports that an archive needs a password before
// entries can be listed or read.
var ErrArchivePasswordRequired = errors.New("archive password required")

// ArchivePasswordRequest describes an interactive archive password request.
type ArchivePasswordRequest struct {
	ArchivePath string
	Format      string
	Retry       bool
}

// ArchivePasswordProvider can interactively or programmatically provide archive
// passwords.
type ArchivePasswordProvider interface {
	GetArchivePassword(context.Context, ArchivePasswordRequest) (string, error)
}

var archivePasswordProvider ArchivePasswordProvider

// SetArchivePasswordProvider sets the global provider used for encrypted 7z/RAR
// archive reads.
func SetArchivePasswordProvider(p ArchivePasswordProvider) {
	archivePasswordProvider = p
}

// CachedArchivePasswordProvider caches archive passwords in memory per archive
// display path.
type CachedArchivePasswordProvider struct {
	fallback ArchivePasswordProvider
	cache    map[string]string
	mu       sync.RWMutex
}

// NewCachedArchivePasswordProvider creates a session-only cache around fallback.
func NewCachedArchivePasswordProvider(fallback ArchivePasswordProvider) *CachedArchivePasswordProvider {
	return &CachedArchivePasswordProvider{fallback: fallback, cache: make(map[string]string)}
}

func (p *CachedArchivePasswordProvider) GetArchivePassword(ctx context.Context, req ArchivePasswordRequest) (string, error) {
	if req.ArchivePath == "" {
		return "", ErrArchivePasswordRequired
	}
	if !req.Retry {
		p.mu.RLock()
		pass, ok := p.cache[req.ArchivePath]
		p.mu.RUnlock()
		if ok {
			return pass, nil
		}
	}
	if p.fallback == nil {
		return "", ErrArchivePasswordRequired
	}
	pass, err := p.fallback.GetArchivePassword(ctx, req)
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	if p.cache == nil {
		p.cache = make(map[string]string)
	}
	p.cache[req.ArchivePath] = pass
	p.mu.Unlock()
	return pass, nil
}

func (p *CachedArchivePasswordProvider) Put(archivePath, password string) {
	if archivePath == "" {
		return
	}
	p.mu.Lock()
	if p.cache == nil {
		p.cache = make(map[string]string)
	}
	p.cache[archivePath] = password
	p.mu.Unlock()
}

func (p *CachedArchivePasswordProvider) Clear(archivePath string) {
	p.mu.Lock()
	delete(p.cache, archivePath)
	p.mu.Unlock()
}

func putCachedArchivePassword(archivePath, password string) {
	if cp, ok := archivePasswordProvider.(*CachedArchivePasswordProvider); ok {
		cp.Put(archivePath, password)
	}
}

func clearCachedArchivePassword(archivePath string) {
	if cp, ok := archivePasswordProvider.(*CachedArchivePasswordProvider); ok {
		cp.Clear(archivePath)
	}
}

func cachedArchivePassword(archivePath string) (string, bool) {
	if cp, ok := archivePasswordProvider.(*CachedArchivePasswordProvider); ok {
		cp.mu.RLock()
		defer cp.mu.RUnlock()
		pass, ok := cp.cache[archivePath]
		return pass, ok
	}
	return "", false
}

func getArchivePassword(ctx context.Context, req ArchivePasswordRequest) (string, error) {
	if archivePasswordProvider == nil {
		return "", ErrArchivePasswordRequired
	}
	return archivePasswordProvider.GetArchivePassword(ctx, req)
}
