package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Window  WindowConfig  `json:"window"`
	Startup StartupConfig `json:"startup"`
	Theme   ThemeConfig   `json:"theme"`
	Debug   DebugConfig   `json:"debug"`
	UI      UIConfig      `json:"ui"`
}

// rawConfig mirrors Config but uses pointer fields to detect presence in JSON.
type rawConfig struct {
	Window  rawWindowConfig  `json:"window"`
	Startup rawStartupConfig `json:"startup"`
	Theme   rawThemeConfig   `json:"theme"`
	Debug   rawDebugConfig   `json:"debug"`
	UI      rawUIConfig      `json:"ui"`
}

type rawWindowConfig struct {
	Width  *int `json:"width"`
	Height *int `json:"height"`
	X      *int `json:"x"`
	Y      *int `json:"y"`
}

type rawStartupConfig struct {
	Directory *string `json:"directory"`
}

type rawThemeConfig struct {
	Dark              *bool                       `json:"dark"`
	FontSize          *int                        `json:"fontSize"`
	FontName          *string                     `json:"fontName"`
	FontPath          *string                     `json:"fontPath"`
	MonospaceFontName *string                     `json:"monospaceFontName"`
	MonospaceFontPath *string                     `json:"monospaceFontPath"`
	Colors            map[string]ThemeColorConfig `json:"colors"`
}

type rawDebugConfig struct {
	Enabled      *bool   `json:"enabled"`
	LogDirectory *string `json:"logDirectory"`
	MaxLogFiles  *int    `json:"maxLogFiles"`
}

type rawUIConfig struct {
	ShowHiddenFiles   *bool                      `json:"showHiddenFiles"`
	Sort              rawSortConfig              `json:"sort"`
	ItemSpacing       *int                       `json:"itemSpacing"`
	Copy              rawCopyConfig              `json:"copy"`
	Viewer            rawViewerConfig            `json:"viewer"`
	Archive           rawArchiveConfig           `json:"archive"`
	IME               rawIMEConfig               `json:"ime"`
	CursorStyle       rawCursorStyleConfig       `json:"cursorStyle"`
	CursorMemory      rawCursorMemoryConfig      `json:"cursorMemory"`
	NavigationHistory rawNavigationHistoryConfig `json:"navigationHistory"`
	FileFilter        rawFileFilterConfig        `json:"fileFilter"`
	DirectoryJumps    rawDirectoryJumpsConfig    `json:"directoryJumps"`
	KeyBindings       []KeyBindingEntry          `json:"keyBindings"`
	ExternalCommands  []ExternalCommandEntry     `json:"externalCommands"`
}

type rawSortConfig struct {
	SortBy           *string `json:"sortBy"`
	SortOrder        *string `json:"sortOrder"`
	DirectoriesFirst *bool   `json:"directoriesFirst"`
}

type rawCopyConfig struct {
	PreserveTimestamps *bool `json:"preserveTimestamps"`
}

type rawViewerConfig struct {
	MaxWidth    *int    `json:"maxWidth"`
	MaxHeight   *int    `json:"maxHeight"`
	DefaultPane *string `json:"defaultPane"`
}

type rawCursorStyleConfig struct {
	Type      *string `json:"type"`
	Thickness *int    `json:"thickness"`
}

type rawIMEConfig struct {
	Enabled *bool `json:"enabled"`
}

type rawArchiveConfig struct {
	ZipNameEncoding *string `json:"zipNameEncoding"`
}

type rawCursorMemoryConfig struct {
	MaxEntries *int `json:"maxEntries"`
}

type rawNavigationHistoryConfig struct {
	MaxEntries *int `json:"maxEntries"`
}

type rawFileFilterConfig struct {
	MaxEntries *int `json:"maxEntries"`
}

type rawDirectoryJumpsConfig struct {
	Entries []DirectoryJumpEntry `json:"entries"`
}

// WindowConfig represents window-related settings
type WindowConfig struct {
	Width  int  `json:"width"`
	Height int  `json:"height"`
	X      *int `json:"x,omitempty"`
	Y      *int `json:"y,omitempty"`
}

// StartupConfig represents startup-related settings.
type StartupConfig struct {
	Directory string `json:"directory,omitempty"` // Starting directory used when no command-line path is supplied
}

