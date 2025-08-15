package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Config represents the application configuration
type Config struct {
	Window WindowConfig `json:"window"`
	Theme  ThemeConfig  `json:"theme"`
	UI     UIConfig     `json:"ui"`
}

// WindowConfig represents window-related settings
type WindowConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ThemeConfig represents theme-related settings
type ThemeConfig struct {
	Dark     bool   `json:"dark"`
	FontSize int    `json:"fontSize"`
	FontPath string `json:"fontPath"`
}

// UIConfig represents UI-related settings
type UIConfig struct {
	ShowHiddenFiles   bool                    `json:"showHiddenFiles"`
	Sort              SortConfig              `json:"sort"`
	ItemSpacing       int                     `json:"itemSpacing"`
	CursorStyle       CursorStyleConfig       `json:"cursorStyle"`
	CursorMemory      CursorMemoryConfig      `json:"cursorMemory"`
	NavigationHistory NavigationHistoryConfig `json:"navigationHistory"`
	FileFilter        FileFilterConfig        `json:"fileFilter"`
}

// SortConfig represents file sorting settings
type SortConfig struct {
	SortBy           string `json:"sortBy"`           // "name", "size", "modified", "extension"
	SortOrder        string `json:"sortOrder"`        // "asc", "desc"
	DirectoriesFirst bool   `json:"directoriesFirst"` // Whether to show directories before files
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
	Entries    []string             `json:"entries"`    // Path history (newest first)
	LastUsed   map[string]time.Time `json:"lastUsed"`   // LRU management
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

// Manager provides configuration management functionality
type Manager struct {
	configPath string
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		configPath: getConfigPath(),
	}
}

// Load loads configuration from file and merges with defaults
func (m *Manager) Load() (*Config, error) {
	// Start with default configuration
	config := getDefaultConfig()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		log.Printf("Config file not found, using defaults: %v", err)
		return config, nil
	}

	// Parse config file into a temporary config
	var fileConfig Config
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Merge file config with defaults
	mergeConfigs(config, &fileConfig)
	return config, nil
}

