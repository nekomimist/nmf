package fileinfo

import "os"

// IsNotExist reports whether err means that a path does not exist, including
// provider-native errors that do not implement errors.Is(fs.ErrNotExist).
func IsNotExist(err error) bool {
	return os.IsNotExist(err) || isProviderNotExist(err)
}
