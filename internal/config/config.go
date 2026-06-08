package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Config represents the application configuration
type Config struct {
	Window WindowConfig `json:"window"`
	Theme  ThemeConfig  `json:"theme"`
	Debug  DebugConfig  `json:"debug"`
	UI     UIConfig     `json:"ui"`
}

// rawConfig mirrors Config but uses pointer fields to detect presence in JSON.
type rawConfig struct {
	Window rawWindowConfig `json:"window"`
	Theme  rawThemeConfig  `json:"theme"`
	Debug  rawDebugConfig  `json:"debug"`
	UI     rawUIConfig     `json:"ui"`
}

type rawWindowConfig struct {
	Width  *int `json:"width"`
	Height *int `json:"height"`
}

type rawThemeConfig struct {
	Dark     *bool                       `json:"dark"`
	FontSize *int                        `json:"fontSize"`
	FontName *string                     `json:"fontName"`
	FontPath *string                     `json:"fontPath"`
	Colors   map[string]ThemeColorConfig `json:"colors"`
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
	MaxWidth  *int `json:"maxWidth"`
	MaxHeight *int `json:"maxHeight"`
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
	MaxEntries *int                 `json:"maxEntries"`
	Entries    map[string]string    `json:"entries"`
	LastUsed   map[string]time.Time `json:"lastUsed"`
}

type rawNavigationHistoryConfig struct {
	MaxEntries *int                 `json:"maxEntries"`
	Entries    []string             `json:"entries"`
	LastUsed   map[string]time.Time `json:"lastUsed"`
	UseCount   map[string]int       `json:"useCount"`
	Pinned     []string             `json:"pinned"`
}

type rawFileFilterConfig struct {
	MaxEntries *int          `json:"maxEntries"`
	Entries    []FilterEntry `json:"entries"`
	Enabled    *bool         `json:"enabled"`
	Current    *FilterEntry  `json:"current"`
}

type rawDirectoryJumpsConfig struct {
	Entries []DirectoryJumpEntry `json:"entries"`
}

// WindowConfig represents window-related settings
type WindowConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ThemeConfig represents theme-related settings
type ThemeConfig struct {
	Dark     bool                        `json:"dark"`
	FontSize int                         `json:"fontSize"`
	FontName string                      `json:"fontName"`
	FontPath string                      `json:"fontPath"`
	Colors   map[string]ThemeColorConfig `json:"colors,omitempty"`
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
	MaxWidth  int `json:"maxWidth"`  // Optional maximum dialog width; 0 means uncapped
	MaxHeight int `json:"maxHeight"` // Optional maximum dialog height; 0 means uncapped
}

// CursorStyleConfig represents cursor appearance settings
type CursorStyleConfig struct {
	Type      string `json:"type"`      // "underline", "border", "background", "icon", "font"
	Thickness int    `json:"thickness"` // Line thickness for underline/border
}

// CursorMemoryConfig represents cursor position memory settings
type CursorMemoryConfig struct {
	MaxEntries int                  `json:"maxEntries"` // Maximum number of directories to remember
	Entries    map[string]string    `json:"entries"`    // key: dirPath, value: fileName
	LastUsed   map[string]time.Time `json:"lastUsed"`   // LRU management
}

// NavigationHistoryConfig represents navigation history settings
type NavigationHistoryConfig struct {
	MaxEntries int                  `json:"maxEntries"` // Maximum number of paths to remember
	Entries    []string             `json:"entries"`    // Path history (frecency order)
	LastUsed   map[string]time.Time `json:"lastUsed"`   // Last usage timestamp
	UseCount   map[string]int       `json:"useCount"`   // Usage frequency counter
	Pinned     []string             `json:"pinned"`     // Saved paths that do not count against history pruning
}

// FilterEntry represents a single filter pattern with metadata
type FilterEntry struct {
	Pattern  string    `json:"pattern"`  // Doublestar glob pattern
	LastUsed time.Time `json:"lastUsed"` // Last usage timestamp
	UseCount int       `json:"useCount"` // Usage frequency counter
}

