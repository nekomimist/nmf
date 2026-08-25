package fileinfo

import (
	"path"
	"path/filepath"
	"strings"
)

// IsSMBDisplay reports whether the path is a canonical smb display path (smb://...).
func IsSMBDisplay(p string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), "smb://")
}

// JoinPath joins base and name for display paths.
// - For smb:// display paths, it joins using forward slashes.
// - Otherwise it uses filepath.Join.
func JoinPath(base, name string) string {
	if IsArchivePath(base) {
		return archiveJoinPath(base, name)
	}
	if IsSMBDisplay(base) {
		b := strings.TrimRight(base, "/")
		return b + "/" + name
	}
	return filepath.Join(base, name)
}

// ParentPath returns the parent directory for a path.
//   - For smb:// display paths, it trims one segment after the share.
//     Root (smb://host/share) returns itself.
//   - Otherwise it uses filepath.Dir.
func ParentPath(p string) string {
	if IsArchivePath(p) {
		return archiveParentPath(p)
	}
	if !IsSMBDisplay(p) {
		return filepath.Dir(p)
	}
	rest := strings.TrimPrefix(p, "smb://")
	parts := strings.Split(rest, "/")
	if len(parts) <= 2 { // smb://host/share => root, no parent
		return p
	}
	parent := "smb://" + strings.Join(parts[:len(parts)-1], "/")
	return parent
}

// NavigationRootPath returns the root of the browsing namespace containing p.
// Local paths use their filesystem volume root, SMB paths use their share
// root, and archive paths use the root inside the archive.
func NavigationRootPath(p string) (string, error) {
	canonical, parsed, err := CanonicalDisplayPath(p)
	if err != nil {
		return "", err
	}

	switch parsed.Scheme {
	case SchemeArchive:
		return ArchiveRootPath(parsed.Archive), nil
	case SchemeSMB:
		return smbDisplayPath(parsed.Host, parsed.Share, nil), nil
	default:
		return filepath.VolumeName(canonical) + string(filepath.Separator), nil
	}
}

// BaseName returns the last path segment analogous to filepath.Base.
// For smb:// paths, it uses URL-style segments.
func BaseName(p string) string {
	if IsArchivePath(p) {
		return archiveBaseName(p)
	}
	if !IsSMBDisplay(p) {
		return filepath.Base(p)
	}
	rest := strings.TrimSuffix(strings.TrimPrefix(p, "smb://"), "/")
	_, last := path.Split(rest)
	return last
}
