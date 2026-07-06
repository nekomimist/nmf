package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

	// Test Window defaults
	if config.Window.Width != 800 {
		t.Errorf("Expected default window width 800, got %d", config.Window.Width)
	}
	if config.Window.Height != 600 {
		t.Errorf("Expected default window height 600, got %d", config.Window.Height)
	}
	if config.Window.X != nil || config.Window.Y != nil {
		t.Errorf("Expected default window position to be unset, got x=%v y=%v", config.Window.X, config.Window.Y)
	}
	if config.Startup.Directory != "" {
		t.Errorf("Expected default startup directory empty, got %q", config.Startup.Directory)
	}

	// Test Theme defaults
	if !config.Theme.Dark {
		t.Error("Expected dark theme to be true by default")
	}
	if config.Theme.FontSize != 14 {
		t.Errorf("Expected default font size 14, got %d", config.Theme.FontSize)
	}
	if config.Theme.FontPath != "" {
		t.Errorf("Expected empty font path, got '%s'", config.Theme.FontPath)
	}
	if config.Theme.FontName != "" {
		t.Errorf("Expected empty font name, got '%s'", config.Theme.FontName)
	}
	if config.Theme.MonospaceFontPath != "" {
		t.Errorf("Expected empty monospace font path, got '%s'", config.Theme.MonospaceFontPath)
	}
	if config.Theme.MonospaceFontName != "" {
		t.Errorf("Expected empty monospace font name, got '%s'", config.Theme.MonospaceFontName)
	}
	if config.Debug.Enabled {
		t.Error("Expected debug logging to be disabled by default")
	}
	if config.Debug.LogDirectory != "" {
		t.Errorf("Expected empty debug log directory, got '%s'", config.Debug.LogDirectory)
	}
	if config.Debug.MaxLogFiles != 10 {
		t.Errorf("Expected default debug max log files 10, got %d", config.Debug.MaxLogFiles)
	}

	// Test UI defaults
	if config.UI.ShowHiddenFiles {
		t.Error("Expected ShowHiddenFiles to be false by default")
	}
	if config.UI.Sort.SortBy != "name" {
		t.Errorf("Expected default sort by 'name', got '%s'", config.UI.Sort.SortBy)
	}
	if config.UI.Sort.SortOrder != "asc" {
		t.Errorf("Expected default sort order 'asc', got '%s'", config.UI.Sort.SortOrder)
	}
	if !config.UI.Sort.DirectoriesFirst {
		t.Error("Expected default DirectoriesFirst to be true")
	}
	if config.UI.ItemSpacing != 4 {
		t.Errorf("Expected default item spacing 4, got %d", config.UI.ItemSpacing)
	}
	if config.UI.Copy.PreserveTimestamps {
		t.Error("Expected copy preserve timestamps to be disabled by default")
	}
	if config.UI.Viewer.MaxWidth != 0 || config.UI.Viewer.MaxHeight != 0 {
		t.Errorf("Expected default viewer max size 0x0, got %dx%d", config.UI.Viewer.MaxWidth, config.UI.Viewer.MaxHeight)
	}
	if config.UI.Viewer.DefaultPane != "auto" {
		t.Errorf("Expected default viewer pane auto, got %q", config.UI.Viewer.DefaultPane)
	}
	if config.UI.Archive.ZipNameEncoding != "shift_jis" {
		t.Errorf("Expected default ZIP name encoding 'shift_jis', got '%s'", config.UI.Archive.ZipNameEncoding)
	}
	if !config.UI.IME.Enabled {
		t.Error("Expected IME integration to be enabled by default")
	}

	// Test CursorStyle defaults
	if config.UI.CursorStyle.Type != "underline" {
		t.Errorf("Expected default cursor type 'underline', got '%s'", config.UI.CursorStyle.Type)
	}
	if config.UI.CursorStyle.Thickness != 2 {
		t.Errorf("Expected default cursor thickness 2, got %d", config.UI.CursorStyle.Thickness)
	}

	// Test CursorMemory defaults (entries/lastUsed live in state.json now)
	if config.UI.CursorMemory.MaxEntries != 100 {
		t.Errorf("Expected default cursor memory max entries 100, got %d", config.UI.CursorMemory.MaxEntries)
	}

	// Test NavigationHistory defaults (entries/lastUsed/useCount/pinned live in state.json now)
	if config.UI.NavigationHistory.MaxEntries != 10000 {
		t.Errorf("Expected default navigation history max entries 10000, got %d", config.UI.NavigationHistory.MaxEntries)
	}

	// Test FileFilter defaults (entries/current/enabled live in state.json now)
	if config.UI.FileFilter.MaxEntries != 30 {
		t.Errorf("Expected default file filter max entries 30, got %d", config.UI.FileFilter.MaxEntries)
	}

	// Test DirectoryJumps defaults
	if config.UI.DirectoryJumps.Entries == nil {
		t.Error("Expected directory jump entries to be initialized")
	}
	if len(config.UI.DirectoryJumps.Entries) != 0 {
		t.Errorf("Expected no default directory jumps, got %d", len(config.UI.DirectoryJumps.Entries))
	}
	if config.UI.KeyBindings == nil {
		t.Error("Expected key bindings to be initialized")
	}
	if config.UI.ExternalCommands == nil {
		t.Error("Expected external commands to be initialized")
	}
}

