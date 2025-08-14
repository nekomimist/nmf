package constants

import "time"

// Application constants
const (
	ApplicationName  = "nmf"
	ApplicationTitle = "File Manager"
)

// UI constants
const (
	// Window dimensions
	DefaultWindowWidth  = 800
	DefaultWindowHeight = 600

	// Tree dialog dimensions
	TreeDialogWidth  = 550
	TreeDialogHeight = 500
	TreeWidth        = 500
	TreeHeight       = 400

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

// Status colors (RGBA values)
var (
	// Status background colors
	StatusAddedColor    = [4]uint8{0, 200, 0, 80}     // Semi-transparent green
	StatusDeletedColor  = [4]uint8{128, 128, 128, 60} // Semi-transparent gray
	StatusModifiedColor = [4]uint8{255, 200, 0, 80}   // Semi-transparent orange

	// Selection background color
	SelectionBackgroundColor = [4]uint8{100, 150, 200, 100}

	// Default file type colors
	DefaultRegularFileColor = [4]uint8{220, 220, 220, 255} // Light gray
	DefaultDirectoryColor   = [4]uint8{135, 206, 250, 255} // Light sky blue
	DefaultSymlinkColor     = [4]uint8{255, 165, 0, 255}   // Orange
	DefaultHiddenFileColor  = [4]uint8{105, 105, 105, 255} // Dim gray

	// Default cursor color
	DefaultCursorColor = [4]uint8{255, 255, 255, 255} // White
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
