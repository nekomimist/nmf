package shellmenu

import "errors"

// ErrPathTooLong indicates that the Windows Shell could not parse a path that
// exceeds its legacy path-length limit.
var ErrPathTooLong = errors.New("path is too long for the Explorer context menu")