// ThemeConfig represents theme-related settings
type ThemeConfig struct {
	Dark              bool                        `json:"dark"`
	FontSize          int                         `json:"fontSize"`
	FontName          string                      `json:"fontName"`
	FontPath          string                      `json:"fontPath"`
	MonospaceFontName string                      `json:"monospaceFontName,omitempty"`
	MonospaceFontPath string                      `json:"monospaceFontPath,omitempty"`
	Colors            map[string]ThemeColorConfig `json:"colors,omitempty"`
}

// DebugConfig controls persistent debug logging.
type DebugConfig struct {
	Enabled      bool   `json:"enabled"`      // Whether debug logging is enabled for normal startup
	LogDirectory string `json:"logDirectory"` // Empty means a logs directory next to config.json/init.star
	MaxLogFiles  int    `json:"maxLogFiles"`  // Maximum rotating session log files to retain
}

// ThemeColorValue is a color override expressed as either an RGBA tuple or a
// named color resolved by the theme package.
type ThemeColorValue struct {
	RGBA   [4]uint8
	Name   string
	IsRGBA bool
}

// ThemeColorConfig stores common and variant-specific overrides for an app
// color. Nil fields mean "use the built-in default".
type ThemeColorConfig struct {
	Value        *ThemeColorValue `json:"value,omitempty"`
	Dark         *ThemeColorValue `json:"dark,omitempty"`
	Light        *ThemeColorValue `json:"light,omitempty"`
	DarkDefault  bool             `json:"-"`
	LightDefault bool             `json:"-"`
}

// UnmarshalJSON accepts either a direct color value or an object with value,
// dark, and light fields.
func (c *ThemeColorConfig) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*c = ThemeColorConfig{}
		return nil
	}

	var direct ThemeColorValue
	if err := json.Unmarshal(data, &direct); err == nil {
		*c = ThemeColorConfig{Value: &direct}
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("theme color must be a name, RGBA array, or object: %w", err)
	}
	var parsed ThemeColorConfig
	if data, ok := raw["value"]; ok && string(data) != "null" {
		var value ThemeColorValue
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		parsed.Value = &value
	}
	if data, ok := raw["dark"]; ok {
		if string(data) == "null" {
			parsed.DarkDefault = true
		} else {
			var value ThemeColorValue
			if err := json.Unmarshal(data, &value); err != nil {
				return err
			}
			parsed.Dark = &value
		}
	}
	if data, ok := raw["light"]; ok {
		if string(data) == "null" {
			parsed.LightDefault = true
		} else {
			var value ThemeColorValue
			if err := json.Unmarshal(data, &value); err != nil {
				return err
			}
			parsed.Light = &value
		}
	}
	*c = parsed
	return nil
}

func (c ThemeColorConfig) MarshalJSON() ([]byte, error) {
	raw := make(map[string]interface{})
	if c.Value != nil {
		raw["value"] = c.Value
	}
	if c.DarkDefault {
		raw["dark"] = nil
	} else if c.Dark != nil {
		raw["dark"] = c.Dark
	}
	if c.LightDefault {
		raw["light"] = nil
	} else if c.Light != nil {
		raw["light"] = c.Light
	}
	return json.Marshal(raw)
}

func (v *ThemeColorValue) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = ThemeColorValue{}
		return nil
	}

	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("color name must not be empty")
		}
		*v = ThemeColorValue{Name: name}
		return nil
	}

	var rgba []uint8
	if err := json.Unmarshal(data, &rgba); err != nil {
		return fmt.Errorf("color value must be a name or RGBA array")
	}
	if len(rgba) != 4 {
		return fmt.Errorf("RGBA color array must have 4 elements")
	}
	*v = ThemeColorValue{RGBA: [4]uint8{rgba[0], rgba[1], rgba[2], rgba[3]}, IsRGBA: true}
	return nil
}

func (v ThemeColorValue) MarshalJSON() ([]byte, error) {
	if v.IsRGBA {
		return json.Marshal([]uint8{v.RGBA[0], v.RGBA[1], v.RGBA[2], v.RGBA[3]})
	}
	return json.Marshal(v.Name)
}