func TestMergeConfigsWindowPositionAndStartupDirectory(t *testing.T) {
	cfg := getDefaultConfig()
	x := 1920
	y := -40
	directory := "  ~/work  "

	mergeConfigs(cfg, &rawConfig{
		Window: rawWindowConfig{
			X: &x,
			Y: &y,
		},
		Startup: rawStartupConfig{
			Directory: &directory,
		},
	})

	if cfg.Window.X == nil || *cfg.Window.X != 1920 || cfg.Window.Y == nil || *cfg.Window.Y != -40 {
		t.Fatalf("window position = x=%v y=%v, want 1920,-40", cfg.Window.X, cfg.Window.Y)
	}
	if cfg.Startup.Directory != "~/work" {
		t.Fatalf("startup directory = %q, want ~/work", cfg.Startup.Directory)
	}
}

func TestMergeConfigsMergesMaxEntriesForTrimmedSections(t *testing.T) {
	cfg := getDefaultConfig()
	cursorMax := 5
	historyMax := 6
	filterMax := 7
	fileConfig := &rawConfig{
		UI: rawUIConfig{
			CursorMemory:      rawCursorMemoryConfig{MaxEntries: &cursorMax},
			NavigationHistory: rawNavigationHistoryConfig{MaxEntries: &historyMax},
			FileFilter:        rawFileFilterConfig{MaxEntries: &filterMax},
		},
	}

	mergeConfigs(cfg, fileConfig)

	if cfg.UI.CursorMemory.MaxEntries != 5 {
		t.Errorf("cursor memory max entries = %d, want 5", cfg.UI.CursorMemory.MaxEntries)
	}
	if cfg.UI.NavigationHistory.MaxEntries != 6 {
		t.Errorf("navigation history max entries = %d, want 6", cfg.UI.NavigationHistory.MaxEntries)
	}
	if cfg.UI.FileFilter.MaxEntries != 7 {
		t.Errorf("file filter max entries = %d, want 7", cfg.UI.FileFilter.MaxEntries)
	}
}

func TestMergeConfigsIgnoresNegativeViewerMaxSize(t *testing.T) {
	cfg := getDefaultConfig()
	cfg.UI.Viewer.MaxWidth = 1000
	cfg.UI.Viewer.MaxHeight = 800
	maxWidth := -1
	maxHeight := -2

	mergeConfigs(cfg, &rawConfig{
		UI: rawUIConfig{
			Viewer: rawViewerConfig{
				MaxWidth:  &maxWidth,
				MaxHeight: &maxHeight,
			},
		},
	})

	if cfg.UI.Viewer.MaxWidth != 1000 || cfg.UI.Viewer.MaxHeight != 800 {
		t.Fatalf("viewer max size = %dx%d, want unchanged 1000x800", cfg.UI.Viewer.MaxWidth, cfg.UI.Viewer.MaxHeight)
	}
}

