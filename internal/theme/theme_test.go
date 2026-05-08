package theme

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"nmf/internal/config"
)

func TestCustomThemePrimaryColorMatchesForegroundForEntryCursor(t *testing.T) {
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
			want := color.NRGBAModel.Convert(base.Color(theme.ColorNameForeground, variant))
			if got != want {
				t.Fatalf("primary color = %#v, want foreground %#v", got, want)
			}
		})
	}
}
