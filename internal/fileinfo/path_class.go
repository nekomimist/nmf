package fileinfo

// PathClass describes storage properties that affect whether maintenance
// cleanup should treat a missing directory as safe to remove from runtime state.
type PathClass struct {
	Network   bool
	Removable bool
}

// ClassifyPath reports whether a path is known to be network-backed or
// removable. Unknown local paths return a zero PathClass.
func ClassifyPath(p string) (PathClass, error) {
	_, parsed, err := CanonicalDisplayPath(p)
	if err != nil {
		return PathClass{}, err
	}
	if parsed.Scheme == SchemeSMB {
		return PathClass{Network: true}, nil
	}

	native := parsed.Native
	if native == "" {
		native = parsed.Display
	}
	if native == "" {
		native = p
	}
	return classifyLocalPath(native)
}

func isNetworkFilesystemType(fsType string) bool {
	switch fsType {
	case "9p", "afs", "cifs", "davfs", "fuse.sshfs", "ncpfs", "nfs", "nfs4", "smb3", "smbfs", "sshfs":
		return true
	default:
		return false
	}
}
