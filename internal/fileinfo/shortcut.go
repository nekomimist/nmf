package fileinfo

import (
	"context"
	"errors"
	"fmt"
)

// ShortcutNavigationStage identifies which part of Windows shortcut
// resolution failed. Callers use it to distinguish a shortcut that nmf could
// not parse from a target that was parsed but could not be inspected.
type ShortcutNavigationStage string

const (
	ShortcutNavigationRead   ShortcutNavigationStage = "read"
	ShortcutNavigationTarget ShortcutNavigationStage = "target"
)

// ShortcutNavigationError reports a staged Windows shortcut resolution
// failure while preserving the provider or operating-system error.
type ShortcutNavigationError struct {
	Stage  ShortcutNavigationStage
	Path   string
	Target string
	Err    error
}

func (e *ShortcutNavigationError) Error() string {
	if e == nil {
		return "shortcut navigation failed"
	}
	switch e.Stage {
	case ShortcutNavigationRead:
		return fmt.Sprintf("read shortcut %q: %v", e.Path, e.Err)
	case ShortcutNavigationTarget:
		return fmt.Sprintf("inspect shortcut target %q: %v", e.Target, e.Err)
	default:
		return fmt.Sprintf("resolve shortcut %q: %v", e.Path, e.Err)
	}
}

func (e *ShortcutNavigationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsShortcutNavigationReadError reports whether err came from reading the
// shortcut itself, before nmf obtained a target path. Such failures may still
// be delegated to the platform's default shortcut handler.
func IsShortcutNavigationReadError(err error) bool {
	var shortcutErr *ShortcutNavigationError
	return errors.As(err, &shortcutErr) && shortcutErr.Stage == ShortcutNavigationRead
}

// ResolveShortcutNavigationDir resolves a Windows shortcut to the directory
// nmf should navigate to when the regular open command is used.
func ResolveShortcutNavigationDir(p string) (string, bool, error) {
	return ResolveShortcutNavigationDirContext(context.Background(), p)
}
