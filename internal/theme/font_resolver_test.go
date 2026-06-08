package theme

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	fynetheme "fyne.io/fyne/v2/theme"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
)

func TestConfiguredFontNames(t *testing.T) {
	if got := configuredFontNames("Custom Font", true); len(got) < 2 || got[0] != "Custom Font" {
		t.Fatalf("configuredFontNames returned %#v", got)
	}

	if got := configuredFontNames(" auto ", true); len(got) == 0 {
		t.Fatal("configuredFontNames should return default names for auto")
	}

	if got := configuredFontNames("", true); len(got) == 0 {
		t.Fatal("configuredFontNames should return default names for empty input")
	}

	if got := configuredFontNames("", false); len(got) != 0 {
		t.Fatalf("configuredFontNames without defaults returned %#v, want empty", got)
	}

	if got := configuredFontNames("Custom Mono", false); len(got) != 1 || got[0] != "Custom Mono" {
		t.Fatalf("configuredFontNames without defaults returned %#v, want only configured name", got)
	}
}

func TestDefaultFontNames(t *testing.T) {
	tests := []struct {
		goos string
		want string
	}{
		{goos: "windows", want: "Yu Gothic UI"},
		{goos: "linux", want: "Noto Sans CJK JP"},
		{goos: "plan9", want: "Noto Sans"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got := defaultFontNames(tt.goos)
			if len(got) == 0 || got[0] != tt.want {
				t.Fatalf("defaultFontNames(%q) = %#v, want first %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestLoadFontResourceFromPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, fynetheme.DefaultTextFont().Content(), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	res, err := loadFontResourceFromPath(path)
	if err != nil {
		t.Fatalf("loadFontResourceFromPath failed: %v", err)
	}
	if res.Name() != "font.ttf" {
		t.Fatalf("resource name = %q, want font.ttf", res.Name())
	}
}

func TestSortFontLocationsByRegularPreference(t *testing.T) {
	locations := []fontscan.Location{
		{File: "family-bold.ttf"},
		{File: "family-italic.ttf"},
		{File: "family-regular.ttf"},
		{File: "family-medium.ttf"},
	}
	descriptions := map[string]font.Description{
		"family-bold.ttf": {
			Family: "Family",
			Aspect: font.Aspect{
				Style:   font.StyleNormal,
				Weight:  font.WeightBold,
				Stretch: font.StretchNormal,
			},
		},
		"family-italic.ttf": {
			Family: "Family",
			Aspect: font.Aspect{
				Style:   font.StyleItalic,
				Weight:  font.WeightNormal,
				Stretch: font.StretchNormal,
			},
		},
		"family-regular.ttf": {
			Family: "Family",
			Aspect: font.Aspect{
				Style:   font.StyleNormal,
				Weight:  font.WeightNormal,
				Stretch: font.StretchNormal,
			},
		},
		"family-medium.ttf": {
			Family: "Family",
			Aspect: font.Aspect{
				Style:   font.StyleNormal,
				Weight:  font.WeightMedium,
				Stretch: font.StretchNormal,
			},
		},
	}

	sortFontLocationsByRegularPreference(locations, func(location fontscan.Location) (font.Description, bool) {
		desc, ok := descriptions[location.File]
		return desc, ok
	})

	got := []string{locations[0].File, locations[1].File, locations[2].File, locations[3].File}
	want := []string{"family-regular.ttf", "family-medium.ttf", "family-bold.ttf", "family-italic.ttf"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted locations = %#v, want %#v", got, want)
		}
	}
}

func TestScanFontLocationsByNameFindsUDEVWhenAvailable(t *testing.T) {
	if _, err := os.Stat("/home/neko/.fonts/UDEVGothicJPDOC-Regular.ttf"); err != nil {
		t.Skipf("UDEV Gothic JPDOC unavailable: %v", err)
	}

	locations := scanFontLocationsByName("UDEV Gothic JPDOC")
	if len(locations) == 0 {
		t.Fatal("scanFontLocationsByName did not find UDEV Gothic JPDOC")
	}
}

func TestExtractCollectionFontDataWhenNotoCJKIsAvailable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Noto CJK TTC path is Linux-specific")
	}

	path := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("Noto CJK TTC unavailable: %v", err)
	}

	data, err := extractCollectionFontData(path, 0)
	if err != nil {
		t.Fatalf("extractCollectionFontData failed: %v", err)
	}
	if err := validateFontData(data); err != nil {
		t.Fatalf("extracted font failed validation: %v", err)
	}
}
