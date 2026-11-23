package theme

import (
	"image/color"
	"strings"
	"sync"
	"time"

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
	ColorLineEditCursor          = "lineEditCursor"
	ColorLineEditSelection       = "lineEditSelection"
	ColorDialogListCursor        = "dialogListCursor"
	ColorMenuCursor              = "menuCursor"
	ColorCopyMoveOpenDestination = "copyMoveOpenDestination"
	ColorSearchOverlayBackground = "searchOverlayBackground"
	ColorSearchOverlayForeground = "searchOverlayForeground"
	ColorBusyOverlayBackground   = "busyOverlayBackground"
)

var (
	fyneAppColorDefaults = map[string]fyne.ThemeColorName{
		ColorLineEditCursor:    fynetheme.ColorNamePrimary,
		ColorLineEditSelection: fynetheme.ColorNameSelection,
		ColorDialogListCursor:  fynetheme.ColorNameSelection,
		ColorMenuCursor:        fynetheme.ColorNameFocus,
	}

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
		ColorCopyMoveOpenDestination: {30, 120, 80, 255},
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
		ColorCopyMoveOpenDestination: {120, 220, 170, 255},
		ColorSearchOverlayBackground: {220, 220, 220, 240},
		ColorSearchOverlayForeground: {0, 0, 0, 255},
		ColorBusyOverlayBackground:   {0, 0, 0, 96},
	}

	darkPalette = map[fyne.ThemeColorName]color.Color{
		fynetheme.ColorNameBackground:          color.NRGBA{R: 0x18, G: 0x18, B: 0x1b, A: 0xff},
		fynetheme.ColorNameOverlayBackground:   color.NRGBA{R: 0x22, G: 0x22, B: 0x26, A: 0xff},
		fynetheme.ColorNameMenuBackground:      color.NRGBA{R: 0x26, G: 0x26, B: 0x2b, A: 0xff},
		fynetheme.ColorNameHeaderBackground:    color.NRGBA{R: 0x1e, G: 0x1e, B: 0x22, A: 0xff},
		fynetheme.ColorNameInputBackground:     color.NRGBA{R: 0x1b, G: 0x1b, B: 0x1f, A: 0xff},
		fynetheme.ColorNameInputBorder:         color.NRGBA{R: 0x3a, G: 0x3a, B: 0x42, A: 0xff},
		fynetheme.ColorNameButton:              color.NRGBA{R: 0x2e, G: 0x2e, B: 0x35, A: 0xff},
		fynetheme.ColorNameDisabledButton:      color.NRGBA{R: 0x27, G: 0x27, B: 0x2c, A: 0xff},
		fynetheme.ColorNameDisabled:            color.NRGBA{R: 0x6b, G: 0x6b, B: 0x76, A: 0xff},
		fynetheme.ColorNameForeground:          color.NRGBA{R: 0xec, G: 0xec, B: 0xef, A: 0xff},
		fynetheme.ColorNamePlaceHolder:         color.NRGBA{R: 0x8a, G: 0x8a, B: 0x96, A: 0xff},
		fynetheme.ColorNameSeparator:           color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x1c},
		fynetheme.ColorNameHover:               color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x10},
		fynetheme.ColorNameFocus:               color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0x48},
		fynetheme.ColorNameSelection:           color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0x5c},
		fynetheme.ColorNamePrimary:             color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff},
		fynetheme.ColorNameForegroundOnPrimary: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fynetheme.ColorNameError:               color.NRGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff},
		fynetheme.ColorNameForegroundOnError:   color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fynetheme.ColorNameShadow:              color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x73},
		fynetheme.ColorNameScrollBar:           color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x60},
		fynetheme.ColorNameScrollBarBackground: color.NRGBA{R: 0x1e, G: 0x1e, B: 0x22, A: 0xff},
	}

	lightPalette = map[fyne.ThemeColorName]color.Color{
		fynetheme.ColorNameBackground:          color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fynetheme.ColorNameOverlayBackground:   color.NRGBA{R: 0xfb, G: 0xfb, B: 0xfc, A: 0xff},
		fynetheme.ColorNameMenuBackground:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fynetheme.ColorNameHeaderBackground:    color.NRGBA{R: 0xf4, G: 0xf5, B: 0xf7, A: 0xff},
		fynetheme.ColorNameInputBackground:     color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fynetheme.ColorNameInputBorder:         color.NRGBA{R: 0xd0, G: 0xd5, B: 0xdd, A: 0xff},
		fynetheme.ColorNameButton:              color.NRGBA{R: 0xee, G: 0xf0, B: 0xf3, A: 0xff},
		fynetheme.ColorNameDisabledButton:      color.NRGBA{R: 0xf3, G: 0xf4, B: 0xf6, A: 0xff},
		fynetheme.ColorNameDisabled:            color.NRGBA{R: 0x98, G: 0xa2, B: 0xb3, A: 0xff},
		fynetheme.ColorNameForeground:          color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff},
		fynetheme.ColorNamePlaceHolder:         color.NRGBA{R: 0x6e, G: 0x77, B: 0x81, A: 0xff},
		fynetheme.ColorNameSeparator:           color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x1a},
		fynetheme.ColorNameHover:               color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0d},
		fynetheme.ColorNameFocus:               color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0x28},
		fynetheme.ColorNameSelection:           color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0x38},
		fynetheme.ColorNamePrimary:             color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff},
		fynetheme.ColorNameForegroundOnPrimary: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fynetheme.ColorNameError:               color.NRGBA{R: 0xdc, G: 0x26, B: 0x26, A: 0xff},
		fynetheme.ColorNameForegroundOnError:   color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		fynetheme.ColorNameShadow:              color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x44},
		fynetheme.ColorNameScrollBar:           color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x60},
		fynetheme.ColorNameScrollBarBackground: color.NRGBA{R: 0xf0, G: 0xf1, B: 0xf4, A: 0xff},
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
	if _, ok := lightAppColorDefaults[name]; ok {
		return true
	}
	_, ok := fyneAppColorDefaults[name]
	return ok
}

