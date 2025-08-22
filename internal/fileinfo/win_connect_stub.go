//go:build !windows
// +build !windows

package fileinfo

import "errors"

func isWinAccessError(err error) bool                       { return false }
func ensureWindowsConnection(p Parsed, native string) error { return errors.New("unsupported") }
func IsWindowsCredentialConflict(err error) bool            { return false }