// UIConfig represents UI-related settings
type UIConfig struct {
	ShowHiddenFiles   bool                    `json:"showHiddenFiles"`
	Sort              SortConfig              `json:"sort"`
	ItemSpacing       int                     `json:"itemSpacing"`
	Copy              CopyConfig              `json:"copy"`
	Viewer            ViewerConfig            `json:"viewer"`
	Archive           ArchiveConfig           `json:"archive"`
	IME               IMEConfig               `json:"ime"`
	CursorStyle       CursorStyleConfig       `json:"cursorStyle"`
	CursorMemory      CursorMemoryConfig      `json:"cursorMemory"`
	NavigationHistory NavigationHistoryConfig `json:"navigationHistory"`
	FileFilter        FileFilterConfig        `json:"fileFilter"`
	DirectoryJumps    DirectoryJumpsConfig    `json:"directoryJumps"`
	KeyBindings       []KeyBindingEntry       `json:"keyBindings,omitempty"`
	ExternalCommands  []ExternalCommandEntry  `json:"externalCommands,omitempty"`
}

// IMEConfig controls platform IME integration behavior.
type IMEConfig struct {
	Enabled bool `json:"enabled"` // Whether to update native IME candidate/composition anchor positions
}

// ArchiveConfig controls archive virtual directory behavior.
type ArchiveConfig struct {
	ZipNameEncoding string `json:"zipNameEncoding"` // Fallback charset for non-UTF-8 ZIP entry names
}

// SortConfig represents file sorting settings
type SortConfig struct {
	SortBy           string `json:"sortBy"`           // "name", "size", "modified", "extension"
	SortOrder        string `json:"sortOrder"`        // "asc", "desc"
	DirectoriesFirst bool   `json:"directoriesFirst"` // Whether to show directories before files
}

// CopyConfig controls copy operation defaults.
type CopyConfig struct {
	PreserveTimestamps bool `json:"preserveTimestamps"` // Default for preserving file and directory modified times
}

// ViewerConfig controls the built-in file viewer dialog.
type ViewerConfig struct {
	MaxWidth    int    `json:"maxWidth"`    // Optional maximum dialog width; 0 means uncapped
	MaxHeight   int    `json:"maxHeight"`   // Optional maximum dialog height; 0 means uncapped
	DefaultPane string `json:"defaultPane"` // "auto", "text", "markdown", or "hex"
}

// CursorStyleConfig represents cursor appearance settings
type CursorStyleConfig struct {
	Type      string `json:"type"`      // "underline", "border", "background", "icon", "font"
	Thickness int    `json:"thickness"` // Line thickness for underline/border
}

// CursorMemoryConfig represents cursor position memory settings. The actual
// remembered positions live in state.json (see State.CursorMemory); this is
// just the user-configured entry limit.
type CursorMemoryConfig struct {
	MaxEntries int `json:"maxEntries"` // Maximum number of directories to remember
}

// NavigationHistoryConfig represents navigation history settings. The actual
// history data lives in state.json (see State.NavigationHistory); this is
// just the user-configured entry limit.
type NavigationHistoryConfig struct {
	MaxEntries int `json:"maxEntries"` // Maximum number of paths to remember
}

// FilterEntry represents a single filter pattern with metadata
type FilterEntry struct {
	Pattern  string    `json:"pattern"`  // Doublestar glob pattern
	LastUsed time.Time `json:"lastUsed"` // Last usage timestamp
	UseCount int       `json:"useCount"` // Usage frequency counter
}

// FileFilterConfig represents file filter settings. The actual filter history
// and currently applied filter live in state.json (see State.FileFilter);
// this is just the user-configured entry limit.
type FileFilterConfig struct {
	MaxEntries int `json:"maxEntries"` // Maximum number of filter patterns to remember
}

// DirectoryJumpEntry represents a configured directory jump target.
type DirectoryJumpEntry struct {
	Shortcut  string `json:"shortcut"`  // Empty or shortcut prefix, matched case-insensitively
	Directory string `json:"directory"` // Directory path as written in config.json
}

// DirectoryJumpsConfig represents manually configured directory jump targets.
type DirectoryJumpsConfig struct {
	Entries []DirectoryJumpEntry `json:"entries"` // Entries are displayed by shortcut sort order
}

// KeyBindingEntry maps a key specification to an internal command.
type KeyBindingEntry struct {
	Target  string `json:"target,omitempty"` // Empty/main, lineEdit, or fileViewer
	Key     string `json:"key"`              // Forms: "Key", "S-Key", "A-Key", "C-Key"
	Command string `json:"command"`          // Stable internal command ID
	Event   string `json:"event,omitempty"`  // Deprecated: ignored. Bindings fire on key activation (typed/shortcut).
}

