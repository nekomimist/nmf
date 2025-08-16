package theme

import (
	"image/color"
	"io/ioutil"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"nmf/internal/config"
)

// CustomColorScheme defines custom colors for light and dark themes
type CustomColorScheme struct {
	// Status colors
	StatusAdded    color.RGBA
	StatusDeleted  color.RGBA
	StatusModified color.RGBA

	// UI colors
	SelectionBackground color.RGBA
	SearchOverlay       color.RGBA

	// File type colors
	FileRegular   color.RGBA
	FileDirectory color.RGBA
	FileSymlink   color.RGBA
	FileHidden    color.RGBA

	// Cursor color
	Cursor color.RGBA
}

var (
	LightColorScheme = &CustomColorScheme{
		StatusAdded:         color.RGBA{0, 150, 0, 80},     // Slightly darker green for light theme
		StatusDeleted:       color.RGBA{100, 100, 100, 60}, // Slightly darker gray for light theme
		StatusModified:      color.RGBA{200, 150, 0, 80},   // Slightly darker orange for light theme
		SelectionBackground: color.RGBA{70, 120, 170, 100}, // Slightly darker blue for light theme
		SearchOverlay:       color.RGBA{40, 40, 40, 240},   // Dark overlay for light theme
		// File type colors for light theme
		FileRegular:   color.RGBA{60, 60, 60, 255},    // Dark gray text for light theme
		FileDirectory: color.RGBA{30, 100, 200, 255},  // Darker blue for light theme
		FileSymlink:   color.RGBA{200, 100, 0, 255},   // Darker orange for light theme
		FileHidden:    color.RGBA{120, 120, 120, 255}, // Medium gray for light theme
		// Cursor color for light theme
		Cursor: color.RGBA{0, 0, 0, 255}, // Black cursor for light theme
	}

	DarkColorScheme = &CustomColorScheme{
		StatusAdded:         color.RGBA{0, 200, 0, 80}, // Current values for dark theme
		StatusDeleted:       color.RGBA{128, 128, 128, 60},
		StatusModified:      color.RGBA{255, 200, 0, 80},
		SelectionBackground: color.RGBA{100, 150, 200, 100},
		SearchOverlay:       color.RGBA{220, 220, 220, 240}, // Light overlay for dark theme
		// File type colors for dark theme (current config defaults)
		FileRegular:   color.RGBA{220, 220, 220, 255}, // Light gray text for dark theme
		FileDirectory: color.RGBA{135, 206, 250, 255}, // Light sky blue for dark theme
		FileSymlink:   color.RGBA{255, 165, 0, 255},   // Orange for dark theme
		FileHidden:    color.RGBA{105, 105, 105, 255}, // Dim gray for dark theme
		// Cursor color for dark theme (current config default)
		Cursor: color.RGBA{255, 255, 255, 255}, // White cursor for dark theme
	}
)

// CustomTheme implements fyne.Theme with configurable font settings
type CustomTheme struct {
	config     *config.Config
	customFont fyne.Resource
	debugPrint func(format string, args ...interface{})
}

// NewCustomTheme creates a new custom theme with the given configuration
func NewCustomTheme(config *config.Config, debugPrint func(format string, args ...interface{})) *CustomTheme {
	customTheme := &CustomTheme{
		config:     config,
		debugPrint: debugPrint,
	}

	// Load custom font if specified
	if config.Theme.FontPath != "" {
		customTheme.loadCustomFont()
	}

	return customTheme
}

// loadCustomFont loads a custom font from the specified path
func (t *CustomTheme) loadCustomFont() {
	fontPath := t.config.Theme.FontPath

	// Check if font file exists
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.debugPrint("Custom font file not found: %s", fontPath)
		return
	}

	// Read font file
	fontData, err := ioutil.ReadFile(fontPath)
	if err != nil {
		t.debugPrint("Error reading font file %s: %v", fontPath, err)
		return
	}

	// Create font resource
	t.customFont = fyne.NewStaticResource(filepath.Base(fontPath), fontData)
	t.debugPrint("Loaded custom font: %s", fontPath)
}

// Color methods from default theme
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.config.Theme.Dark {
		return theme.DarkTheme().Color(name, variant)
	}
	return theme.LightTheme().Color(name, variant)
}

// Icon methods from default theme
func (t *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	if t.config.Theme.Dark {
		return theme.DarkTheme().Icon(name)
	}
	return theme.LightTheme().Icon(name)
}

// Font method with custom font support
func (t *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	// Return custom font if loaded and available
	if t.customFont != nil {
		return t.customFont
	}

	if t.config.Theme.Dark {
		return theme.DarkTheme().Font(style)
	}
	return theme.LightTheme().Font(style)
}

// Size method with custom font size and spacing support
func (t *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText && t.config.Theme.FontSize > 0 {
		return float32(t.config.Theme.FontSize)
	}

	// Custom item spacing support with icon consideration
	if t.config.UI.ItemSpacing > 0 {
		switch name {
		case theme.SizeNamePadding:
			// Ensure minimum padding for icons but allow small spacing
			minPadding := float32(2) // Very minimal padding
			requested := float32(t.config.UI.ItemSpacing)
			if requested < minPadding {
				return minPadding
			}
			return requested
		case theme.SizeNameInnerPadding:
			return 0
		}
	}

	if t.config.Theme.Dark {
		return theme.DarkTheme().Size(name)
	}
	return theme.LightTheme().Size(name)
}

// GetCustomColor returns custom colors based on the current theme (light/dark)
func (t *CustomTheme) GetCustomColor(colorType string) color.RGBA {
	scheme := LightColorScheme
	if t.config.Theme.Dark {
		scheme = DarkColorScheme
	}

	switch colorType {
	case "statusAdded":
		return scheme.StatusAdded
	case "statusDeleted":
		return scheme.StatusDeleted
	case "statusModified":
		return scheme.StatusModified
	case "selectionBackground":
		return scheme.SelectionBackground
	case "searchOverlay":
		return scheme.SearchOverlay
	case "fileRegular":
		return scheme.FileRegular
	case "fileDirectory":
		return scheme.FileDirectory
	case "fileSymlink":
		return scheme.FileSymlink
	case "fileHidden":
		return scheme.FileHidden
	case "cursor":
		return scheme.Cursor
	default:
		// Return transparent color as fallback
		return color.RGBA{0, 0, 0, 0}
	}
}
