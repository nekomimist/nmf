package shellmenu

import "unicode/utf16"

const legacyShellPathMaxChars = 260

// exceedsLegacyShellPathLimit reports whether a path, including the required
// terminating NUL, is too long for the legacy Shell path parser.
func exceedsLegacyShellPathLimit(path string) bool {
	return len(utf16.Encode([]rune(path))) >= legacyShellPathMaxChars
}
