package main

import (
	"time"

	"nmf/internal/browser"
	"nmf/internal/keymanager"
)

const (
	directoryCacheTTL        = 2 * time.Minute
	directoryCacheMaxEntries = 8
)

type directoryListingState uint8

const (
	directoryListingFresh directoryListingState = iota
	directoryListingCachedRefreshing
	directoryListingCachedStale
)

func (fm *FileManager) ensureDirectoryCache() *browser.DirectoryCache {
	if fm.directoryCache == nil {
		fm.directoryCache = browser.NewDirectoryCache(directoryCacheTTL, directoryCacheMaxEntries)
	}
	return fm.directoryCache
}

func (fm *FileManager) setDirectoryListingState(state directoryListingState) {
	if fm == nil || fm.directoryListingState == state {
		return
	}
	fm.directoryListingState = state
	fm.updateStatusBar()
}

func (fm *FileManager) directoryListingNavigationOnly() bool {
	return fm != nil && fm.directoryListingState != directoryListingFresh
}

func (fm *FileManager) directoryListingStateAfterCanceledRefresh() directoryListingState {
	if fm == nil {
		return directoryListingFresh
	}
	if fm.directoryListingState == directoryListingCachedRefreshing {
		return directoryListingCachedStale
	}
	return fm.directoryListingState
}

// mainScreenCommandAllowed keeps a cached listing useful for moving around
// while preventing it from becoming authority for marks, file opening, or
// filesystem mutation. The policy receives semantic command IDs, so custom
// key bindings cannot bypass it.
func (fm *FileManager) mainScreenCommandAllowed(commandID string) bool {
	if !fm.directoryListingNavigationOnly() {
		return true
	}
	switch commandID {
	case keymanager.CommandCursorUp,
		keymanager.CommandCursorDown,
		keymanager.CommandCursorPageUp,
		keymanager.CommandCursorPageDown,
		keymanager.CommandCursorFirst,
		keymanager.CommandCursorLast,
		keymanager.CommandOpen,
		keymanager.CommandParentDirectory,
		keymanager.CommandRefresh,
		keymanager.CommandHome,
		keymanager.CommandQuit,
		keymanager.CommandNoop:
		return true
	default:
		return false
	}
}

func (fm *FileManager) logCachedListingBlocked(action string) {
	if fm == nil || !fm.directoryListingNavigationOnly() {
		return
	}
	debugPrint("FileManager: Cached listing blocked action=%s path=%s state=%d", action, fm.GetCurrentPath(), fm.directoryListingState)
}
