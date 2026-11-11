package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMergeConfigValidationLeavesDefaultsUnchanged(t *testing.T) {
	// A valid early field must not be applied when any later section fails.
	for _, tt := range []struct {
		name string
		json string
		want string
	}{
		{"window", `{"window":{"width":1234,"height":0}}`, "window.height must be positive"},
		{"theme", `{"window":{"width":1234},"theme":{"fontSize":-1}}`, "theme.fontSize must be zero or positive"},
		{"debug", `{"window":{"width":1234},"debug":{"maxLogFiles":0}}`, "debug.maxLogFiles must be positive"},
		{"sort", `{"window":{"width":1234},"ui":{"sort":{"sortBy":"invalid"}}}`, "ui.sort.sortBy must be name, size, modified, or extension"},
		{"spacing", `{"window":{"width":1234},"ui":{"itemSpacing":-1}}`, "ui.itemSpacing must be zero or positive"},
		{"viewer", `{"window":{"width":1234},"ui":{"viewer":{"maxHeight":-1}}}`, "ui.viewer.maxHeight must be zero or positive"},
		{"archive", `{"window":{"width":1234},"ui":{"archive":{"zipNameEncoding":" "}}}`, "ui.archive.zipNameEncoding must not be empty"},
		{"cursor", `{"window":{"width":1234},"ui":{"cursorStyle":{"thickness":-1}}}`, "ui.cursorStyle.thickness must be zero or positive"},
		{"history", `{"window":{"width":1234},"ui":{"fileFilter":{"maxEntries":0}}}`, "ui.fileFilter.maxEntries must be positive"},
		{"first error wins", `{"window":{"width":1234},"ui":{"sort":{"sortOrder":"invalid"},"viewer":{"maxWidth":-1}}}`, "ui.sort.sortOrder must be asc or desc"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var raw rawConfig
			if err := json.Unmarshal([]byte(tt.json), &raw); err != nil {
				t.Fatal(err)
			}
			cfg, want := getDefaultConfig(), getDefaultConfig()
			if err := mergeConfigs(cfg, &raw); err == nil || err.Error() != tt.want {
				t.Fatalf("mergeConfigs error = %v, want %q", err, tt.want)
			}
			if !reflect.DeepEqual(cfg, want) {
				t.Fatal("invalid overlay changed the defaults")
			}
		})
	}
}
