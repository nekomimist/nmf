package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestThemeColorValueMarshalJSON(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value ThemeColorValue
		want  string
	}{
		{"rgba", ThemeColorValue{RGBA: [4]uint8{1, 2, 3, 4}, IsRGBA: true}, `[1,2,3,4]`},
		{"transparent black", ThemeColorValue{IsRGBA: true}, `[0,0,0,0]`},
		{"channel limits", ThemeColorValue{RGBA: [4]uint8{0, 127, 128, 255}, IsRGBA: true}, `[0,127,128,255]`},
		{"named color", ThemeColorValue{Name: "foreground"}, `"foreground"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != tt.want {
				t.Fatalf("JSON = %s, want %s", encoded, tt.want)
			}
			var decoded ThemeColorValue
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded != tt.value {
				t.Fatalf("round trip = %#v, want %#v", decoded, tt.value)
			}
		})
	}
}

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
		{"common rgba", `{"value":[0,127,128,255],"dark":null}`, ThemeColorConfig{Value: &ThemeColorValue{RGBA: [4]uint8{0, 127, 128, 255}, IsRGBA: true}, DarkDefault: true}},
		{"dark rgba", `{"dark":[0,0,0,0],"light":null}`, ThemeColorConfig{Dark: &ThemeColorValue{IsRGBA: true}, LightDefault: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ThemeColorConfig{Value: common, Dark: common, LightDefault: true}
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("color = %#v, want %#v", got, tt.want)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip ThemeColorConfig
			if err := json.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(roundTrip, tt.want) {
				t.Fatalf("round trip lost color or optional field semantics: %s", encoded)
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
