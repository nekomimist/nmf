package fileinfo

import (
	"context"
	"sync"

	"nmf/internal/secret"
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
	Get(context.Context, string, string, string) (Credentials, error)
}

var (
	credentialsMu sync.RWMutex
	credProvider  CredentialsProvider
	secretStore   secret.Store
)

// SetCredentialsProvider sets the global credentials provider used by SMBFS.
func SetCredentialsProvider(p CredentialsProvider) {
	credentialsMu.Lock()
	credProvider = p
	credentialsMu.Unlock()
}

// SetSecretStore sets the global secret store (OS keyring). If nil, only memory cache will be used.
func SetSecretStore(s secret.Store) {
	credentialsMu.Lock()
	secretStore = s
	credentialsMu.Unlock()
}

func currentCredentialsProvider() CredentialsProvider {
	credentialsMu.RLock()
	defer credentialsMu.RUnlock()
	return credProvider
}

func currentSecretStore() secret.Store {
	credentialsMu.RLock()
	defer credentialsMu.RUnlock()
	return secretStore
}

func getCredentials(ctx context.Context, host, share, rel string) (Credentials, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// 1) Prefer in-memory cached credentials (e.g., seeded from URL)
	if c, ok := GetCachedCredentials(host, share); ok {
		return c, nil
	}
	// 2) Then try keyring (if available)
	if store := currentSecretStore(); store != nil {
		if d, u, p, found, _ := store.Get(host, share); found {
			c := Credentials{Domain: d, Username: u, Password: p}
			// Seed memory cache for this session
			PutCachedCredentials(host, share, c)
			return c, nil
		}
	}
	// 3) Finally, ask provider (may prompt UI). The provider itself caches.
	provider := currentCredentialsProvider()
	if provider == nil {
		return Credentials{}, nil
	}
	c, err := provider.Get(ctx, host, share, rel)
	if err != nil {
		return Credentials{}, err
	}
	return c, nil
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

func (p *CachedCredentialsProvider) Get(ctx context.Context, host, share, relPath string) (Credentials, error) {
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
	c, err := p.fallback.Get(ctx, host, share, relPath)
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
	if cp, ok := currentCredentialsProvider().(*CachedCredentialsProvider); ok {
		cp.Put(host, share, c)
	}
}

// GetCachedCredentials returns cached credentials if present in memory.
// It does not consult keyring or UI providers.
func GetCachedCredentials(host, share string) (Credentials, bool) {
	if cp, ok := currentCredentialsProvider().(*CachedCredentialsProvider); ok {
		cp.mu.RLock()
		defer cp.mu.RUnlock()
		if c, ok := cp.cache[host+"\x00"+share]; ok && (c.Username != "" || c.Password != "" || c.Domain != "") {
			return c, true
		}
	}
	return Credentials{}, false
}

// ClearCachedCredentials removes cached credentials for host/share from the
// in-memory cache only. Use it for per-operation failures, where the error says
// something about the requested path rather than about the login itself.
func ClearCachedCredentials(host, share string) {
	if cp, ok := currentCredentialsProvider().(*CachedCredentialsProvider); ok {
		cp.mu.Lock()
		delete(cp.cache, host+"\x00"+share)
		cp.mu.Unlock()
	}
}

// ClearRejectedLogin drops credentials the server refused at authentication
// time from the in-memory cache and, when the OS keyring is holding those same
// values, from the keyring too.
//
// Clearing only the memory copy is not recoverable: getCredentials consults the
// keyring before it prompts, so a password changed on the server would loop
// "keyring hit -> refused -> clear memory -> keyring hit" forever and the login
// dialog would never reappear. The keyring entry is removed only when it
// matches what was just refused, so credentials seeded from an smb:// URL or
// typed into the prompt cannot evict a different stored login.
func ClearRejectedLogin(host, share string, rejected Credentials) {
	ClearCachedCredentials(host, share)

	store := currentSecretStore()
	if store == nil {
		return
	}
	domain, user, pass, found, err := store.Get(host, share)
	if err != nil || !found {
		return
	}
	if domain != rejected.Domain || user != rejected.Username || pass != rejected.Password {
		return
	}
	_ = store.Delete(host, share)
}