// CustomTheme implements fyne.Theme with configurable font settings
type CustomTheme struct {
	config        *config.Config
	customFont    fyne.Resource
	monospaceFont fyne.Resource
	lightBase     fyne.Theme
	darkBase      fyne.Theme
	lightBaseOnce sync.Once
	darkBaseOnce  sync.Once
	debugPrint    func(format string, args ...interface{})
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
	started := time.Now()
	resolver := newFontResolver()
	t.customFont = resolver.resolveThemeFont(t.config.Theme, t.debugPrint)
	t.monospaceFont = resolver.resolveThemeMonospaceFont(t.config.Theme, t.debugPrint)
	if t.debugPrint != nil {
		t.debugPrint("Theme: fonts resolved elapsed=%s", time.Since(started))
	}
}

// Color resolves the application's surface palette, delegating any
// uncustomized Fyne color tokens to the matching built-in theme variant.
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == fynetheme.VariantDark
	if t != nil && t.config != nil {
		dark = t.config.Theme.Dark
	}
	return t.colorForVariant(name, dark)
}

func (t *CustomTheme) colorForVariant(name fyne.ThemeColorName, dark bool) color.Color {
	palette := lightPalette
	variant := fynetheme.VariantLight
	if dark {
		palette = darkPalette
		variant = fynetheme.VariantDark
	}
	if c, ok := palette[name]; ok {
		return c
	}
	return t.baseTheme(dark).Color(name, variant)
}

// Icon methods from default theme
func (t *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.fyneTheme().Icon(name)
}

// fyneTheme returns the base Fyne theme matching the configured dark/light
// variant, for delegating colors, icons, and font fallbacks.
func (t *CustomTheme) fyneTheme() fyne.Theme {
	return t.baseTheme(t.config.Theme.Dark)
}

func (t *CustomTheme) baseTheme(dark bool) fyne.Theme {
	if dark {
		t.darkBaseOnce.Do(func() {
			t.darkBase = fynetheme.DarkTheme()
		})
		return t.darkBase
	}
	t.lightBaseOnce.Do(func() {
		t.lightBase = fynetheme.LightTheme()
	})
	return t.lightBase
}

// Font method with custom font support
func (t *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace {
		if t.monospaceFont != nil {
			return t.monospaceFont
		}
		// Do not fall back to the proportional UI font: widget.TextGrid sizes
		// every cell from "M", so a proportional face renders with uneven gaps.
		return t.fyneTheme().Font(style)
	}

	// Return custom font if loaded and available
	if t.customFont != nil {
		return t.customFont
	}

	return t.fyneTheme().Font(style)
}

