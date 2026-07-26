package fileinfo

import (
	"errors"
	"io/fs"
)

// IsNotExist reports whether err means that a path does not exist, including
// provider-native errors that do not implement errors.Is(fs.ErrNotExist).
//
// errors.Is rather than os.IsNotExist: the latter only unwraps *PathError,
// *LinkError and *SyscallError one level, so it silently misses an error a
// caller wrapped with %w. Every producer feeding this today returns an
// unwrapped *PathError, but "target is missing" turning into "unexpected
// error" is a bad way to find that out later.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || isProviderNotExist(err)
}
