package fileinfo

import "testing"

func TestParseSMBURLBasic(t *testing.T) {
	host, share, segs, user, pass, domain := parseSMBURL("smb://server/share/a/b")
	if host != "server" || share != "share" {
		t.Fatalf("host/share mismatch: %s/%s", host, share)
	}
	if len(segs) != 2 || segs[0] != "a" || segs[1] != "b" {
		t.Fatalf("segments mismatch: %#v", segs)
	}
	if user != "" || pass != "" || domain != "" {
		t.Fatalf("unexpected creds parsed: %q %q %q", user, pass, domain)
	}
}

func TestParseSMBURLWithCreds(t *testing.T) {
	// user:pass
	_, _, _, user, pass, domain := parseSMBURL("smb://alice:secret@host/share")
	if user != "alice" || pass != "secret" || domain != "" {
		t.Fatalf("cred mismatch: %q %q %q", user, pass, domain)
	}
	// domain;user
	_, _, _, user, pass, domain = parseSMBURL("smb://corp;bob:pwd@host/share")
	if user != "bob" || pass != "pwd" || domain != "corp" {
		t.Fatalf("domain;user mismatch: %q %q %q", user, pass, domain)
	}
	// domain\user
	_, _, _, user, pass, domain = parseSMBURL("smb://corp\\carol:pw@host/share")
	if user != "carol" || pass != "pw" || domain != "corp" {
		t.Fatalf("domain\\user mismatch: %q %q %q", user, pass, domain)
	}
}

func TestParseSMBURLAcceptsCaseInsensitiveScheme(t *testing.T) {
	host, share, segments, user, pass, _ := parseSMBURL("SMB://alice:secret@SERVER/share/dir")
	if host != "server" || share != "share" || len(segments) != 1 || segments[0] != "dir" {
		t.Fatalf("parsed path = host=%q share=%q segments=%#v", host, share, segments)
	}
	if user != "alice" || pass != "secret" {
		t.Fatalf("parsed credentials = user=%q pass=%q", user, pass)
	}
}

func TestSMBResolversRejectParentSegments(t *testing.T) {
	inputs := []string{
		"smb://server/share/../etc",
		"smb://server/share/dir/../../etc",
		"smb://server/share\\..\\etc",
		"//server/share/../etc",
		"SMB://server/share/../etc",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			if _, _, err := ResolveRead(input); err == nil {
				t.Fatalf("ResolveRead(%q) should reject parent segments", input)
			}
			if _, _, err := CanonicalDisplayPath(input); err == nil {
				t.Fatalf("CanonicalDisplayPath(%q) should reject parent segments", input)
			}
		})
	}
	if _, _, err := CanonicalDisplayPath(`\\server\share\..\etc`); err == nil {
		t.Fatal("CanonicalDisplayPath should reject parent segments in UNC input")
	}
}

func TestNormalizeSMBPathComponents(t *testing.T) {
	segments, err := normalizeSMBPathComponents("server", "share", []string{"", "dir", "", "file"})
	if err != nil {
		t.Fatalf("normalizeSMBPathComponents returned error: %v", err)
	}
	if len(segments) != 2 || segments[0] != "dir" || segments[1] != "file" {
		t.Fatalf("normalized segments = %#v, want [dir file]", segments)
	}

	for _, tc := range []struct {
		host     string
		share    string
		segments []string
	}{
		{host: "..", share: "share"},
		{host: "server", share: ".."},
		{host: "server", share: "share", segments: []string{"."}},
		{host: "server", share: "share", segments: []string{".."}},
	} {
		if _, err := normalizeSMBPathComponents(tc.host, tc.share, tc.segments); err == nil {
			t.Fatalf("normalizeSMBPathComponents(%q, %q, %#v) should fail", tc.host, tc.share, tc.segments)
		}
	}
}

func TestUNCConversionRoundTripDisplay(t *testing.T) {
	// to UNC and back to display via parseUNC
	unc := smbURLToUNC("smb://srv/share/dir/file")
	if unc != "\\\\srv\\share\\dir\\file" {
		t.Fatalf("UNC conversion failed: %q", unc)
	}
	p := parseUNC(unc)
	if p.Display != "smb://srv/share/dir/file" {
		t.Fatalf("display mismatch: %q", p.Display)
	}
}

