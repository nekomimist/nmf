package theme

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"nmf/internal/config"
)

func TestCustomThemeDelegatesFyneThemeColors(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	tests := []struct {
		name string
		dark bool
	}{
		{name: "dark", dark: true},
		{name: "light", dark: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Theme: config.ThemeConfig{Dark: tt.dark}}
			customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})
			variant := theme.VariantLight
			base := theme.LightTheme()
			if tt.dark {
				variant = theme.VariantDark
				base = theme.DarkTheme()
			}

			got := color.NRGBAModel.Convert(customTheme.Color(theme.ColorNamePrimary, variant))
			want := color.NRGBAModel.Convert(base.Color(theme.ColorNamePrimary, variant))
			if got != want {
				t.Fatalf("primary color = %#v, want base primary %#v", got, want)
			}

			got = color.NRGBAModel.Convert(customTheme.Color(theme.ColorNameFocus, variant))
			want = color.NRGBAModel.Convert(base.Color(theme.ColorNameFocus, variant))
			if got != want {
				t.Fatalf("focus color = %#v, want base focus %#v", got, want)
			}
		})
	}
}

func TestCustomThemeAppColorOverrides(t *testing.T) {
	cfg := &config.Config{
		Theme: config.ThemeConfig{
			Dark: true,
			Colors: map[string]config.ThemeColorConfig{
				ColorCursor: {
					Value:       &config.ThemeColorValue{RGBA: [4]uint8{1, 2, 3, 4}, IsRGBA: true},
					Light:       &config.ThemeColorValue{Name: "foreground"},
					DarkDefault: true,
				},
			},
		},
	}
	customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})

	if got, want := customTheme.GetCustomColor(ColorCursor), (color.RGBA{255, 255, 255, 255}); got != want {
		t.Fatalf("dark cursor = %#v, want default %#v", got, want)
	}

	got := color.NRGBAModel.Convert(customTheme.GetCustomColorForVariant(ColorCursor, false))
	want := color.NRGBAModel.Convert(theme.LightTheme().Color(theme.ColorNameForeground, theme.VariantLight))
	if got != want {
		t.Fatalf("light cursor = %#v, want foreground %#v", got, want)
	}
}

func TestCustomThemeFyneBackedAppColorDefaults(t *testing.T) {
	cfg := &config.Config{Theme: config.ThemeConfig{Dark: false}}
	customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})

	tests := []struct {
		name  string
		color string
		want  color.Color
	}{
		{name: "line edit cursor", color: ColorLineEditCursor, want: theme.LightTheme().Color(theme.ColorNamePrimary, theme.VariantLight)},
		{name: "line edit selection", color: ColorLineEditSelection, want: theme.LightTheme().Color(theme.ColorNameSelection, theme.VariantLight)},
		{name: "dialog list cursor", color: ColorDialogListCursor, want: theme.LightTheme().Color(theme.ColorNameSelection, theme.VariantLight)},
		{name: "menu cursor", color: ColorMenuCursor, want: theme.LightTheme().Color(theme.ColorNameFocus, theme.VariantLight)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := color.RGBAModel.Convert(customTheme.GetCustomColor(tt.color))
			want := color.RGBAModel.Convert(tt.want)
			if got != want {
				t.Fatalf("%s = %#v, want %#v", tt.color, got, want)
			}
			if !IsAppColorName(tt.color) {
				t.Fatalf("%s should be an app color name", tt.color)
			}
		})
	}
}

func TestCustomThemeCopyMoveOpenDestinationColor(t *testing.T) {
	cfg := &config.Config{}
	customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})

	if got, want := customTheme.GetCustomColor(ColorCopyMoveOpenDestination), (color.RGBA{30, 120, 80, 255}); got != want {
		t.Fatalf("copy move open destination = %#v, want %#v", got, want)
	}
	if !IsAppColorName(ColorCopyMoveOpenDestination) {
		t.Fatalf("%s should be an app color name", ColorCopyMoveOpenDestination)
	}
}

