package secret

// Store abstracts a secure credentials store (e.g., OS keyring).
// Implementations should be safe to call from multiple goroutines.
type Store interface {
	Get(host, share string) (domain, user, pass string, found bool, err error)
	Set(host, share, domain, user, pass string) error
	Delete(host, share string) error
}
