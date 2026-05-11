package theme

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"nmf/internal/config"
)

const (
	ColorFileRegular             = "fileRegular"
	ColorFileDirectory           = "fileDirectory"
	ColorFileSymlink             = "fileSymlink"
	ColorFileHidden              = "fileHidden"
	ColorStatusAdded             = "statusAdded"
	ColorStatusDeleted           = "statusDeleted"
	ColorStatusModified          = "statusModified"
	ColorSelectionBackground     = "selectionBackground"
	ColorCursor                  = "cursor"
	ColorSearchOverlayBackground = "searchOverlayBackground"
	ColorSearchOverlayForeground = "searchOverlayForeground"
	ColorBusyOverlayBackground   = "busyOverlayBackground"
)

var (
	lightAppColorDefaults = map[string]color.RGBA{
		ColorFileRegular:             {60, 60, 60, 255},
		ColorFileDirectory:           {30, 100, 200, 255},
		ColorFileSymlink:             {200, 100, 0, 255},
		ColorFileHidden:              {120, 120, 120, 255},
		ColorStatusAdded:             {0, 150, 0, 80},
		ColorStatusDeleted:           {100, 100, 100, 60},
		ColorStatusModified:          {200, 150, 0, 80},
		ColorSelectionBackground:     {70, 120, 170, 100},
		ColorCursor:                  {0, 0, 0, 255},
		ColorSearchOverlayBackground: {40, 40, 40, 240},
		ColorSearchOverlayForeground: {255, 255, 255, 255},
		ColorBusyOverlayBackground:   {0, 0, 0, 96},
	}
	darkAppColorDefaults = map[string]color.RGBA{
		ColorFileRegular:             {220, 220, 220, 255},
		ColorFileDirectory:           {135, 206, 250, 255},
		ColorFileSymlink:             {255, 165, 0, 255},
		ColorFileHidden:              {105, 105, 105, 255},
		ColorStatusAdded:             {0, 200, 0, 80},
		ColorStatusDeleted:           {128, 128, 128, 60},
		ColorStatusModified:          {255, 200, 0, 80},
		ColorSelectionBackground:     {100, 150, 200, 100},
		ColorCursor:                  {255, 255, 255, 255},
		ColorSearchOverlayBackground: {220, 220, 220, 240},
		ColorSearchOverlayForeground: {0, 0, 0, 255},
		ColorBusyOverlayBackground:   {0, 0, 0, 96},
	}

	fyneColorNames = map[string]fyne.ThemeColorName{
		"background":          fynetheme.ColorNameBackground,
		"button":              fynetheme.ColorNameButton,
		"disabledButton":      fynetheme.ColorNameDisabledButton,
		"disabled":            fynetheme.ColorNameDisabled,
		"error":               fynetheme.ColorNameError,
		"focus":               fynetheme.ColorNameFocus,
		"foreground":          fynetheme.ColorNameForeground,
		"foregroundOnError":   fynetheme.ColorNameForegroundOnError,
		"foregroundOnPrimary": fynetheme.ColorNameForegroundOnPrimary,
		"foregroundOnSuccess": fynetheme.ColorNameForegroundOnSuccess,
		"foregroundOnWarning": fynetheme.ColorNameForegroundOnWarning,
		"headerBackground":    fynetheme.ColorNameHeaderBackground,
		"hover":               fynetheme.ColorNameHover,
		"hyperlink":           fynetheme.ColorNameHyperlink,
		"inputBackground":     fynetheme.ColorNameInputBackground,
		"inputBorder":         fynetheme.ColorNameInputBorder,
		"menuBackground":      fynetheme.ColorNameMenuBackground,
		"overlayBackground":   fynetheme.ColorNameOverlayBackground,
		"placeholder":         fynetheme.ColorNamePlaceHolder,
		"pressed":             fynetheme.ColorNamePressed,
		"primary":             fynetheme.ColorNamePrimary,
		"scrollBar":           fynetheme.ColorNameScrollBar,
		"scrollBarBackground": fynetheme.ColorNameScrollBarBackground,
		"selection":           fynetheme.ColorNameSelection,
		"separator":           fynetheme.ColorNameSeparator,
		"success":             fynetheme.ColorNameSuccess,
		"shadow":              fynetheme.ColorNameShadow,
		"warning":             fynetheme.ColorNameWarning,
	}

	primaryColorNames = map[string]bool{
		fynetheme.ColorRed:    true,
		fynetheme.ColorOrange: true,
		fynetheme.ColorYellow: true,
		fynetheme.ColorGreen:  true,
		fynetheme.ColorBlue:   true,
		fynetheme.ColorPurple: true,
		fynetheme.ColorBrown:  true,
		fynetheme.ColorGray:   true,
	}
)