func TestCustomThemeMonospaceFontFallsBackToCustomFont(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cfg := &config.Config{
		Theme: config.ThemeConfig{
			FontPath: theme.DefaultTextFont().Name(),
		},
	}
	customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})
	customTheme.customFont = fyne.NewStaticResource("ui.ttf", theme.DefaultTextFont().Content())

	if got := customTheme.Font(fyne.TextStyle{Monospace: true}); got == nil || got.Name() != "ui.ttf" {
		t.Fatalf("monospace font = %v, want custom UI fallback", got)
	}
}

func TestCustomThemeMonospaceFontOverridesCustomFont(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cfg := &config.Config{}
	customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})
	customTheme.customFont = fyne.NewStaticResource("ui.ttf", theme.DefaultTextFont().Content())
	customTheme.monospaceFont = fyne.NewStaticResource("mono.ttf", theme.DefaultTextFont().Content())

	if got := customTheme.Font(fyne.TextStyle{Monospace: true}); got == nil || got.Name() != "mono.ttf" {
		t.Fatalf("monospace font = %v, want mono override", got)
	}
	if got := customTheme.Font(fyne.TextStyle{}); got == nil || got.Name() != "ui.ttf" {
		t.Fatalf("regular font = %v, want UI font", got)
	}
}

func TestCustomThemeFyneBackedAppColorOverrides(t *testing.T) {
	cfg := &config.Config{
		Theme: config.ThemeConfig{
			Colors: map[string]config.ThemeColorConfig{
				ColorDialogListCursor: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{9, 8, 7, 6}, IsRGBA: true},
				},
			},
		},
	}
	customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})

	if got, want := customTheme.GetCustomColor(ColorDialogListCursor), (color.RGBA{9, 8, 7, 6}); got != want {
		t.Fatalf("dialog list cursor = %#v, want %#v", got, want)
	}
}

func TestScopedOverrideThemes(t *testing.T) {
	cfg := &config.Config{
		Theme: config.ThemeConfig{
			Colors: map[string]config.ThemeColorConfig{
				ColorLineEditCursor: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{1, 2, 3, 4}, IsRGBA: true},
				},
				ColorLineEditSelection: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{5, 6, 7, 8}, IsRGBA: true},
				},
				ColorDialogListCursor: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{9, 10, 11, 12}, IsRGBA: true},
				},
				ColorMenuCursor: {
					Value: &config.ThemeColorValue{RGBA: [4]uint8{13, 14, 15, 16}, IsRGBA: true},
				},
			},
		},
	}
	customTheme := NewCustomTheme(cfg, func(string, ...interface{}) {})

	lineEditTheme := NewLineEditOverrideTheme(theme.LightTheme(), customTheme)
	if got, want := lineEditTheme.Color(theme.ColorNamePrimary, theme.VariantLight), (color.RGBA{1, 2, 3, 4}); got != want {
		t.Fatalf("line edit primary = %#v, want %#v", got, want)
	}
	if got, want := lineEditTheme.Color(theme.ColorNameSelection, theme.VariantLight), (color.RGBA{5, 6, 7, 8}); got != want {
		t.Fatalf("line edit selection = %#v, want %#v", got, want)
	}

	listTheme := NewDialogListOverrideTheme(theme.LightTheme(), customTheme)
	if got, want := listTheme.Color(theme.ColorNameSelection, theme.VariantLight), (color.RGBA{9, 10, 11, 12}); got != want {
		t.Fatalf("dialog list selection = %#v, want %#v", got, want)
	}

	menuTheme := NewMenuOverrideTheme(theme.LightTheme(), customTheme)
	if got, want := menuTheme.Color(theme.ColorNameFocus, theme.VariantLight), (color.RGBA{13, 14, 15, 16}); got != want {
		t.Fatalf("menu focus = %#v, want %#v", got, want)
	}
}