func TestMergeConfigsDebugLogging(t *testing.T) {
	cfg := getDefaultConfig()
	enabled := true
	logDirectory := "debug-logs"
	maxLogFiles := 3

	mergeConfigs(cfg, &rawConfig{
		Debug: rawDebugConfig{
			Enabled:      &enabled,
			LogDirectory: &logDirectory,
			MaxLogFiles:  &maxLogFiles,
		},
	})

	if !cfg.Debug.Enabled {
		t.Fatal("debug enabled = false, want true")
	}
	if cfg.Debug.LogDirectory != "debug-logs" {
		t.Fatalf("debug log directory = %q, want debug-logs", cfg.Debug.LogDirectory)
	}
	if cfg.Debug.MaxLogFiles != 3 {
		t.Fatalf("debug max log files = %d, want 3", cfg.Debug.MaxLogFiles)
	}
}

func TestMergeConfigsIgnoresInvalidDebugMaxLogFiles(t *testing.T) {
	cfg := getDefaultConfig()
	cfg.Debug.MaxLogFiles = 7
	maxLogFiles := 0

	mergeConfigs(cfg, &rawConfig{
		Debug: rawDebugConfig{
			MaxLogFiles: &maxLogFiles,
		},
	})

	if cfg.Debug.MaxLogFiles != 7 {
		t.Fatalf("debug max log files = %d, want unchanged 7", cfg.Debug.MaxLogFiles)
	}
}

