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

	// Test CursorStyle defaults
	if config.UI.CursorStyle.Type != "underline" {
		t.Errorf("Expected default cursor type 'underline', got '%s'", config.UI.CursorStyle.Type)
	}
	if config.UI.CursorStyle.Thickness != 2 {
		t.Errorf("Expected default cursor thickness 2, got %d", config.UI.CursorStyle.Thickness)
	}

	// Test CursorMemory defaults
	if config.UI.CursorMemory.MaxEntries != 100 {
		t.Errorf("Expected default cursor memory max entries 100, got %d", config.UI.CursorMemory.MaxEntries)
	}
	if config.UI.CursorMemory.Entries == nil {
		t.Error("Expected cursor memory entries to be initialized")
	}

	// Test NavigationHistory defaults
	if config.UI.NavigationHistory.MaxEntries != 50 {
		t.Errorf("Expected default navigation history max entries 50, got %d", config.UI.NavigationHistory.MaxEntries)
	}
	if config.UI.NavigationHistory.Entries == nil {
		t.Error("Expected navigation history entries to be initialized")
	}

	// Test FileFilter defaults
	if config.UI.FileFilter.MaxEntries != 30 {
		t.Errorf("Expected default file filter max entries 30, got %d", config.UI.FileFilter.MaxEntries)
	}
	if config.UI.FileFilter.Enabled {
		t.Error("Expected file filter to be disabled by default")
	}
	if config.UI.FileFilter.Current != nil {
		t.Error("Expected file filter current to be nil by default")
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

func TestMergeConfigs(t *testing.T) {
	defaultConfig := getDefaultConfig()
	trueVal := true
	falseVal := false
	sortBy := "size"
	sortOrder := "desc"
	border := "border"
	path := "/path/to/font.ttf"
	fontName := "Noto Sans CJK JP"
	itemSpacing := 8
	fontSize := 16
	width := 1024
	height := 768
	thickness := 3

	fileConfig := &rawConfig{
		Window: rawWindowConfig{
			Width:  &width,
			Height: &height,
		},
		Theme: rawThemeConfig{
			Dark:     &falseVal,
			FontSize: &fontSize,
			FontName: &fontName,
			FontPath: &path,
		},
		UI: rawUIConfig{
			ShowHiddenFiles: &trueVal,
			Sort: rawSortConfig{
				SortBy:           &sortBy,
				SortOrder:        &sortOrder,
				DirectoriesFirst: &falseVal,
			},
			ItemSpacing: &itemSpacing,
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
				{Key: "X", Command: "jobs.show"},
			},
			ExternalCommands: []ExternalCommandEntry{
				{Name: "Open in editor", Extensions: []string{".go"}, Command: "vim", Args: []string{"{file}"}},
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
	if defaultConfig.UI.CursorStyle.Type != "border" {
		t.Errorf("Expected merged cursor type 'border', got '%s'", defaultConfig.UI.CursorStyle.Type)
	}
	if len(defaultConfig.UI.DirectoryJumps.Entries) != 3 {
		t.Fatalf("Expected 3 directory jump entries, got %d", len(defaultConfig.UI.DirectoryJumps.Entries))
	}
	if defaultConfig.UI.DirectoryJumps.Entries[1].Shortcut != "" || defaultConfig.UI.DirectoryJumps.Entries[1].Directory != "/tmp" {
		t.Errorf("Expected directory jump order and empty shortcut to be preserved, got %+v", defaultConfig.UI.DirectoryJumps.Entries)
	}
	if len(defaultConfig.UI.KeyBindings) != 1 || defaultConfig.UI.KeyBindings[0].Key != "X" {
		t.Errorf("Expected key bindings to be merged, got %+v", defaultConfig.UI.KeyBindings)
	}
	if len(defaultConfig.UI.ExternalCommands) != 1 || defaultConfig.UI.ExternalCommands[0].Command != "vim" {
		t.Errorf("Expected external commands to be merged, got %+v", defaultConfig.UI.ExternalCommands)
	}
}

func TestManagerInterface(t *testing.T) {
	// Test that Manager implements ManagerInterface
	// Note: Manager now requires debugPrint function
	dummyDebugPrint := func(format string, args ...interface{}) {}
	manager := NewManager(dummyDebugPrint)
	manager.configPath = filepath.Join(os.TempDir(), "test_config.json")
	defer func() {
		if err := manager.Close(); err != nil {
			t.Fatalf("manager.Close failed: %v", err)
		}
	}()

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
	defer func() {
		if err := manager.Close(); err != nil {
			t.Fatalf("manager.Close failed: %v", err)
		}
	}()

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

func TestManagerSaveAndLoad(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	dummyDebugPrint := func(format string, args ...interface{}) {}
	manager := NewManager(dummyDebugPrint)
	manager.configPath = configPath
	defer func() {
		if err := manager.Close(); err != nil {
			t.Fatalf("manager.Close failed: %v", err)
		}
	}()

	// Create a test config
	testConfig := &Config{
		Window: WindowConfig{Width: 1200, Height: 800},
		Theme:  ThemeConfig{Dark: false, FontSize: 18},
		UI: UIConfig{
			ShowHiddenFiles: true,
			Sort: SortConfig{
				SortBy:           "modified",
				SortOrder:        "desc",
				DirectoriesFirst: true,
			},
			CursorStyle: CursorStyleConfig{
				Type:      "background",
				Thickness: 5,
			},
			DirectoryJumps: DirectoryJumpsConfig{
				Entries: []DirectoryJumpEntry{
					{Shortcut: "p", Directory: "/projects"},
					{Shortcut: "", Directory: "/tmp"},
					{Shortcut: "P", Directory: "/duplicate"},
				},
			},
		},
	}

	// Save the config synchronously
	if err := manager.Save(testConfig); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := manager.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load the config
	loadedConfig, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify loaded values match saved values (merged with defaults)
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

func TestCloneConfigDeepCopiesDirectoryJumps(t *testing.T) {
	cfg := getDefaultConfig()
	cfg.UI.DirectoryJumps.Entries = []DirectoryJumpEntry{
		{Shortcut: "p", Directory: "/projects"},
	}

	clone := cloneConfig(cfg)
	clone.UI.DirectoryJumps.Entries[0].Directory = "/changed"

	if cfg.UI.DirectoryJumps.Entries[0].Directory != "/projects" {
		t.Errorf("Expected original directory jump to remain unchanged, got %q", cfg.UI.DirectoryJumps.Entries[0].Directory)
	}
}
