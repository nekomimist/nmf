package fileinfo

import (
	"bufio"
	"errors"
	"os"
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
	User     string
	Password string
	Domain   string
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

	// Non-Windows: support smb:// (and //) via existing CIFS mounts if present.
	// If no mount found, fall back to SMBFS provider (direct SMB access).
	if isSMBURL(raw) || strings.HasPrefix(raw, "//") {
		host, share, segs, user, pass, domain := parseSMBURL(raw)
		if host != "" && share != "" {
			if mp, ok := findSMBMount(host, share); ok {
				native := mp
				if len(segs) > 0 {
					native = "/" + path.Join(strings.TrimPrefix(mp, "/"), path.Join(segs...))
				}
				disp := canonicalizeSMB("smb://" + path.Join(host, share))
				if len(segs) > 0 {
					disp += "/" + path.Join(segs...)
				}
				return LocalFS{}, Parsed{
					Scheme:   SchemeSMB,
					Host:     host,
					Share:    share,
					Segments: segs,
					Raw:      input,
					Display:  disp,
					Native:   native,
					Provider: "local",
					User:     user,
					Password: pass,
					Domain:   domain,
				}, nil
			}
			// Fallback: provide SMBFS with native path relative to share
			native := "/"
			if len(segs) > 0 {
				native = "/" + path.Join(segs...)
			}
			disp := canonicalizeSMB("smb://" + path.Join(host, share))
			if len(segs) > 0 {
				disp += "/" + path.Join(segs...)
			}
			var vfs VFS
			if user != "" || pass != "" || domain != "" {
				vfs = newSMBProvider(host, share, &Credentials{Domain: domain, Username: user, Password: pass})
			} else {
				vfs = newSMBProvider(host, share, nil)
			}
			return vfs, Parsed{
				Scheme:   SchemeSMB,
				Host:     host,
				Share:    share,
				Segments: segs,
				Raw:      input,
				Display:  disp,
				Native:   native,
				Provider: "smb",
				User:     user,
				Password: pass,
				Domain:   domain,
			}, nil
		}
		return nil, Parsed{Raw: input, Scheme: SchemeSMB, Display: canonicalizeSMB(raw), Provider: "smb"}, errUnsupportedSMB()
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
	s := strings.TrimPrefix(strings.ToLower(u), "smb://")
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

// parseSMBURL extracts host, share, segments from an smb-like path.
// Accepts forms: smb://[user[:pass]@]host/share/..., //host/share/...
func parseSMBURL(u string) (host, share string, segments []string, user, pass, domain string) {
	s := strings.TrimSpace(u)
	if strings.HasPrefix(s, "//") && !strings.HasPrefix(strings.ToLower(s), "smb://") {
		s = "smb:" + s // normalize to smb://
	}
	if !isSMBURL(s) {
		return "", "", nil, "", "", ""
	}
	t := strings.TrimPrefix(s, "smb://")
	// Extract and strip creds
	if at := strings.Index(t, "@"); at >= 0 {
		cred := t[:at]
		t = t[at+1:]
		// Split password part
		if colon := strings.Index(cred, ":"); colon >= 0 {
			pass = cred[colon+1:]
			cred = cred[:colon]
		}
		// Detect domain separator
		if semi := strings.Index(cred, ";"); semi >= 0 {
			domain = cred[:semi]
			user = cred[semi+1:]
		} else if bs := strings.Index(cred, "\\"); bs >= 0 {
			domain = cred[:bs]
			user = cred[bs+1:]
		} else {
			user = cred
		}
	}
	parts := strings.Split(t, "/")
	if len(parts) < 2 {
		return "", "", nil, "", "", ""
	}
	host = parts[0]
	share = parts[1]
	if len(parts) > 2 {
		segments = parts[2:]
	}
	return
}

// findSMBMount attempts to find a mounted CIFS/SMB mount matching host/share.
// It scans /proc/self/mountinfo (Linux) and matches either mount source (//host/share)
// or unc=\\\host\share in options.
func findSMBMount(host, share string) (mountPoint string, ok bool) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", false
	}
	defer f.Close()
	targetHost := strings.ToLower(host)
	targetShare := strings.ToLower(share)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fsType, src, mp, superOpts, opts, parsed := parseMountInfo(line)
		if !parsed {
			continue
		}
		lfs := strings.ToLower(fsType)
		if !(lfs == "cifs" || strings.Contains(lfs, "smb")) {
			continue
		}
		// Try source first: expected form //host/share
		shost, sshare := parseSourceUNC(src)
		if shost != "" && strings.EqualFold(shost, targetHost) && strings.EqualFold(sshare, targetShare) {
			return mp, true
		}
		// Fallback: look for unc=\\host\share in options
		unc := findUNCOption(superOpts)
		if unc == "" {
			unc = findUNCOption(opts)
		}
		if unc != "" {
			shost, sshare = parseBackslashUNC(unc)
			if shost != "" && strings.EqualFold(shost, targetHost) && strings.EqualFold(sshare, targetShare) {
				return mp, true
			}
		}
	}
	return "", false
}

// parseMountInfo extracts minimal fields from a mountinfo line.
func parseMountInfo(line string) (fsType, source, mountPoint, superOpts, opts string, ok bool) {
	// split at " - " separator
	parts := strings.SplitN(line, " - ", 2)
	if len(parts) != 2 {
		return
	}
	left := strings.Fields(parts[0])
	right := strings.Fields(parts[1])
	// mountinfo may have zero optional fields; accept 6+ tokens on the left side.
	if len(left) < 6 || len(right) < 3 {
		return
	}
	// left fields: ... root mountPoint opts
	mountPoint = decodeMountPoint(left[4])
	opts = strings.Join(left[5:], " ")
	// right fields: fstype source superOpts
	fsType = right[0]
	source = right[1]
	superOpts = strings.Join(right[2:], " ")
	ok = true
	return
}

// decodeMountPoint converts mountinfo escape sequences (e.g., \040 -> space).
func decodeMountPoint(s string) string {
	// minimal decoding: replace common escapes
	s = strings.ReplaceAll(s, "\\040", " ")
	s = strings.ReplaceAll(s, "\\134", "\\")
	return s
}

func parseSourceUNC(src string) (host, share string) {
	if strings.HasPrefix(src, "//") {
		rest := strings.TrimPrefix(src, "//")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1]
		}
	}
	return "", ""
}

func findUNCOption(opts string) string {
	// look for unc=\\host\share in comma-separated options
	for _, part := range strings.Split(opts, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && strings.ToLower(kv[0]) == "unc" {
			return kv[1]
		}
	}
	return ""
}

func parseBackslashUNC(unc string) (host, share string) {
	s := strings.TrimPrefix(unc, `\\`)
	// Split on single backslash character between host and share
	parts := strings.Split(s, "\\")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
