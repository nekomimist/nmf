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

func TestUNCConversionRoundTripDisplay(t *testing.T) {
	// to UNC and back to display via parseUNC
	unc := smbURLToUNC("smb://srv/share/dir/file")
	if unc == "\\" || len(unc) == 0 {
		t.Fatalf("UNC conversion failed: %q", unc)
	}
	p := parseUNC(unc)
	if p.Display != "smb://srv/share/dir/file" {
		t.Fatalf("display mismatch: %q", p.Display)
	}
}

func TestCanonicalizeSMB(t *testing.T) {
	got := canonicalizeSMB("//SERVER/share\\a\\b")
	if got != "smb://SERVER/share/a/b" {
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
