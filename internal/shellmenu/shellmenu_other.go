//go:build !windows

package shellmenu

import "errors"

// ErrUnsupported indicates that the shell context menu is unavailable.
var ErrUnsupported = errors.New("shell context menu is unsupported on this platform")

// Show opens a platform-native shell context menu.
func Show(_ uintptr, _ []string) error {
	return ErrUnsupported
}