// Save saves configuration to file
func (m *Manager) Save(config *Config) error {
	// Create the config directory if it doesn't exist
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
			FontPath: "",
		},
		UI: UIConfig{
			ShowHiddenFiles: false,
			Sort: SortConfig{
				SortBy:           "name",
				SortOrder:        "asc",
				DirectoriesFirst: true,
			},
			ItemSpacing: 4,
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
				MaxEntries: 50,
				Entries:    make([]string, 0),
				LastUsed:   make(map[string]time.Time),
			},
			FileFilter: FileFilterConfig{
				MaxEntries: 30,
				Entries:    make([]FilterEntry, 0),
				Enabled:    false,
				Current:    nil,
			},
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
func mergeConfigs(defaultConfig *Config, fileConfig *Config) {
	// Merge Window config
	if fileConfig.Window.Width != 0 {
		defaultConfig.Window.Width = fileConfig.Window.Width
	}
	if fileConfig.Window.Height != 0 {
		defaultConfig.Window.Height = fileConfig.Window.Height
	}

	// Merge Theme config
	// Note: for bool values, we can't distinguish between false and unset, so we always use file value
	defaultConfig.Theme.Dark = fileConfig.Theme.Dark
	if fileConfig.Theme.FontSize != 0 {
		defaultConfig.Theme.FontSize = fileConfig.Theme.FontSize
	}
	if fileConfig.Theme.FontPath != "" {
		defaultConfig.Theme.FontPath = fileConfig.Theme.FontPath
	}

	// Merge UI config
	defaultConfig.UI.ShowHiddenFiles = fileConfig.UI.ShowHiddenFiles
	if fileConfig.UI.Sort.SortBy != "" {
		defaultConfig.UI.Sort.SortBy = fileConfig.UI.Sort.SortBy
	}
	if fileConfig.UI.Sort.SortOrder != "" {
		defaultConfig.UI.Sort.SortOrder = fileConfig.UI.Sort.SortOrder
	}
	// DirectoriesFirst is a boolean, so we merge it directly
	defaultConfig.UI.Sort.DirectoriesFirst = fileConfig.UI.Sort.DirectoriesFirst
	if fileConfig.UI.ItemSpacing != 0 {
		defaultConfig.UI.ItemSpacing = fileConfig.UI.ItemSpacing
	}

	// Merge CursorStyle config
	if fileConfig.UI.CursorStyle.Type != "" {
		defaultConfig.UI.CursorStyle.Type = fileConfig.UI.CursorStyle.Type
	}
	if fileConfig.UI.CursorStyle.Thickness != 0 {
		defaultConfig.UI.CursorStyle.Thickness = fileConfig.UI.CursorStyle.Thickness
	}

	// Merge CursorMemory config
	if fileConfig.UI.CursorMemory.MaxEntries != 0 {
		defaultConfig.UI.CursorMemory.MaxEntries = fileConfig.UI.CursorMemory.MaxEntries
	}
	if fileConfig.UI.CursorMemory.Entries != nil {
		defaultConfig.UI.CursorMemory.Entries = fileConfig.UI.CursorMemory.Entries
	}
	if fileConfig.UI.CursorMemory.LastUsed != nil {
		defaultConfig.UI.CursorMemory.LastUsed = fileConfig.UI.CursorMemory.LastUsed
	}

	// Merge NavigationHistory config
	if fileConfig.UI.NavigationHistory.MaxEntries != 0 {
		defaultConfig.UI.NavigationHistory.MaxEntries = fileConfig.UI.NavigationHistory.MaxEntries
	}
	if fileConfig.UI.NavigationHistory.Entries != nil {
		defaultConfig.UI.NavigationHistory.Entries = fileConfig.UI.NavigationHistory.Entries
	}
	if fileConfig.UI.NavigationHistory.LastUsed != nil {
		defaultConfig.UI.NavigationHistory.LastUsed = fileConfig.UI.NavigationHistory.LastUsed
	}

	// Merge FileFilter config
	if fileConfig.UI.FileFilter.MaxEntries != 0 {
		defaultConfig.UI.FileFilter.MaxEntries = fileConfig.UI.FileFilter.MaxEntries
	}
	if fileConfig.UI.FileFilter.Entries != nil {
		defaultConfig.UI.FileFilter.Entries = fileConfig.UI.FileFilter.Entries
	}
	// Always merge Enabled and Current states
	defaultConfig.UI.FileFilter.Enabled = fileConfig.UI.FileFilter.Enabled
	if fileConfig.UI.FileFilter.Current != nil {
		defaultConfig.UI.FileFilter.Current = fileConfig.UI.FileFilter.Current
	}
}

// AddToNavigationHistory adds a path to navigation history
func (c *Config) AddToNavigationHistory(path string) {
	now := time.Now()

	// Remove existing entry if it exists
	for i, entry := range c.UI.NavigationHistory.Entries {
		if entry == path {
			c.UI.NavigationHistory.Entries = append(
				c.UI.NavigationHistory.Entries[:i],
				c.UI.NavigationHistory.Entries[i+1:]...,
			)
			break
		}
	}

	// Add to beginning of slice (newest first)
	c.UI.NavigationHistory.Entries = append([]string{path}, c.UI.NavigationHistory.Entries...)

	// Update last used time
	c.UI.NavigationHistory.LastUsed[path] = now

	// Enforce max entries limit
	if len(c.UI.NavigationHistory.Entries) > c.UI.NavigationHistory.MaxEntries {
		// Remove oldest entry
		oldestPath := c.UI.NavigationHistory.Entries[c.UI.NavigationHistory.MaxEntries]
		c.UI.NavigationHistory.Entries = c.UI.NavigationHistory.Entries[:c.UI.NavigationHistory.MaxEntries]
		delete(c.UI.NavigationHistory.LastUsed, oldestPath)
	}
}

// GetNavigationHistory returns the navigation history entries sorted by last used time (newest first)
func (c *Config) GetNavigationHistory() []string {
	entries := c.UI.NavigationHistory.Entries
	if len(entries) <= 1 {
		return entries
	}

	// Create a copy to avoid modifying the original
	sorted := make([]string, len(entries))
	copy(sorted, entries)

	// Sort by last used time (newest first)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			timeI := c.UI.NavigationHistory.LastUsed[sorted[i]]
			timeJ := c.UI.NavigationHistory.LastUsed[sorted[j]]

			// If timeJ is newer than timeI, swap
			if timeJ.After(timeI) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
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
