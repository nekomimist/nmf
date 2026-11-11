package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestThemeColorOptionalFields(t *testing.T) {
	common := &ThemeColorValue{Name: "foreground"}
	for _, tt := range []struct {
		name string
		json string
		want ThemeColorConfig
	}{
		{"reset", `null`, ThemeColorConfig{}},
		{"absent", `{}`, ThemeColorConfig{}},
		{"direct", `"foreground"`, ThemeColorConfig{Value: common}},
		{"null common", `{"value":null}`, ThemeColorConfig{}},
		{"variant defaults", `{"value":"foreground","dark":null,"light":null}`, ThemeColorConfig{Value: common, DarkDefault: true, LightDefault: true}},
		{"variant values", `{"dark":"foreground","light":[1,2,3,4]}`, ThemeColorConfig{Dark: common, Light: &ThemeColorValue{RGBA: [4]uint8{1, 2, 3, 4}, IsRGBA: true}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ThemeColorConfig{Value: common, Dark: common, LightDefault: true}
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("color = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestThemeColorInvalidFieldLeavesReceiverUnchanged(t *testing.T) {
	for _, input := range []string{
		`{"value":false}`,
		`{"value":"foreground","dark":false}`,
		`{"value":"foreground","dark":null,"light":[1,2]}`,
		`{"value":"foreground","unknown":null}`,
	} {
		t.Run(input, func(t *testing.T) {
			got := ThemeColorConfig{Value: &ThemeColorValue{Name: "original"}, DarkDefault: true}
			want := cloneThemeColorConfig(got)
			if err := json.Unmarshal([]byte(input), &got); err == nil {
				t.Fatal("invalid color was accepted")
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("failed decoding mutated the receiver: %#v", got)
			}
		})
	}
}