// FileFilterConfig represents file filter settings
type FileFilterConfig struct {
	MaxEntries int           `json:"maxEntries"` // Maximum number of filter patterns to remember
	Entries    []FilterEntry `json:"entries"`    // Filter history (most recent first)
	Enabled    bool          `json:"enabled"`    // Current filter enabled state
	Current    *FilterEntry  `json:"current"`    // Currently applied filter pattern
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
	Key     string `json:"key"`             // Forms: "Key", "S-Key", "A-Key", "C-Key"
	Command string `json:"command"`         // Stable internal command ID
	Event   string `json:"event,omitempty"` // Optional: "typed", "down", or "up"
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

// Manager provides configuration management functionality
type Manager struct {
	configPath string
	debugPrint func(format string, args ...interface{})

	saveTransformMu sync.RWMutex
	saveTransform   SaveTransform

	saveDelay     time.Duration
	saveRequests  chan *Config
	flushRequests chan chan error
	closeRequests chan chan error
	stopped       chan struct{}
}

// SaveTransform can adjust a config snapshot before it is persisted.
type SaveTransform func(*Config) *Config

// ErrManagerClosed is returned when operations are attempted after Close has been called.
var ErrManagerClosed = errors.New("config manager closed")

// NewManager creates a new configuration manager
func NewManager(debugPrint func(format string, args ...interface{})) *Manager {
	m := &Manager{
		configPath:    getConfigPath(),
		debugPrint:    debugPrint,
		saveDelay:     500 * time.Millisecond,
		saveRequests:  make(chan *Config, 1),
		flushRequests: make(chan chan error),
		closeRequests: make(chan chan error),
		stopped:       make(chan struct{}),
	}

	m.startWorker()
	return m
}

// ConfigPath returns the full config.json path used by this manager.
func (m *Manager) ConfigPath() string {
	return m.configPath
}

// SetSaveTransform installs a hook used to adjust snapshots before saving.
func (m *Manager) SetSaveTransform(transform SaveTransform) {
	m.saveTransformMu.Lock()
	defer m.saveTransformMu.Unlock()
	m.saveTransform = transform
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

// Save saves configuration to file
func (m *Manager) Save(config *Config) error {
	if err := m.Flush(); err != nil && !errors.Is(err, ErrManagerClosed) {
		return err
	}

	configCopy := m.prepareForSave(config)
	return m.saveConfig(configCopy)
}

// SaveAsync schedules the configuration to be saved after a short debounce window.
func (m *Manager) SaveAsync(config *Config) error {
	select {
	case <-m.stopped:
		return ErrManagerClosed
	default:
	}

	configCopy := m.prepareForSave(config)

	select {
	case m.saveRequests <- configCopy:
		return nil
	case <-m.stopped:
		return ErrManagerClosed
	}
}

func (m *Manager) prepareForSave(config *Config) *Config {
	configCopy := cloneConfig(config)

	m.saveTransformMu.RLock()
	transform := m.saveTransform
	m.saveTransformMu.RUnlock()
	if transform == nil {
		return configCopy
	}
	transformed := transform(configCopy)
	if transformed == nil {
		return configCopy
	}
	return transformed
}

// Flush forces any pending asynchronous save to be written immediately.
func (m *Manager) Flush() error {
	select {
	case <-m.stopped:
		return ErrManagerClosed
	default:
	}

	reply := make(chan error, 1)
	select {
	case m.flushRequests <- reply:
	case <-m.stopped:
		return ErrManagerClosed
	}

	return <-reply
}

// Close flushes pending writes and stops the background worker.
func (m *Manager) Close() error {
	select {
	case <-m.stopped:
		return nil
	default:
	}

	reply := make(chan error, 1)
	select {
	case m.closeRequests <- reply:
	case <-m.stopped:
		return nil
	}

	return <-reply
}

func (m *Manager) startWorker() {
	go m.saveWorker()
}

func (m *Manager) saveWorker() {
	var pending *Config
	var timer *time.Timer

	for {
		var timerChan <-chan time.Time
		if timer != nil {
			timerChan = timer.C
		}

		select {
		case cfg := <-m.saveRequests:
			pending = cfg
			if timer == nil {
				timer = time.NewTimer(m.saveDelay)
			} else {
				stopTimer(timer)
				timer.Reset(m.saveDelay)
			}
		case <-timerChan:
			if pending != nil {
				if err := m.saveConfig(pending); err != nil {
					m.debugPrint("Config: Error saving config: %v", err)
				}
				pending = nil
			}
			timer = nil
		case reply := <-m.flushRequests:
			stopTimer(timer)
			timer = nil
			var err error
			if pending != nil {
				err = m.saveConfig(pending)
				pending = nil
			}
			reply <- err
		case reply := <-m.closeRequests:
			stopTimer(timer)
			timer = nil
			var err error
			if pending != nil {
				err = m.saveConfig(pending)
				pending = nil
			}
			reply <- err
			close(m.stopped)
			return
		}
	}
}

func (m *Manager) saveConfig(config *Config) error {
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

func stopTimer(t *time.Timer) {
	if t == nil {
		return
	}
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

func cloneConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}

	copyConfig := *cfg
	copyConfig.Theme = cloneThemeConfig(cfg.Theme)
	copyConfig.UI = cloneUIConfig(cfg.UI)
	return &copyConfig
}

// Clone returns a deep copy of cfg suitable for independent mutation.
func Clone(cfg *Config) *Config {
	return cloneConfig(cfg)
}

func cloneThemeConfig(src ThemeConfig) ThemeConfig {
	clone := src
	if src.Colors != nil {
		clone.Colors = make(map[string]ThemeColorConfig, len(src.Colors))
		for k, v := range src.Colors {
			clone.Colors[k] = cloneThemeColorConfig(v)
		}
	}
	return clone
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

func cloneUIConfig(src UIConfig) UIConfig {
	clone := src
	clone.CursorMemory = cloneCursorMemoryConfig(src.CursorMemory)
	clone.NavigationHistory = cloneNavigationHistoryConfig(src.NavigationHistory)
	clone.FileFilter = cloneFileFilterConfig(src.FileFilter)
	clone.DirectoryJumps = cloneDirectoryJumpsConfig(src.DirectoryJumps)
	clone.KeyBindings = cloneKeyBindingEntries(src.KeyBindings)
	clone.ExternalCommands = cloneExternalCommandEntries(src.ExternalCommands)
	return clone
}

func cloneCursorMemoryConfig(src CursorMemoryConfig) CursorMemoryConfig {
	clone := src
	if src.Entries != nil {
		clone.Entries = make(map[string]string, len(src.Entries))
		for k, v := range src.Entries {
			clone.Entries[k] = v
		}
	}
	if src.LastUsed != nil {
		clone.LastUsed = make(map[string]time.Time, len(src.LastUsed))
		for k, v := range src.LastUsed {
			clone.LastUsed[k] = v
		}
	}
	return clone
}

func cloneNavigationHistoryConfig(src NavigationHistoryConfig) NavigationHistoryConfig {
	clone := src
	if src.Entries != nil {
		clone.Entries = make([]string, len(src.Entries))
		copy(clone.Entries, src.Entries)
	}
	if src.LastUsed != nil {
		clone.LastUsed = make(map[string]time.Time, len(src.LastUsed))
		for k, v := range src.LastUsed {
			clone.LastUsed[k] = v
		}
	}
	if src.UseCount != nil {
		clone.UseCount = make(map[string]int, len(src.UseCount))
		for k, v := range src.UseCount {
			clone.UseCount[k] = v
		}
	}
	if src.Pinned != nil {
		clone.Pinned = make([]string, len(src.Pinned))
		copy(clone.Pinned, src.Pinned)
	}
	return clone
}

func cloneFileFilterConfig(src FileFilterConfig) FileFilterConfig {
	clone := src
	if src.Entries != nil {
		clone.Entries = make([]FilterEntry, len(src.Entries))
		copy(clone.Entries, src.Entries)
	}
	if src.Current != nil {
		currentCopy := *src.Current
		clone.Current = &currentCopy
	}
	return clone
}

func cloneDirectoryJumpsConfig(src DirectoryJumpsConfig) DirectoryJumpsConfig {
	clone := src
	if src.Entries != nil {
		clone.Entries = make([]DirectoryJumpEntry, len(src.Entries))
		copy(clone.Entries, src.Entries)
	}
	return clone
}

func cloneKeyBindingEntries(src []KeyBindingEntry) []KeyBindingEntry {
	if src == nil {
		return nil
	}
	clone := make([]KeyBindingEntry, len(src))
	copy(clone, src)
	return clone
}

func cloneExternalCommandEntries(src []ExternalCommandEntry) []ExternalCommandEntry {
	if src == nil {
		return nil
	}
	clone := make([]ExternalCommandEntry, len(src))
	for i, entry := range src {
		clone[i] = entry
		if entry.Extensions != nil {
			clone[i].Extensions = make([]string, len(entry.Extensions))
			copy(clone[i].Extensions, entry.Extensions)
		}
		if entry.Args != nil {
			clone[i].Args = make([]string, len(entry.Args))
			copy(clone[i].Args, entry.Args)
		}
	}
	return clone
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *Config {
	return &Config{
		Window: WindowConfig{
			Width:  800,
			Height: 600,
		},
		Theme: ThemeConfig{
			Dark:     true,
			FontSize: 14,
			FontName: "",
			FontPath: "",
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
				MaxWidth:  0,
				MaxHeight: 0,
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
				Entries:    make(map[string]string),
				LastUsed:   make(map[string]time.Time),
			},
			NavigationHistory: NavigationHistoryConfig{
				MaxEntries: 10000,
				Entries:    make([]string, 0),
				LastUsed:   make(map[string]time.Time),
				UseCount:   make(map[string]int),
				Pinned:     make([]string, 0),
			},
			FileFilter: FileFilterConfig{
				MaxEntries: 30,
				Entries:    make([]FilterEntry, 0),
				Enabled:    false,
				Current:    nil,
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
	if fileConfig.UI.CursorMemory.Entries != nil {
		defaultConfig.UI.CursorMemory.Entries = fileConfig.UI.CursorMemory.Entries
	}
	if fileConfig.UI.CursorMemory.LastUsed != nil {
		defaultConfig.UI.CursorMemory.LastUsed = fileConfig.UI.CursorMemory.LastUsed
	}

	// Merge NavigationHistory config
	if fileConfig.UI.NavigationHistory.MaxEntries != nil && *fileConfig.UI.NavigationHistory.MaxEntries != 0 {
		defaultConfig.UI.NavigationHistory.MaxEntries = *fileConfig.UI.NavigationHistory.MaxEntries
	}
	if fileConfig.UI.NavigationHistory.Entries != nil {
		defaultConfig.UI.NavigationHistory.Entries = fileConfig.UI.NavigationHistory.Entries
	}
	if fileConfig.UI.NavigationHistory.LastUsed != nil {
		defaultConfig.UI.NavigationHistory.LastUsed = fileConfig.UI.NavigationHistory.LastUsed
	}
	if fileConfig.UI.NavigationHistory.UseCount != nil {
		defaultConfig.UI.NavigationHistory.UseCount = fileConfig.UI.NavigationHistory.UseCount
	}
	if fileConfig.UI.NavigationHistory.Pinned != nil {
		defaultConfig.UI.NavigationHistory.Pinned = fileConfig.UI.NavigationHistory.Pinned
	}
	ensureNavigationHistoryStats(&defaultConfig.UI.NavigationHistory)
	ensureNavigationHistoryPinned(&defaultConfig.UI.NavigationHistory)

	// Merge FileFilter config
	if fileConfig.UI.FileFilter.MaxEntries != nil && *fileConfig.UI.FileFilter.MaxEntries != 0 {
		defaultConfig.UI.FileFilter.MaxEntries = *fileConfig.UI.FileFilter.MaxEntries
	}
	if fileConfig.UI.FileFilter.Entries != nil {
		defaultConfig.UI.FileFilter.Entries = fileConfig.UI.FileFilter.Entries
	}
	if fileConfig.UI.FileFilter.Enabled != nil {
		defaultConfig.UI.FileFilter.Enabled = *fileConfig.UI.FileFilter.Enabled
	}
	if fileConfig.UI.FileFilter.Current != nil {
		copyEntry := *fileConfig.UI.FileFilter.Current
		defaultConfig.UI.FileFilter.Current = &copyEntry
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

// AddToNavigationHistory adds a path to navigation history
func (c *Config) AddToNavigationHistory(path string) {
	now := time.Now()
	history := &c.UI.NavigationHistory
	ensureNavigationHistoryStats(history)

	found := false
	for _, entry := range history.Entries {
		if entry == path {
			found = true
			break
		}
	}
	if !found {
		history.Entries = append(history.Entries, path)
	}

	history.LastUsed[path] = now
	history.UseCount[path]++
	sortNavigationHistory(history, now)
	pruneNavigationHistory(history, now)
}

// GetNavigationHistory returns the navigation history entries sorted by frecency.
func (c *Config) GetNavigationHistory() []string {
	history := &c.UI.NavigationHistory
	ensureNavigationHistoryStats(history)
	entries := history.Entries
	if len(entries) <= 1 {
		return entries
	}

	sorted := make([]string, len(entries))
	copy(sorted, entries)
	sortNavigationHistoryEntries(sorted, history.LastUsed, history.UseCount, time.Now())

	return sorted
}

func ensureNavigationHistoryStats(history *NavigationHistoryConfig) {
	if history.LastUsed == nil {
		history.LastUsed = make(map[string]time.Time)
	}
	if history.UseCount == nil {
		history.UseCount = make(map[string]int)
	}
	for _, path := range history.Entries {
		if _, ok := history.UseCount[path]; !ok {
			history.UseCount[path] = 1
		}
	}
}

func ensureNavigationHistoryPinned(history *NavigationHistoryConfig) {
	if history.Pinned == nil {
		history.Pinned = make([]string, 0)
	}
}

// PinNavigationPath saves a path for History Jump without adding it to prunable history.
func (c *Config) PinNavigationPath(path string) bool {
	if path == "" {
		return false
	}
	history := &c.UI.NavigationHistory
	ensureNavigationHistoryPinned(history)
	for _, entry := range history.Pinned {
		if entry == path {
			return false
		}
	}
	history.Pinned = append(history.Pinned, path)
	return true
}

// UnpinNavigationPath removes a saved History Jump path.
func (c *Config) UnpinNavigationPath(path string) bool {
	history := &c.UI.NavigationHistory
	ensureNavigationHistoryPinned(history)
	for i, entry := range history.Pinned {
		if entry == path {
			history.Pinned = append(history.Pinned[:i], history.Pinned[i+1:]...)
			return true
		}
	}
	return false
}

// IsNavigationPathPinned reports whether a path is saved for History Jump.
func (c *Config) IsNavigationPathPinned(path string) bool {
	history := &c.UI.NavigationHistory
	ensureNavigationHistoryPinned(history)
	for _, entry := range history.Pinned {
		if entry == path {
			return true
		}
	}
	return false
}

func sortNavigationHistory(history *NavigationHistoryConfig, now time.Time) {
	sortNavigationHistoryEntries(history.Entries, history.LastUsed, history.UseCount, now)
}

func sortNavigationHistoryEntries(entries []string, lastUsed map[string]time.Time, useCount map[string]int, now time.Time) {
	sort.SliceStable(entries, func(i, j int) bool {
		scoreI := frecencyScore(useCount[entries[i]], lastUsed[entries[i]], now)
		scoreJ := frecencyScore(useCount[entries[j]], lastUsed[entries[j]], now)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		timeI := lastUsed[entries[i]]
		timeJ := lastUsed[entries[j]]
		if !timeI.Equal(timeJ) {
			return timeI.After(timeJ)
		}
		return entries[i] < entries[j]
	})
}

func pruneNavigationHistory(history *NavigationHistoryConfig, now time.Time) {
	if history.MaxEntries <= 0 || len(history.Entries) <= history.MaxEntries {
		return
	}
	sortNavigationHistory(history, now)
	for _, path := range history.Entries[history.MaxEntries:] {
		delete(history.LastUsed, path)
		delete(history.UseCount, path)
	}
	history.Entries = history.Entries[:history.MaxEntries]
}

func frecencyScore(useCount int, lastUsed time.Time, now time.Time) float64 {
	if useCount <= 0 {
		useCount = 1
	}
	age := now.Sub(lastUsed)
	switch {
	case lastUsed.IsZero():
		return float64(useCount) * 0.25
	case age <= time.Hour:
		return float64(useCount) * 4
	case age <= 24*time.Hour:
		return float64(useCount) * 2
	case age <= 7*24*time.Hour:
		return float64(useCount) * 0.5
	default:
		return float64(useCount) * 0.25
	}
}

// EffectiveFilterPattern returns the glob portion of a saved filter entry.
// Text after ";;" is treated as a searchable user comment.
func EffectiveFilterPattern(pattern string) string {
	if idx := strings.Index(pattern, ";;"); idx >= 0 {
		pattern = pattern[:idx]
	}
	return strings.TrimSpace(pattern)
}

// GetFileFilterEntries returns filter history sorted by frecency.
func (c *Config) GetFileFilterEntries() []FilterEntry {
	entries := c.UI.FileFilter.Entries
	if len(entries) <= 1 {
		return entries
	}
	sorted := make([]FilterEntry, len(entries))
	copy(sorted, entries)
	sortFileFilterEntries(sorted, time.Now())
	return sorted
}

// AddToFileFilterHistory records a filter pattern use and prunes by frecency.
func (c *Config) AddToFileFilterHistory(entry *FilterEntry) {
	if entry == nil || strings.TrimSpace(entry.Pattern) == "" || EffectiveFilterPattern(entry.Pattern) == "" {
		return
	}
	filter := &c.UI.FileFilter
	now := time.Now()
	for i := range filter.Entries {
		if filter.Entries[i].Pattern == entry.Pattern {
			filter.Entries[i].LastUsed = now
			filter.Entries[i].UseCount++
			sortFileFilterEntries(filter.Entries, now)
			pruneFileFilterEntries(filter, now)
			return
		}
	}

	newEntry := *entry
	newEntry.LastUsed = now
	if newEntry.UseCount <= 0 {
		newEntry.UseCount = 1
	}
	filter.Entries = append(filter.Entries, newEntry)
	sortFileFilterEntries(filter.Entries, now)
	pruneFileFilterEntries(filter, now)
}

// RemoveFileFilterEntry removes an exact saved filter pattern from history.
func (c *Config) RemoveFileFilterEntry(pattern string) bool {
	entries := c.UI.FileFilter.Entries
	for i := range entries {
		if entries[i].Pattern == pattern {
			c.UI.FileFilter.Entries = append(entries[:i], entries[i+1:]...)
			return true
		}
	}
	return false
}

func sortFileFilterEntries(entries []FilterEntry, now time.Time) {
	sort.SliceStable(entries, func(i, j int) bool {
		scoreI := frecencyScore(entries[i].UseCount, entries[i].LastUsed, now)
		scoreJ := frecencyScore(entries[j].UseCount, entries[j].LastUsed, now)
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		if !entries[i].LastUsed.Equal(entries[j].LastUsed) {
			return entries[i].LastUsed.After(entries[j].LastUsed)
		}
		return entries[i].Pattern < entries[j].Pattern
	})
}

func pruneFileFilterEntries(filter *FileFilterConfig, now time.Time) {
	if filter.MaxEntries <= 0 || len(filter.Entries) <= filter.MaxEntries {
		return
	}
	sortFileFilterEntries(filter.Entries, now)
	filter.Entries = filter.Entries[:filter.MaxEntries]
}

// FilterNavigationHistory filters history entries by query (case-insensitive partial match)
func (c *Config) FilterNavigationHistory(query string) []string {
	if query == "" {
		return c.UI.NavigationHistory.Entries
	}

	query = strings.ToLower(query)
	var filtered []string

	for _, path := range c.UI.NavigationHistory.Entries {
		if strings.Contains(strings.ToLower(path), query) {
			filtered = append(filtered, path)
		}
	}

	return filtered
}
