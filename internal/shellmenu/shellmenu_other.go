//go:build !windows

package shellmenu

import "errors"

// ErrUnsupported indicates that the shell context menu is unavailable.
var ErrUnsupported = errors.New("shell context menu is unsupported on this platform")

// Debugf receives optional debug messages from this package.
var Debugf func(format string, args ...interface{})

// Show opens a platform-native shell context menu.
func Show(_ uintptr, _ []string) error {
	return ErrUnsupported
}

// ShowAtClientPosition opens a platform-native shell context menu at a window
// client coordinate.
func ShowAtClientPosition(_ uintptr, _ []string, _, _ int) error {
	return ErrUnsupported
}

// StartFileDrag starts a platform-native file drag operation.
func StartFileDrag(_ uintptr, _ []string) error {
	return ErrUnsupported
}