// ExternalCommandEntry describes a command that can be run for matching files.
type ExternalCommandEntry struct {
	Name       string   `json:"name"`                 // Menu label
	Key        string   `json:"key,omitempty"`        // Optional single-key menu accelerator
	Extensions []string `json:"extensions,omitempty"` // Case-insensitive, with or without dot; "*" matches all files
	Command    string   `json:"command"`              // Executable path or command name
	Args       []string `json:"args,omitempty"`       // Supports {file}, {files}, {all_files}, {dir}, {name}
	Cwd        string   `json:"cwd,omitempty"`        // Optional working directory; supports {file}, {dir}, {name}
	Edit       bool     `json:"edit,omitempty"`       // Confirm and edit the final command line before running
}

// Manager loads configuration from config.json. config.json is treated as
// read-only application state: runtime state that used to be saved back into
// it (cursor memory, navigation history, file filter history, last-applied
// sort) now lives in state.json, managed separately by StateManager.
type Manager struct {
	configPath string
	debugPrint func(format string, args ...interface{})
}

// NewManager creates a new configuration manager
func NewManager(debugPrint func(format string, args ...interface{})) *Manager {
	return &Manager{
		configPath: getConfigPath(),
		debugPrint: debugPrint,
	}
}

// ConfigPath returns the full config.json path used by this manager.
func (m *Manager) ConfigPath() string {
	return m.configPath
}

// Load loads configuration from file and merges with defaults
func (m *Manager) Load() (*Config, error) {
	// Start with default configuration
	config := getDefaultConfig()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		m.debugPrint("Config: Config file not found, using defaults: %v", err)
		return config, nil
	}

	// Parse config file into a temporary config
	var fileConfig rawConfig
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Merge file config with defaults
	mergeConfigs(config, &fileConfig)
	return config, nil
}

func cloneThemeColorConfig(src ThemeColorConfig) ThemeColorConfig {
	clone := ThemeColorConfig{
		DarkDefault:  src.DarkDefault,
		LightDefault: src.LightDefault,
	}
	if src.Value != nil {
		value := *src.Value
		clone.Value = &value
	}
	if src.Dark != nil {
		dark := *src.Dark
		clone.Dark = &dark
	}
	if src.Light != nil {
		light := *src.Light
		clone.Light = &light
	}
	return clone
}

// Default returns the default configuration without reading any file.
func Default() *Config {
	return getDefaultConfig()
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *Config {
	return &Config{
		Window: WindowConfig{
			Width:  800,
			Height: 600,
		},
		Startup: StartupConfig{
			Directory: "",
		},
		Theme: ThemeConfig{
			Dark:              true,
			FontSize:          14,
			FontName:          "",
			FontPath:          "",
			MonospaceFontName: "",
			MonospaceFontPath: "",
		},
		Debug: DebugConfig{
			Enabled:      false,
			LogDirectory: "",
			MaxLogFiles:  10,
		},
		UI: UIConfig{
			ShowHiddenFiles: false,
			Sort: SortConfig{
				SortBy:           "name",
				SortOrder:        "asc",
				DirectoriesFirst: true,
			},
			ItemSpacing: 4,
			Copy: CopyConfig{
				PreserveTimestamps: false,
			},
			Viewer: ViewerConfig{
				MaxWidth:    0,
				MaxHeight:   0,
				DefaultPane: "auto",
			},
			Archive: ArchiveConfig{
				ZipNameEncoding: "shift_jis",
			},
			IME: IMEConfig{
				Enabled: true,
			},
			CursorStyle: CursorStyleConfig{
				Type:      "underline",
				Thickness: 2,
			},
			CursorMemory: CursorMemoryConfig{
				MaxEntries: 100,
			},
			NavigationHistory: NavigationHistoryConfig{
				MaxEntries: 10000,
			},
			FileFilter: FileFilterConfig{
				MaxEntries: 30,
			},
			DirectoryJumps: DirectoryJumpsConfig{
				Entries: make([]DirectoryJumpEntry, 0),
			},
			KeyBindings:      make([]KeyBindingEntry, 0),
			ExternalCommands: make([]ExternalCommandEntry, 0),
		},
	}
}

// getConfigPath returns the path to the configuration file following OS conventions
func getConfigPath() string {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		// Windows: %APPDATA%\nekomimist\nmf\config.json
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "config.json"
			}
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		configDir = filepath.Join(appData, "nekomimist", "nmf")

	case "darwin":
		// macOS: ~/Library/Application Support/nekomimist/nmf/config.json
		home, err := os.UserHomeDir()
		if err != nil {
			return "config.json"
		}
		configDir = filepath.Join(home, "Library", "Application Support", "nekomimist", "nmf")

	default:
		// Linux/Unix: $XDG_CONFIG_HOME/nekomimist/nmf/config.json or ~/.config/nekomimist/nmf/config.json
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "config.json"
			}
			xdgConfigHome = filepath.Join(home, ".config")
		}
		configDir = filepath.Join(xdgConfigHome, "nekomimist", "nmf")
	}

	return filepath.Join(configDir, "config.json")
}

