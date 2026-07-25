package fileinfo

import (
	"context"
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

// recordingSecret tracks Delete so tests can assert keyring eviction.
type recordingSecret struct {
	d, u, p string
	found   bool
	deleted int
}

func (s *recordingSecret) Get(host, share string) (string, string, string, bool, error) {
	return s.d, s.u, s.p, s.found, nil
}
func (s *recordingSecret) Set(host, share, d, u, p string) error { return nil }
func (s *recordingSecret) Delete(host, share string) error {
	s.deleted++
	s.found = false
	return nil
}

// stub provider counting calls
type countingProv struct {
	calls int
	ret   Credentials
}

func (c *countingProv) Get(context.Context, string, string, string) (Credentials, error) {
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

	got, err := getCredentials(context.Background(), "host", "share", "")
	if err != nil {
		t.Fatal(err)
	}
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
	got, err := getCredentials(context.Background(), "h", "s", "")
	if err != nil {
		t.Fatal(err)
	}
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
	got, err := getCredentials(context.Background(), "h2", "s2", "rel")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "pu" || got.Password != "pp" || got.Domain != "pd" {
		t.Fatalf("provider creds not returned: %+v", got)
	}
	if base.calls != 1 {
		t.Fatalf("provider should be called exactly once, got %d", base.calls)
	}
}

// A password changed on the server must not lock the user out: once the stored
// credentials are refused at login, the keyring copy has to go too, otherwise
// getCredentials keeps re-reading it and the prompt never reappears.
func TestClearRejectedLoginEvictsMatchingKeyringEntry(t *testing.T) {
	base := &countingProv{ret: Credentials{Username: "typed", Password: "fresh"}}
	SetCredentialsProvider(NewCachedCredentialsProvider(base))
	store := &recordingSecret{d: "kd", u: "ku", p: "stale", found: true}
	SetSecretStore(store)

	stored, err := getCredentials(context.Background(), "nas", "pub", "")
	if err != nil {
		t.Fatal(err)
	}

	ClearRejectedLogin("nas", "pub", stored)

	if store.deleted != 1 {
		t.Fatalf("keyring Delete called %d times, want 1", store.deleted)
	}
	if _, ok := GetCachedCredentials("nas", "pub"); ok {
		t.Fatal("memory cache still holds the rejected credentials")
	}
	got, err := getCredentials(context.Background(), "nas", "pub", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != "fresh" || base.calls != 1 {
		t.Fatalf("prompt did not run after eviction: creds=%+v calls=%d", got, base.calls)
	}
}

// Credentials seeded from an smb:// URL or typed into the prompt must not evict
// a different login that the keyring is holding.
func TestClearRejectedLoginKeepsUnrelatedKeyringEntry(t *testing.T) {
	SetCredentialsProvider(NewCachedCredentialsProvider(&countingProv{}))
	store := &recordingSecret{d: "kd", u: "ku", p: "kp", found: true}
	SetSecretStore(store)

	ClearRejectedLogin("nas", "pub", Credentials{Username: "url-user", Password: "url-pass"})

	if store.deleted != 0 {
		t.Fatalf("keyring Delete called %d times, want 0", store.deleted)
	}
}

func TestClearRejectedLoginWithoutSecretStore(t *testing.T) {
	SetCredentialsProvider(NewCachedCredentialsProvider(&countingProv{}))
	SetSecretStore(nil)
	PutCachedCredentials("nas", "pub", Credentials{Username: "u", Password: "p"})

	ClearRejectedLogin("nas", "pub", Credentials{Username: "u", Password: "p"})

	if _, ok := GetCachedCredentials("nas", "pub"); ok {
		t.Fatal("memory cache still holds the rejected credentials")
	}
}
