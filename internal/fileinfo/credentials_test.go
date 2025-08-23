package fileinfo

import (
	"testing"
)

// stub secret store for tests
type stubSecret struct {
	d, u, p string
	found   bool
}

func (s stubSecret) Get(host, share string) (string, string, string, bool, error) {
	return s.d, s.u, s.p, s.found, nil
}
func (s stubSecret) Set(host, share, d, u, p string) error { return nil }
func (s stubSecret) Delete(host, share string) error       { return nil }

// stub provider counting calls
type countingProv struct {
	calls int
	ret   Credentials
}

func (c *countingProv) Get(host, share, rel string) (Credentials, error) {
	c.calls++
	return c.ret, nil
}

func TestCredentialsPrecedence_MemoryFirst(t *testing.T) {
	// provider with a known return, but we expect memory to win and provider not called
	base := &countingProv{ret: Credentials{Domain: "pd", Username: "pu", Password: "pp"}}
	SetCredentialsProvider(NewCachedCredentialsProvider(base))
	// keyring with different creds (should be ignored due to memory hit)
	SetSecretStore(stubSecret{d: "kd", u: "ku", p: "kp", found: true})

	// seed memory (e.g., from URL)
	PutCachedCredentials("host", "share", Credentials{Domain: "md", Username: "mu", Password: "mp"})

	got := getCredentials("host", "share", "")
	if got.Username != "mu" || got.Password != "mp" || got.Domain != "md" {
		t.Fatalf("memory creds not preferred: %+v", got)
	}
	if base.calls != 0 {
		t.Fatalf("provider called despite memory hit")
	}
}

func TestCredentialsPrecedence_KeyringSecond(t *testing.T) {
	base := &countingProv{ret: Credentials{Domain: "pd", Username: "pu", Password: "pp"}}
	cp := NewCachedCredentialsProvider(base)
	SetCredentialsProvider(cp)
	SetSecretStore(stubSecret{d: "kd", u: "ku", p: "kp", found: true})

	// no memory seed
	got := getCredentials("h", "s", "")
	if got.Username != "ku" || got.Password != "kp" || got.Domain != "kd" {
		t.Fatalf("keyring creds not preferred: %+v", got)
	}
	// keyring hit should seed memory
	if _, ok := GetCachedCredentials("h", "s"); !ok {
		t.Fatalf("keyring result not seeded to memory cache")
	}
	if base.calls != 0 {
		t.Fatalf("provider called despite keyring hit")
	}
}

func TestCredentialsPrecedence_ProviderLast(t *testing.T) {
	base := &countingProv{ret: Credentials{Domain: "pd", Username: "pu", Password: "pp"}}
	SetCredentialsProvider(NewCachedCredentialsProvider(base))
	SetSecretStore(stubSecret{found: false})
	got := getCredentials("h2", "s2", "rel")
	if got.Username != "pu" || got.Password != "pp" || got.Domain != "pd" {
		t.Fatalf("provider creds not returned: %+v", got)
	}
	if base.calls != 1 {
		t.Fatalf("provider should be called exactly once, got %d", base.calls)
	}
}
