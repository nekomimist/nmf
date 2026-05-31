package constants

import "time"

// Application constants
const (
	ApplicationName  = "nmf"
	ApplicationTitle = "File Manager"
)

// UI constants
const (
	// Icon sizes
	DefaultIconSize = 16

	// Cursor settings
	DefaultCursorThickness = 2
	MinCursorThickness     = 1
	MinPadding             = 2

	// Item spacing
	MaxItemSpacingForHiddenSeparators = 2

	// Keyboard navigation
	FastNavigationStep = 20
)

// Directory watcher constants
const (
	WatcherInterval   = 2 * time.Second
	WatcherBufferSize = 10
)

// File size constants
const (
	FileSizeUnit  = 1024
	FileSizeUnits = "KMGTPE"
)

// Theme constants
const (
	DefaultFontSize  = 14
	DarkThemeDefault = true
)

// File system constants
const (
	RootPath            = "/"
	ParentDirectoryName = ".."
	DeletedFilePrefix   = "⊠ "
)

// Configuration constants
const (
	ConfigFileName          = "config.json"
	DefaultSortBy           = "name"
	DefaultSortOrder        = "asc"
	DefaultDirectoriesFirst = true
	DefaultShowHiddenFiles  = false
	DefaultCursorType       = "underline"
)

// Tree dialog constants
const (
	RootModeOptionText   = "Root /"
	ParentModeOptionText = "Parent Dir"
)