// Size method with custom font size, spacing, and corner-radius support.
func (t *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == fynetheme.SizeNameText && t.config.Theme.FontSize > 0 {
		return float32(t.config.Theme.FontSize)
	}

	switch name {
	case fynetheme.SizeNameButtonRadius, fynetheme.SizeNameInputRadius, fynetheme.SizeNameMenuRadius:
		return 6
	case fynetheme.SizeNamePopupRadius, fynetheme.SizeNameCardRadius, fynetheme.SizeNameDialogRadius:
		return 8
	case fynetheme.SizeNameSelectionRadius, fynetheme.SizeNameScrollBarRadius:
		return 4
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

	return t.fyneTheme().Size(name)
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
		fyneColorName, ok := fyneAppColorDefaults[colorType]
		if !ok {
			return color.RGBA{0, 0, 0, 0}
		}
		fallback = color.RGBAModel.Convert(t.colorForVariant(fyneColorName, dark)).(color.RGBA)
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
		return color.RGBAModel.Convert(t.colorForVariant(colorName, variant == fynetheme.VariantDark)).(color.RGBA), true
	}
	if primaryColorNames[name] {
		return color.RGBAModel.Convert(fynetheme.PrimaryColorNamed(name)).(color.RGBA), true
	}
	return color.RGBA{}, false
}

const (
	dialogInnerPadding float32 = 5
	dialogMinPadding   float32 = 6
)

// NewDialogOverrideTheme returns a scoped theme for dialog surfaces so buttons,
// entries, and controls keep compact inner padding while the main file list
// retains its zero-inner-padding density.
func NewDialogOverrideTheme(base fyne.Theme) fyne.Theme {
	if base == nil {
		base = fynetheme.DefaultTheme()
	}
	return &dialogOverrideTheme{base: base}
}

type dialogOverrideTheme struct {
	base fyne.Theme
}

func (t *dialogOverrideTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return t.base.Color(name, variant)
}

func (t *dialogOverrideTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *dialogOverrideTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *dialogOverrideTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case fynetheme.SizeNameInnerPadding:
		return dialogInnerPadding
	case fynetheme.SizeNamePadding:
		if basePad := t.base.Size(name); basePad > dialogMinPadding {
			return basePad
		}
		return dialogMinPadding
	}
	return t.base.Size(name)
}

// NewLineEditOverrideTheme returns a scoped theme for line edit dialogs.
func NewLineEditOverrideTheme(base fyne.Theme, themeProvider interface {
	GetCustomColor(string) color.RGBA
}) fyne.Theme {
	return newColorOverrideTheme(base, map[fyne.ThemeColorName]color.Color{
		fynetheme.ColorNamePrimary:   themeProvider.GetCustomColor(ColorLineEditCursor),
		fynetheme.ColorNameSelection: themeProvider.GetCustomColor(ColorLineEditSelection),
	})
}

// NewDialogListOverrideTheme returns a scoped theme for dialog cursor lists.
func NewDialogListOverrideTheme(base fyne.Theme, themeProvider interface {
	GetCustomColor(string) color.RGBA
}) fyne.Theme {
	return newColorOverrideTheme(base, map[fyne.ThemeColorName]color.Color{
		fynetheme.ColorNameSelection: themeProvider.GetCustomColor(ColorDialogListCursor),
	})
}

// NewMenuOverrideTheme returns a scoped theme for command menu cursor rows.
func NewMenuOverrideTheme(base fyne.Theme, themeProvider interface {
	GetCustomColor(string) color.RGBA
}) fyne.Theme {
	return newColorOverrideTheme(base, map[fyne.ThemeColorName]color.Color{
		fynetheme.ColorNameFocus: themeProvider.GetCustomColor(ColorMenuCursor),
	})
}

func newColorOverrideTheme(base fyne.Theme, colors map[fyne.ThemeColorName]color.Color) fyne.Theme {
	if base == nil {
		base = fynetheme.DefaultTheme()
	}
	return &colorOverrideTheme{base: base, colors: colors}
}

type colorOverrideTheme struct {
	base   fyne.Theme
	colors map[fyne.ThemeColorName]color.Color
}

func (t *colorOverrideTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if c, ok := t.colors[name]; ok {
		return c
	}
	return t.base.Color(name, variant)
}

func (t *colorOverrideTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *colorOverrideTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *colorOverrideTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}
