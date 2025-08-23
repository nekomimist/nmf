package fileinfo

import (
	"nmf/internal/secret"
	"sync"
)

// Credentials represents SMB authentication parameters.
type Credentials struct {
	Domain   string
	Username string
	Password string
	Persist  bool
}

// CredentialsProvider can interactively or programmatically provide credentials.
type CredentialsProvider interface {
	Get(host, share, relPath string) (Credentials, error)
}

var credProvider CredentialsProvider
var secretStore secret.Store

// SetCredentialsProvider sets the global credentials provider used by SMBFS.
func SetCredentialsProvider(p CredentialsProvider) { credProvider = p }

// SetSecretStore sets the global secret store (OS keyring). If nil, only memory cache will be used.
func SetSecretStore(s secret.Store) { secretStore = s }

func getCredentials(host, share, rel string) Credentials {
	// 1) Prefer in-memory cached credentials (e.g., seeded from URL)
	if c, ok := GetCachedCredentials(host, share); ok {
		return c
	}
	// 2) Then try keyring (if available)
	if secretStore != nil {
		if d, u, p, found, _ := secretStore.Get(host, share); found {
			c := Credentials{Domain: d, Username: u, Password: p}
			// Seed memory cache for this session
			PutCachedCredentials(host, share, c)
			return c
		}
	}
	// 3) Finally, ask provider (may prompt UI). The provider itself caches.
	if credProvider == nil {
		return Credentials{}
	}
	c, err := credProvider.Get(host, share, rel)
	if err != nil {
		return Credentials{}
	}
	return c
}

// CachedCredentialsProvider caches credentials per host/share in memory.
type CachedCredentialsProvider struct {
	fallback CredentialsProvider
	cache    map[string]Credentials
	mu       sync.RWMutex
}

// NewCachedCredentialsProvider creates a new caching provider wrapping fallback.
func NewCachedCredentialsProvider(fallback CredentialsProvider) *CachedCredentialsProvider {
	return &CachedCredentialsProvider{fallback: fallback, cache: make(map[string]Credentials)}
}

func (p *CachedCredentialsProvider) Get(host, share, relPath string) (Credentials, error) {
	key := host + "\x00" + share
	p.mu.RLock()
	if c, ok := p.cache[key]; ok && (c.Username != "" || c.Password != "" || c.Domain != "") {
		p.mu.RUnlock()
		return c, nil
	}
	p.mu.RUnlock()
	if p.fallback == nil {
		return Credentials{}, nil
	}
	c, err := p.fallback.Get(host, share, relPath)
	if err != nil {
		return c, err
	}
	p.mu.Lock()
	if p.cache == nil {
		p.cache = make(map[string]Credentials)
	}
	p.cache[key] = c
	p.mu.Unlock()
	return c, nil
}

// Put allows programmatic seeding of cached credentials (e.g., from URL).
func (p *CachedCredentialsProvider) Put(host, share string, c Credentials) {
	p.mu.Lock()
	if p.cache == nil {
		p.cache = make(map[string]Credentials)
	}
	p.cache[host+"\x00"+share] = c
	p.mu.Unlock()
}

// PutCachedCredentials seeds cached credentials if the global provider supports it.
func PutCachedCredentials(host, share string, c Credentials) {
	if cp, ok := credProvider.(*CachedCredentialsProvider); ok {
		cp.Put(host, share, c)
	}
}

// GetCachedCredentials returns cached credentials if present in memory.
// It does not consult keyring or UI providers.
func GetCachedCredentials(host, share string) (Credentials, bool) {
	if cp, ok := credProvider.(*CachedCredentialsProvider); ok {
		cp.mu.RLock()
		defer cp.mu.RUnlock()
		if c, ok := cp.cache[host+"\x00"+share]; ok && (c.Username != "" || c.Password != "" || c.Domain != "") {
			return c, true
		}
	}
	return Credentials{}, false
}

// ClearCachedCredentials removes cached credentials for host/share.
func ClearCachedCredentials(host, share string) {
	if cp, ok := credProvider.(*CachedCredentialsProvider); ok {
		cp.mu.Lock()
		delete(cp.cache, host+"\x00"+share)
		cp.mu.Unlock()
	}
}