// IsAppColorName reports whether name is a configurable NMF UI color.
func IsAppColorName(name string) bool {
	_, ok := lightAppColorDefaults[name]
	return ok
}

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

	customTheme.loadCustomFont()

	return customTheme
}

// loadCustomFont resolves and loads the configured font.
func (t *CustomTheme) loadCustomFont() {
	t.customFont = resolveThemeFont(t.config.Theme, t.debugPrint)
}

// Color methods from default theme
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.config.Theme.Dark {
		return fynetheme.DarkTheme().Color(name, variant)
	}
	return fynetheme.LightTheme().Color(name, variant)
}

// Icon methods from default theme
func (t *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	if t.config.Theme.Dark {
		return fynetheme.DarkTheme().Icon(name)
	}
	return fynetheme.LightTheme().Icon(name)
}

// Font method with custom font support
func (t *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	// Return custom font if loaded and available
	if t.customFont != nil {
		return t.customFont
	}

	if t.config.Theme.Dark {
		return fynetheme.DarkTheme().Font(style)
	}
	return fynetheme.LightTheme().Font(style)
}

// Size method with custom font size and spacing support
func (t *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == fynetheme.SizeNameText && t.config.Theme.FontSize > 0 {
		return float32(t.config.Theme.FontSize)
	}

	// Custom item spacing support with icon consideration
	if t.config.UI.ItemSpacing > 0 {
		switch name {
		case fynetheme.SizeNamePadding:
			// Ensure minimum padding for icons but allow small spacing
			minPadding := float32(2) // Very minimal padding
			requested := float32(t.config.UI.ItemSpacing)
			if requested < minPadding {
				return minPadding
			}
			return requested
		case fynetheme.SizeNameInnerPadding:
			return 0
		}
	}

	if t.config.Theme.Dark {
		return fynetheme.DarkTheme().Size(name)
	}
	return fynetheme.LightTheme().Size(name)
}

// GetCustomColor returns app-specific colors based on the current theme.
func (t *CustomTheme) GetCustomColor(colorType string) color.RGBA {
	return t.GetCustomColorForVariant(colorType, t.config.Theme.Dark)
}

// GetCustomColorForVariant resolves an app-specific color for a theme variant.
func (t *CustomTheme) GetCustomColorForVariant(colorType string, dark bool) color.RGBA {
	defaults := lightAppColorDefaults
	variant := fynetheme.VariantLight
	if dark {
		defaults = darkAppColorDefaults
		variant = fynetheme.VariantDark
	}
	fallback, ok := defaults[colorType]
	if !ok {
		return color.RGBA{0, 0, 0, 0}
	}
	if t == nil || t.config == nil || t.config.Theme.Colors == nil {
		return fallback
	}
	override, ok := t.config.Theme.Colors[colorType]
	if !ok {
		return fallback
	}
	value := override.Value
	if dark {
		if override.DarkDefault {
			return fallback
		}
		if override.Dark != nil {
			value = override.Dark
		}
	}
	if !dark {
		if override.LightDefault {
			return fallback
		}
		if override.Light != nil {
			value = override.Light
		}
	}
	if value == nil {
		return fallback
	}
	resolved, ok := t.resolveConfiguredColor(*value, variant)
	if !ok {
		t.debugPrint("Theme: Unknown color name=%s appColor=%s", value.Name, colorType)
		return fallback
	}
	return resolved
}

func (t *CustomTheme) resolveConfiguredColor(value config.ThemeColorValue, variant fyne.ThemeVariant) (color.RGBA, bool) {
	if value.IsRGBA {
		return color.RGBA{value.RGBA[0], value.RGBA[1], value.RGBA[2], value.RGBA[3]}, true
	}
	name := strings.TrimSpace(value.Name)
	if name == "" {
		return color.RGBA{}, false
	}
	if colorName, ok := fyneColorNames[name]; ok {
		base := fynetheme.LightTheme()
		if variant == fynetheme.VariantDark {
			base = fynetheme.DarkTheme()
		}
		return color.RGBAModel.Convert(base.Color(colorName, variant)).(color.RGBA), true
	}
	if primaryColorNames[name] {
		return color.RGBAModel.Convert(fynetheme.PrimaryColorNamed(name)).(color.RGBA), true
	}
	return color.RGBA{}, false
}
