package secret

import (
	"fmt"

	"github.com/99designs/keyring"
)

const serviceName = "nmf.smb"

type keyringStore struct {
	ring keyring.Keyring
}

// NewKeyringStore tries to open the OS keyring via 99designs/keyring.
// If it fails, returns an error so callers can fallback to memory.
func NewKeyringStore() (Store, error) {
	r, err := keyring.Open(keyring.Config{ServiceName: serviceName})
	if err != nil {
		return nil, err
	}
	return &keyringStore{ring: r}, nil
}

func makeKey(host, share string) string { return fmt.Sprintf("%s|%s", host, share) }

func (s *keyringStore) Get(host, share string) (domain, user, pass string, found bool, err error) {
	item, err := s.ring.Get(makeKey(host, share))
	if err != nil {
		if err == keyring.ErrKeyNotFound {
			return "", "", "", false, nil
		}
		return "", "", "", false, err
	}
	// Store user/domain in item.Description as "domain\user"; password in item.Data
	desc := item.Description
	if desc != "" {
		// parse domain\user or user
		if i := indexRuneAny(desc, []rune{'\\', ';'}); i >= 0 {
			domain = desc[:i]
			user = desc[i+1:]
		} else {
			user = desc
		}
	}
	pass = string(item.Data)
	return domain, user, pass, true, nil
}

func (s *keyringStore) Set(host, share, domain, user, pass string) error {
	desc := user
	if domain != "" {
		desc = domain + "\\" + user
	}
	return s.ring.Set(keyring.Item{
		Key:         makeKey(host, share),
		Data:        []byte(pass),
		Description: desc,
		Label:       serviceName,
	})
}

func (s *keyringStore) Delete(host, share string) error {
	return s.ring.Remove(makeKey(host, share))
}

// indexRuneAny returns the first index of any rune in targets.
func indexRuneAny(s string, targets []rune) int {
	for i, r := range s {
		for _, t := range targets {
			if r == t {
				return i
			}
		}
	}
	return -1
}
