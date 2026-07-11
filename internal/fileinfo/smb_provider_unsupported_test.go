//go:build !linux && !windows

package fileinfo

import "testing"

func TestResolveReadRejectsDirectSMBOnUnsupportedPlatform(t *testing.T) {
	vfs, parsed, err := ResolveRead("smb://server/share/etc")
	if err == nil {
		t.Fatal("ResolveRead should reject direct SMB on this platform")
	}
	if vfs != nil {
		t.Fatalf("ResolveRead VFS = %T, want nil", vfs)
	}
	if parsed.Scheme != SchemeSMB || parsed.Provider != "smb" {
		t.Fatalf("ResolveRead parsed = %+v, want direct SMB metadata", parsed)
	}
}
