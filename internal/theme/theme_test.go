package theme

import (
	"image/color"
	"testing"

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
