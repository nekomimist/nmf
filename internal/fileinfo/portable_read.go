package fileinfo

import "os"

// ReadDirPortable resolves the input path to a suitable provider and performs ReadDir.
func ReadDirPortable(p string) ([]os.DirEntry, error) {
	vfs, parsed, err := ResolveRead(p)
	if err != nil {
		return nil, err
	}
	native := parsed.Native
	if native == "" {
		native = p
	}
	return vfs.ReadDir(native)
}
