//go:build !windows

package fileinfo

import (
	"errors"
	"os/exec"
)

// OpenWithDefaultApp opens the given path with the system default application.
// On Unix-like systems, try xdg-open (with basic fallbacks if unavailable).
func OpenWithDefaultApp(p string) error {
	// Prefer local mount path for smb:// if available so non-GVFS apps still work.
	target := p
	if _, parsed, err := ResolveRead(p); err == nil {
		if parsed.Scheme == SchemeSMB && parsed.Provider == "local" && parsed.Native != "" {
			target = parsed.Native
		}
	}
	// Try common openers; xdg-open is the standard on most desktops.
	candidates := [][]string{
		{"xdg-open", target},
		{"gio", "open", target},
		{"gvfs-open", target},
		{"gnome-open", target},
		{"kde-open", target},
	}
	var lastErr error
	for _, args := range candidates {
		// Ensure the binary exists before trying
		if path, lookErr := exec.LookPath(args[0]); lookErr == nil {
			cmd := exec.Command(path, args[1:]...)
			if err := cmd.Start(); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no suitable opener found (xdg-open/gio/gnome-open)")
	}
	return lastErr
}
