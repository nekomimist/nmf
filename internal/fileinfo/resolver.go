package fileinfo

import (
	"errors"
	"path"
	"runtime"
	"strings"
)

// Scheme represents a logical protocol for display/history normalization.
type Scheme string

const (
	SchemeFile Scheme = "file"
	SchemeSMB  Scheme = "smb"
)

// Parsed contains a normalized view of an input path.
// Display is a canonical string (smb://host/share/seg...),
// Native is the provider-native absolute path used for I/O.
type Parsed struct {
	Scheme   Scheme
	Host     string
	Share    string
	Segments []string
	Raw      string
	Display  string
	Native   string
	Provider string // "local" | "smb" (reserved)
}

// ResolveRead maps input into a VFS provider and native path for ReadDir.
// Minimal implementation: Windows supports UNC and smb:// to UNC conversion.
// Other OS: only local paths are supported for now.
func ResolveRead(input string) (VFS, Parsed, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return LocalFS{}, Parsed{Raw: input, Scheme: SchemeFile, Display: input, Native: input, Provider: "local"}, nil
	}

	// Windows: support UNC and smb://
	if runtime.GOOS == "windows" {
		if isUNC(raw) {
			p := parseUNC(raw)
			p.Raw = input
			p.Provider = "local"
			return LocalFS{}, p, nil
		}
		if isSMBURL(raw) {
			unc := smbURLToUNC(raw)
			p := parseUNC(unc)
			p.Raw = input
			p.Provider = "local"
			return LocalFS{}, p, nil
		}
		// Fallback: treat as local path
		return LocalFS{}, Parsed{Raw: input, Scheme: SchemeFile, Display: input, Native: input, Provider: "local"}, nil
	}

	// Non-Windows: only local paths for now; do not reinterpret //server/share.
	if isSMBURL(raw) {
		return nil, Parsed{Raw: input, Scheme: SchemeSMB, Display: canonicalizeSMB(raw)}, errUnsupportedSMB()
	}
	return LocalFS{}, Parsed{Raw: input, Scheme: SchemeFile, Display: input, Native: input, Provider: "local"}, nil
}

// ReadDirPortable lists a directory using ResolveRead and LocalFS/other providers.
// ReadDirPortable is implemented in portable_read.go

// NormalizeInputPath converts user input to a provider-native path for navigation.
// On Windows, converts smb://host/share/... to \\host\share\...; otherwise returns input as-is.
func NormalizeInputPath(input string) string {
	if runtime.GOOS == "windows" && isSMBURL(input) {
		return smbURLToUNC(input)
	}
	return input
}

func isUNC(p string) bool {
	// Leading \\ or \\?\UNC\
	return strings.HasPrefix(p, `\\\\?\\UNC\\`) || strings.HasPrefix(p, `\\`)
}

func isSMBURL(p string) bool {
	return strings.HasPrefix(strings.ToLower(p), "smb://")
}

func smbURLToUNC(u string) string {
	// smb://[user[:pass]@]host/share/seg1/seg2 -> \\host\share\seg1\seg2 (drop creds)
	s := strings.TrimPrefix(u, "SMB://")
	s = strings.TrimPrefix(strings.ToLower(u), "smb://")
	// find authority and path
	hostAndPath := s
	// Strip optional creds
	if at := strings.Index(hostAndPath, "@"); at >= 0 {
		hostAndPath = hostAndPath[at+1:]
	}
	parts := strings.Split(hostAndPath, "/")
	if len(parts) == 0 || parts[0] == "" {
		return `\\` // invalid, best effort
	}
	host := parts[0]
	rest := parts[1:]
	b := strings.Builder{}
	b.WriteString(`\\`)
	b.WriteString(host)
	if len(rest) > 0 {
		b.WriteString(`\\`)
		b.WriteString(strings.Join(rest, `\\`))
	}
	return b.String()
}

func parseUNC(unc string) Parsed {
	// Accept both \\host\share\... and \\?\UNC\host\share\...
	raw := unc
	u := unc
	if strings.HasPrefix(u, `\\\\?\\UNC\\`) {
		u = strings.TrimPrefix(u, `\\\\?\\UNC\\`)
	} else if strings.HasPrefix(u, `\\`) {
		u = strings.TrimPrefix(u, `\\`)
	}
	seg := strings.Split(u, `\\`)
	host := ""
	share := ""
	segments := []string{}
	if len(seg) > 0 {
		host = seg[0]
	}
	if len(seg) > 1 {
		share = seg[1]
	}
	if len(seg) > 2 {
		segments = seg[2:]
	}
	display := "smb://" + path.Join(host, share)
	if len(segments) > 0 {
		display += "/" + path.Join(segments...)
	}
	return Parsed{
		Scheme:   SchemeSMB,
		Host:     host,
		Share:    share,
		Segments: segments,
		Raw:      raw,
		Display:  display,
		Native:   unc,
	}
}

func canonicalizeSMB(url string) string {
	// cheap canonicalization for display purposes
	s := strings.TrimSpace(url)
	s = strings.ReplaceAll(s, "\\", "/")
	if !strings.HasPrefix(strings.ToLower(s), "smb://") {
		s = "smb://" + strings.TrimPrefix(s, "//")
	}
	return s
}

func errUnsupportedSMB() error {
	return errors.New("smb paths are not supported on this platform yet")
}