// mergeConfigs merges file config values into default config
func mergeConfigs(defaultConfig *Config, fileConfig *rawConfig) {
	// Merge Window config
	if fileConfig.Window.Width != nil {
		defaultConfig.Window.Width = *fileConfig.Window.Width
	}
	if fileConfig.Window.Height != nil {
		defaultConfig.Window.Height = *fileConfig.Window.Height
	}
	if fileConfig.Window.X != nil {
		x := *fileConfig.Window.X
		defaultConfig.Window.X = &x
	}
	if fileConfig.Window.Y != nil {
		y := *fileConfig.Window.Y
		defaultConfig.Window.Y = &y
	}

	// Merge Startup config
	if fileConfig.Startup.Directory != nil {
		defaultConfig.Startup.Directory = strings.TrimSpace(*fileConfig.Startup.Directory)
	}

	// Merge Theme config
	if fileConfig.Theme.Dark != nil {
		defaultConfig.Theme.Dark = *fileConfig.Theme.Dark
	}
	if fileConfig.Theme.FontSize != nil {
		defaultConfig.Theme.FontSize = *fileConfig.Theme.FontSize
	}
	if fileConfig.Theme.FontName != nil {
		defaultConfig.Theme.FontName = strings.TrimSpace(*fileConfig.Theme.FontName)
	}
	if fileConfig.Theme.FontPath != nil && *fileConfig.Theme.FontPath != "" {
		defaultConfig.Theme.FontPath = *fileConfig.Theme.FontPath
	}
	if fileConfig.Theme.MonospaceFontName != nil {
		defaultConfig.Theme.MonospaceFontName = strings.TrimSpace(*fileConfig.Theme.MonospaceFontName)
	}
	if fileConfig.Theme.MonospaceFontPath != nil && *fileConfig.Theme.MonospaceFontPath != "" {
		defaultConfig.Theme.MonospaceFontPath = *fileConfig.Theme.MonospaceFontPath
	}
	if fileConfig.Theme.Colors != nil {
		defaultConfig.Theme.Colors = make(map[string]ThemeColorConfig, len(fileConfig.Theme.Colors))
		for k, v := range fileConfig.Theme.Colors {
			defaultConfig.Theme.Colors[strings.TrimSpace(k)] = cloneThemeColorConfig(v)
		}
	}

	// Merge Debug config
	if fileConfig.Debug.Enabled != nil {
		defaultConfig.Debug.Enabled = *fileConfig.Debug.Enabled
	}
	if fileConfig.Debug.LogDirectory != nil {
		defaultConfig.Debug.LogDirectory = strings.TrimSpace(*fileConfig.Debug.LogDirectory)
	}
	if fileConfig.Debug.MaxLogFiles != nil && *fileConfig.Debug.MaxLogFiles > 0 {
		defaultConfig.Debug.MaxLogFiles = *fileConfig.Debug.MaxLogFiles
	}

	// Merge UI config
	if fileConfig.UI.ShowHiddenFiles != nil {
		defaultConfig.UI.ShowHiddenFiles = *fileConfig.UI.ShowHiddenFiles
	}
	if fileConfig.UI.Sort.SortBy != nil && *fileConfig.UI.Sort.SortBy != "" {
		defaultConfig.UI.Sort.SortBy = *fileConfig.UI.Sort.SortBy
	}
	if fileConfig.UI.Sort.SortOrder != nil && *fileConfig.UI.Sort.SortOrder != "" {
		defaultConfig.UI.Sort.SortOrder = *fileConfig.UI.Sort.SortOrder
	}
	if fileConfig.UI.Sort.DirectoriesFirst != nil {
		defaultConfig.UI.Sort.DirectoriesFirst = *fileConfig.UI.Sort.DirectoriesFirst
	}
	if fileConfig.UI.ItemSpacing != nil && *fileConfig.UI.ItemSpacing != 0 {
		defaultConfig.UI.ItemSpacing = *fileConfig.UI.ItemSpacing
	}
	if fileConfig.UI.Copy.PreserveTimestamps != nil {
		defaultConfig.UI.Copy.PreserveTimestamps = *fileConfig.UI.Copy.PreserveTimestamps
	}
	if fileConfig.UI.Viewer.MaxWidth != nil && *fileConfig.UI.Viewer.MaxWidth >= 0 {
		defaultConfig.UI.Viewer.MaxWidth = *fileConfig.UI.Viewer.MaxWidth
	}
	if fileConfig.UI.Viewer.MaxHeight != nil && *fileConfig.UI.Viewer.MaxHeight >= 0 {
		defaultConfig.UI.Viewer.MaxHeight = *fileConfig.UI.Viewer.MaxHeight
	}
	if fileConfig.UI.Viewer.DefaultPane != nil {
		if pane := normalizeViewerDefaultPane(*fileConfig.UI.Viewer.DefaultPane); pane != "" {
			defaultConfig.UI.Viewer.DefaultPane = pane
		}
	}
	if fileConfig.UI.Archive.ZipNameEncoding != nil && strings.TrimSpace(*fileConfig.UI.Archive.ZipNameEncoding) != "" {
		defaultConfig.UI.Archive.ZipNameEncoding = strings.TrimSpace(*fileConfig.UI.Archive.ZipNameEncoding)
	}
	if fileConfig.UI.IME.Enabled != nil {
		defaultConfig.UI.IME.Enabled = *fileConfig.UI.IME.Enabled
	}

	// Merge CursorStyle config
	if fileConfig.UI.CursorStyle.Type != nil && *fileConfig.UI.CursorStyle.Type != "" {
		defaultConfig.UI.CursorStyle.Type = *fileConfig.UI.CursorStyle.Type
	}
	if fileConfig.UI.CursorStyle.Thickness != nil && *fileConfig.UI.CursorStyle.Thickness != 0 {
		defaultConfig.UI.CursorStyle.Thickness = *fileConfig.UI.CursorStyle.Thickness
	}

	// Merge CursorMemory config
	if fileConfig.UI.CursorMemory.MaxEntries != nil && *fileConfig.UI.CursorMemory.MaxEntries != 0 {
		defaultConfig.UI.CursorMemory.MaxEntries = *fileConfig.UI.CursorMemory.MaxEntries
	}

	// Merge NavigationHistory config
	if fileConfig.UI.NavigationHistory.MaxEntries != nil && *fileConfig.UI.NavigationHistory.MaxEntries != 0 {
		defaultConfig.UI.NavigationHistory.MaxEntries = *fileConfig.UI.NavigationHistory.MaxEntries
	}

	// Merge FileFilter config
	if fileConfig.UI.FileFilter.MaxEntries != nil && *fileConfig.UI.FileFilter.MaxEntries != 0 {
		defaultConfig.UI.FileFilter.MaxEntries = *fileConfig.UI.FileFilter.MaxEntries
	}

	// Merge DirectoryJumps config
	if fileConfig.UI.DirectoryJumps.Entries != nil {
		defaultConfig.UI.DirectoryJumps.Entries = fileConfig.UI.DirectoryJumps.Entries
	}
	if fileConfig.UI.KeyBindings != nil {
		defaultConfig.UI.KeyBindings = fileConfig.UI.KeyBindings
	}
	if fileConfig.UI.ExternalCommands != nil {
		defaultConfig.UI.ExternalCommands = fileConfig.UI.ExternalCommands
	}
}

func normalizeViewerDefaultPane(pane string) string {
	switch strings.ToLower(strings.TrimSpace(pane)) {
	case "auto", "text", "markdown", "hex":
		return strings.ToLower(strings.TrimSpace(pane))
	default:
		return ""
	}
}

// frecencyScore, sortNavigationHistoryEntries, sortFileFilterEntries,
// EffectiveFilterPattern, and stopTimer moved to state.go: after the
// config.json/state.json split, Config no longer has any use for them (the
// history/filter accessors that called them now live on *State), and
// stopTimer's only remaining caller is StateManager's save worker.
