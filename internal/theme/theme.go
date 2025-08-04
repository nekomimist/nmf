package theme

import (
	"image/color"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"nmf/internal/config"
)

// CustomTheme implements fyne.Theme with configurable font settings
type CustomTheme struct {
	config     *config.Config
	customFont fyne.Resource
}

// NewCustomTheme creates a new custom theme with the given configuration
func NewCustomTheme(config *config.Config) *CustomTheme {
	customTheme := &CustomTheme{config: config}

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
		log.Printf("Custom font file not found: %s", fontPath)
		return
	}

	// Read font file
	fontData, err := ioutil.ReadFile(fontPath)
	if err != nil {
		log.Printf("Error reading font file %s: %v", fontPath, err)
		return
	}

	// Create font resource
	t.customFont = fyne.NewStaticResource(filepath.Base(fontPath), fontData)
	log.Printf("Loaded custom font: %s", fontPath)
}

// Color methods from default theme
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.config.Theme.Dark {
		return theme.DarkTheme().Color(name, variant)
	}
	return theme.DefaultTheme().Color(name, variant)
}

// Icon methods from default theme
func (t *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	if t.config.Theme.Dark {
		return theme.DarkTheme().Icon(name)
	}
	return theme.DefaultTheme().Icon(name)
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
	return theme.DefaultTheme().Font(style)
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
	return theme.DefaultTheme().Size(name)
}
