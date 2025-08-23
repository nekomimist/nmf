package fileinfo

import (
	"os"
)

// StatPortable resolves the input path to a suitable provider and performs Stat.
// For SMB providers, it uses the provider-native path from the resolver.
func StatPortable(p string) (os.FileInfo, error) {
	vfs, parsed, err := ResolveRead(p)
	if err != nil {
		return nil, err
	}
	native := parsed.Native
	if native == "" {
		native = p
	}
	return vfs.Stat(native)
}