func TestEffectiveFilterPatternStripsComment(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "*.go ;; Go files", want: "*.go"},
		{in: "  *.{jpg,png}  ;; images ", want: "*.{jpg,png}"},
		{in: "*.txt", want: "*.txt"},
		{in: ";; comment only", want: ""},
	}

	for _, tt := range tests {
		if got := EffectiveFilterPattern(tt.in); got != tt.want {
			t.Fatalf("EffectiveFilterPattern(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMergeConfigs(t *testing.T) {
	defaultConfig := getDefaultConfig()
	trueVal := true
	falseVal := false
	sortBy := "size"
	sortOrder := "desc"
	border := "border"
	path := "/path/to/font.ttf"
	fontName := "Noto Sans CJK JP"
	monospacePath := "/path/to/mono.ttf"
	monospaceFontName := "UDEV Gothic"
	itemSpacing := 8
	preserveTimestamps := true
	viewerMaxWidth := 1200
	viewerMaxHeight := 900
	viewerDefaultPane := "text"
	zipNameEncoding := "cp437"
	imeEnabled := false
	fontSize := 16
	width := 1024
	height := 768
	thickness := 3
	colors := map[string]ThemeColorConfig{
		"cursor": {
			Dark: &ThemeColorValue{RGBA: [4]uint8{1, 2, 3, 4}, IsRGBA: true},
		},
	}

	fileConfig := &rawConfig{
		Window: rawWindowConfig{
			Width:  &width,
			Height: &height,
		},
		Theme: rawThemeConfig{
			Dark:              &falseVal,
			FontSize:          &fontSize,
			FontName:          &fontName,
			FontPath:          &path,
			MonospaceFontName: &monospaceFontName,
			MonospaceFontPath: &monospacePath,
			Colors:            colors,
		},
		UI: rawUIConfig{
			ShowHiddenFiles: &trueVal,
			Sort: rawSortConfig{
				SortBy:           &sortBy,
				SortOrder:        &sortOrder,
				DirectoriesFirst: &falseVal,
			},
			ItemSpacing: &itemSpacing,
			Copy: rawCopyConfig{
				PreserveTimestamps: &preserveTimestamps,
			},
			Viewer: rawViewerConfig{
				MaxWidth:    &viewerMaxWidth,
				MaxHeight:   &viewerMaxHeight,
				DefaultPane: &viewerDefaultPane,
			},
			Archive: rawArchiveConfig{
				ZipNameEncoding: &zipNameEncoding,
			},
			IME: rawIMEConfig{
				Enabled: &imeEnabled,
			},
			CursorStyle: rawCursorStyleConfig{
				Type:      &border,
				Thickness: &thickness,
			},
			DirectoryJumps: rawDirectoryJumpsConfig{
				Entries: []DirectoryJumpEntry{
					{Shortcut: "p", Directory: "/projects"},
					{Shortcut: "", Directory: "/tmp"},
					{Shortcut: "P", Directory: "/duplicate"},
				},
			},
			KeyBindings: []KeyBindingEntry{
				{Target: "fileViewer", Key: "X", Command: "fileViewer.pane.hex"},
			},
			ExternalCommands: []ExternalCommandEntry{
				{Name: "Open in editor", Extensions: []string{".go"}, Command: "vim", Args: []string{"{file}"}, Cwd: "{dir}"},
			},
		},
	}

	mergeConfigs(defaultConfig, fileConfig)

	// Check merged values
	if defaultConfig.Window.Width != 1024 {
		t.Errorf("Expected merged window width 1024, got %d", defaultConfig.Window.Width)
	}
	if defaultConfig.Window.Height != 768 {
		t.Errorf("Expected merged window height 768, got %d", defaultConfig.Window.Height)
	}
	if defaultConfig.Theme.Dark {
		t.Error("Expected merged theme to be light (false)")
	}
	if defaultConfig.Theme.FontSize != 16 {
		t.Errorf("Expected merged font size 16, got %d", defaultConfig.Theme.FontSize)
	}
	if defaultConfig.Theme.FontName != "Noto Sans CJK JP" {
		t.Errorf("Expected merged font name 'Noto Sans CJK JP', got '%s'", defaultConfig.Theme.FontName)
	}
	if defaultConfig.Theme.MonospaceFontName != "UDEV Gothic" {
		t.Errorf("Expected merged monospace font name 'UDEV Gothic', got '%s'", defaultConfig.Theme.MonospaceFontName)
	}
	if defaultConfig.Theme.MonospaceFontPath != "/path/to/mono.ttf" {
		t.Errorf("Expected merged monospace font path '/path/to/mono.ttf', got '%s'", defaultConfig.Theme.MonospaceFontPath)
	}
	if got := defaultConfig.Theme.Colors["cursor"].Dark.RGBA; got != [4]uint8{1, 2, 3, 4} {
		t.Errorf("Expected merged cursor dark color, got %+v", got)
	}
	if defaultConfig.UI.ShowHiddenFiles != true {
		t.Error("Expected merged ShowHiddenFiles to be true")
	}
	if defaultConfig.UI.Sort.SortBy != "size" {
		t.Errorf("Expected merged sort by 'size', got '%s'", defaultConfig.UI.Sort.SortBy)
	}
	if defaultConfig.UI.Sort.SortOrder != "desc" {
		t.Errorf("Expected merged sort order 'desc', got '%s'", defaultConfig.UI.Sort.SortOrder)
	}
	if defaultConfig.UI.Sort.DirectoriesFirst != false {
		t.Error("Expected merged DirectoriesFirst to be false")
	}
	if !defaultConfig.UI.Copy.PreserveTimestamps {
		t.Error("Expected merged copy preserve timestamps to be true")
	}
	if defaultConfig.UI.Viewer.MaxWidth != 1200 || defaultConfig.UI.Viewer.MaxHeight != 900 {
		t.Errorf("Expected merged viewer max size 1200x900, got %dx%d", defaultConfig.UI.Viewer.MaxWidth, defaultConfig.UI.Viewer.MaxHeight)
	}
	if defaultConfig.UI.Viewer.DefaultPane != "text" {
		t.Errorf("Expected merged viewer default pane text, got %q", defaultConfig.UI.Viewer.DefaultPane)
	}
	if defaultConfig.UI.Archive.ZipNameEncoding != "cp437" {
		t.Errorf("Expected merged ZIP name encoding 'cp437', got '%s'", defaultConfig.UI.Archive.ZipNameEncoding)
	}
	if defaultConfig.UI.IME.Enabled {
		t.Error("Expected merged IME integration to be disabled")
	}
	if defaultConfig.UI.CursorStyle.Type != "border" {
		t.Errorf("Expected merged cursor type 'border', got '%s'", defaultConfig.UI.CursorStyle.Type)
	}
	if len(defaultConfig.UI.DirectoryJumps.Entries) != 3 {
		t.Fatalf("Expected 3 directory jump entries, got %d", len(defaultConfig.UI.DirectoryJumps.Entries))
	}
	if defaultConfig.UI.DirectoryJumps.Entries[1].Shortcut != "" || defaultConfig.UI.DirectoryJumps.Entries[1].Directory != "/tmp" {
		t.Errorf("Expected directory jump order and empty shortcut to be preserved, got %+v", defaultConfig.UI.DirectoryJumps.Entries)
	}
	if len(defaultConfig.UI.KeyBindings) != 1 ||
		defaultConfig.UI.KeyBindings[0].Target != "fileViewer" ||
		defaultConfig.UI.KeyBindings[0].Key != "X" {
		t.Errorf("Expected key bindings to be merged, got %+v", defaultConfig.UI.KeyBindings)
	}
	if len(defaultConfig.UI.ExternalCommands) != 1 || defaultConfig.UI.ExternalCommands[0].Command != "vim" || defaultConfig.UI.ExternalCommands[0].Cwd != "{dir}" {
		t.Errorf("Expected external commands to be merged, got %+v", defaultConfig.UI.ExternalCommands)
	}
}

func TestThemeColorConfigUnmarshal(t *testing.T) {
	src := []byte(`{
		"theme": {
			"colors": {
				"cursor": [1, 2, 3, 4],
				"fileRegular": "foreground",
				"selectionBackground": {
					"dark": "selection",
					"light": [5, 6, 7, 8]
				},
				"busyOverlayBackground": {
					"value": [0, 0, 0, 96],
					"dark": null
				}
			}
		}
	}`)
	var raw rawConfig
	if err := json.Unmarshal(src, &raw); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	cfg := getDefaultConfig()
	mergeConfigs(cfg, &raw)

	if got := cfg.Theme.Colors["cursor"].Value.RGBA; got != [4]uint8{1, 2, 3, 4} {
		t.Fatalf("cursor color = %+v, want RGBA override", got)
	}
	if got := cfg.Theme.Colors["fileRegular"].Value.Name; got != "foreground" {
		t.Fatalf("fileRegular color = %q, want foreground", got)
	}
	if got := cfg.Theme.Colors["selectionBackground"].Dark.Name; got != "selection" {
		t.Fatalf("selection dark = %q, want selection", got)
	}
	if got := cfg.Theme.Colors["selectionBackground"].Light.RGBA; got != [4]uint8{5, 6, 7, 8} {
		t.Fatalf("selection light = %+v, want RGBA override", got)
	}
	if !cfg.Theme.Colors["busyOverlayBackground"].DarkDefault {
		t.Fatal("busy overlay dark override should be explicit default")
	}
}

func TestManagerInterface(t *testing.T) {
	// Test that Manager implements ManagerInterface
	// Note: Manager now requires debugPrint function
	dummyDebugPrint := func(format string, args ...interface{}) {}
	manager := NewManager(dummyDebugPrint)

	var managerInterface ManagerInterface = manager

	if managerInterface == nil {
		t.Error("Manager should implement ManagerInterface")
	}
}

func TestConfigSerialization(t *testing.T) {
	config := getDefaultConfig()

	// Test JSON marshaling
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaledConfig Config
	err = json.Unmarshal(data, &unmarshaledConfig)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Check that key values are preserved
	if config.Window.Width != unmarshaledConfig.Window.Width {
		t.Errorf("Window width not preserved: expected %d, got %d",
			config.Window.Width, unmarshaledConfig.Window.Width)
	}

	if config.UI.CursorStyle.Type != unmarshaledConfig.UI.CursorStyle.Type {
		t.Errorf("Cursor type not preserved: expected %s, got %s",
			config.UI.CursorStyle.Type, unmarshaledConfig.UI.CursorStyle.Type)
	}
}

func TestGetConfigPath(t *testing.T) {
	path := getConfigPath()

	// Should return a non-empty path
	if path == "" {
		t.Error("Config path should not be empty")
	}

	// Should end with config.json
	if !strings.HasSuffix(path, "config.json") {
		t.Errorf("Config path should end with 'config.json', got '%s'", path)
	}
}

func TestManagerLoadNonExistentFile(t *testing.T) {
	// Create a manager with a non-existent file path
	dummyDebugPrint := func(format string, args ...interface{}) {}
	manager := NewManager(dummyDebugPrint)
	manager.configPath = filepath.Join(os.TempDir(), "nonexistent", "config.json")

	config, err := manager.Load()

	// Should not return an error, but should return default config
	if err != nil {
		t.Errorf("Load should not return error for non-existent file, got: %v", err)
	}

	if config == nil {
		t.Error("Load should return default config for non-existent file")
	}

	// Should be default values
	if config.Window.Width != 800 {
		t.Errorf("Should return default config with width 800, got %d", config.Window.Width)
	}
}

func TestManagerLoadReadsHandWrittenConfigFile(t *testing.T) {
	// Manager is read-only now (no Save/SaveAsync/Flush/Close): write
	// config.json by hand and verify Load merges it with defaults.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	raw := `{
		"window": {"width": 1200, "height": 800},
		"theme": {"dark": false, "fontSize": 18},
		"ui": {
			"showHiddenFiles": true,
			"sort": {"sortBy": "modified", "sortOrder": "desc", "directoriesFirst": true},
			"cursorStyle": {"type": "background", "thickness": 5},
			"directoryJumps": {"entries": [
				{"shortcut": "p", "directory": "/projects"},
				{"shortcut": "", "directory": "/tmp"},
				{"shortcut": "P", "directory": "/duplicate"}
			]}
		}
	}`
	if err := os.WriteFile(configPath, []byte(raw), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	dummyDebugPrint := func(format string, args ...interface{}) {}
	manager := NewManager(dummyDebugPrint)
	manager.configPath = configPath

	loadedConfig, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify loaded values match the hand-written file (merged with defaults)
	if loadedConfig.Window.Width != 1200 {
		t.Errorf("Expected loaded width 1200, got %d", loadedConfig.Window.Width)
	}
	if loadedConfig.Theme.FontSize != 18 {
		t.Errorf("Expected loaded font size 18, got %d", loadedConfig.Theme.FontSize)
	}
	if loadedConfig.UI.ShowHiddenFiles != true {
		t.Error("Expected loaded ShowHiddenFiles to be true")
	}
	if len(loadedConfig.UI.DirectoryJumps.Entries) != 3 {
		t.Fatalf("Expected loaded directory jumps length 3, got %d", len(loadedConfig.UI.DirectoryJumps.Entries))
	}
	if loadedConfig.UI.DirectoryJumps.Entries[2].Shortcut != "P" || loadedConfig.UI.DirectoryJumps.Entries[2].Directory != "/duplicate" {
		t.Errorf("Expected loaded directory jumps to preserve order and value, got %+v", loadedConfig.UI.DirectoryJumps.Entries)
	}
}