func TestCanonicalDisplayPathNormalizesSMBForms(t *testing.T) {
	tests := []string{
		`\\wsl$\Ubuntu\home\neko`,
		`\\wsl.localhost\Ubuntu\home\neko`,
		"//wsl$/Ubuntu/home/neko",
		"//wsl.localhost/Ubuntu/home/neko",
		"smb://wsl$/Ubuntu/home/neko",
	}

	for _, input := range tests {
		got, parsed, err := CanonicalDisplayPath(input)
		if err != nil {
			t.Fatalf("CanonicalDisplayPath(%q) error: %v", input, err)
		}
		if parsed.Scheme != SchemeSMB {
			t.Fatalf("CanonicalDisplayPath(%q) scheme = %q, want smb", input, parsed.Scheme)
		}
		if got != "smb://wsl.localhost/Ubuntu/home/neko" {
			t.Fatalf("CanonicalDisplayPath(%q) = %q, want canonical WSL path", input, got)
		}
	}
}

func TestCanonicalDisplayPathNormalizesUNCArchivePath(t *testing.T) {
	input := `\\wsl.localhost\Ubuntu\home\neko\src\lookup.tar.gz!/`
	want := "smb://wsl.localhost/Ubuntu/home/neko/src/lookup.tar.gz!/"

	got, parsed, err := CanonicalDisplayPath(input)
	if err != nil {
		t.Fatalf("CanonicalDisplayPath(%q) error: %v", input, err)
	}
	if parsed.Scheme != SchemeArchive {
		t.Fatalf("CanonicalDisplayPath(%q) scheme = %q, want archive", input, parsed.Scheme)
	}
	if got != want {
		t.Fatalf("CanonicalDisplayPath(%q) = %q, want %q", input, got, want)
	}
	if parsed.Archive != "smb://wsl.localhost/Ubuntu/home/neko/src/lookup.tar.gz" || parsed.Inner != "." {
		t.Fatalf("unexpected parsed archive path: %+v", parsed)
	}
}

func TestParseUNCWithSingleBackslashSeparators(t *testing.T) {
	p := parseUNC("\\\\naja.local\\neko\\a")
	if p.Host != "naja.local" || p.Share != "neko" {
		t.Fatalf("host/share mismatch: %q/%q", p.Host, p.Share)
	}
	if p.Display != "smb://naja.local/neko/a" {
		t.Fatalf("display mismatch: %q", p.Display)
	}
	if parent := ParentPath(p.Display); parent != "smb://naja.local/neko" {
		t.Fatalf("parent mismatch: %q", parent)
	}
}

func TestParseExtendedUNC(t *testing.T) {
	p := parseUNC("\\\\?\\UNC\\server\\share\\dir\\file")
	if p.Host != "server" || p.Share != "share" {
		t.Fatalf("host/share mismatch: %q/%q", p.Host, p.Share)
	}
	if p.Display != "smb://server/share/dir/file" {
		t.Fatalf("display mismatch: %q", p.Display)
	}
}

func TestCanonicalizeSMB(t *testing.T) {
	got := canonicalizeSMB("//SERVER/share\\a\\b")
	if got != "smb://server/share/a/b" {
		t.Fatalf("canonicalize got %q", got)
	}
}

func TestMountInfoHelpers(t *testing.T) {
	fs, src, mp, super, opts, ok := parseMountInfo("36 23 0:27 / /mnt/share rw,relatime - cifs //server/share rw,sec=ntlm,unc=\\\\server\\share")
	if !ok || fs != "cifs" || src != "//server/share" || mp != "/mnt/share" {
		t.Fatalf("parseMountInfo mismatch: fs=%q src=%q mp=%q ok=%v", fs, src, mp, ok)
	}
	if findUNCOption(super) == "" && findUNCOption(opts) == "" {
		t.Fatalf("findUNCOption failed to detect unc")
	}
	h, s := parseSourceUNC(src)
	if h != "server" || s != "share" {
		t.Fatalf("parseSourceUNC mismatch: %q %q", h, s)
	}
	// backslash form
	h2, s2 := parseBackslashUNC("\\\\server\\share")
	if h2 != "server" || s2 != "share" {
		t.Fatalf("parseBackslashUNC mismatch: %q %q", h2, s2)
	}
}
