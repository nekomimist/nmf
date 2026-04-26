package config

import (
	"encoding/json"
	"errors"
	"fmt"
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

// rawConfig mirrors Config but uses pointer fields to detect presence in JSON.
type rawConfig struct {
	Window rawWindowConfig `json:"window"`
	Theme  rawThemeConfig  `json:"theme"`
	UI     rawUIConfig     `json:"ui"`
}

type rawWindowConfig struct {
	Width  *int `json:"width"`
	Height *int `json:"height"`
}

type rawThemeConfig struct {
	Dark     *bool   `json:"dark"`
	FontSize *int    `json:"fontSize"`
	FontPath *string `json:"fontPath"`
}

type rawUIConfig struct {
	ShowHiddenFiles   *bool                      `json:"showHiddenFiles"`
	Sort              rawSortConfig              `json:"sort"`
	ItemSpacing       *int                       `json:"itemSpacing"`
	CursorStyle       rawCursorStyleConfig       `json:"cursorStyle"`
	CursorMemory      rawCursorMemoryConfig      `json:"cursorMemory"`
	NavigationHistory rawNavigationHistoryConfig `json:"navigationHistory"`
	FileFilter        rawFileFilterConfig        `json:"fileFilter"`
	DirectoryJumps    rawDirectoryJumpsConfig    `json:"directoryJumps"`
}

type rawSortConfig struct {
	SortBy           *string `json:"sortBy"`
	SortOrder        *string `json:"sortOrder"`
	DirectoriesFirst *bool   `json:"directoriesFirst"`
}

type rawCursorStyleConfig struct {
	Type      *string `json:"type"`
	Thickness *int    `json:"thickness"`
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
	DirectoryJumps    DirectoryJumpsConfig    `json:"directoryJumps"`
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

// DirectoryJumpEntry represents a configured directory jump target.
type DirectoryJumpEntry struct {
	Shortcut  string `json:"shortcut"`  // Empty or one character, matched case-insensitively
	Directory string `json:"directory"` // Directory path as written in config.json
}

// DirectoryJumpsConfig represents manually configured directory jump targets.
type DirectoryJumpsConfig struct {
	Entries []DirectoryJumpEntry `json:"entries"` // Config order is display order
}

// Manager provides configuration management functionality
type Manager struct {
	configPath string
	debugPrint func(format string, args ...interface{})

	saveDelay     time.Duration
	saveRequests  chan *Config
	flushRequests chan chan error
	closeRequests chan chan error
	stopped       chan struct{}
}

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

	configCopy := cloneConfig(config)
	return m.saveConfig(configCopy)
}

// SaveAsync schedules the configuration to be saved after a short debounce window.
func (m *Manager) SaveAsync(config *Config) error {
	select {
	case <-m.stopped:
		return ErrManagerClosed
	default:
	}

	configCopy := cloneConfig(config)

	select {
	case m.saveRequests <- configCopy:
		return nil
	case <-m.stopped:
		return ErrManagerClosed
	}
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
	copyConfig.UI = cloneUIConfig(cfg.UI)
	return &copyConfig
}

func cloneUIConfig(src UIConfig) UIConfig {
	clone := src
	clone.CursorMemory = cloneCursorMemoryConfig(src.CursorMemory)
	clone.NavigationHistory = cloneNavigationHistoryConfig(src.NavigationHistory)
	clone.FileFilter = cloneFileFilterConfig(src.FileFilter)
	clone.DirectoryJumps = cloneDirectoryJumpsConfig(src.DirectoryJumps)
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
			DirectoryJumps: DirectoryJumpsConfig{
				Entries: make([]DirectoryJumpEntry, 0),
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
	if fileConfig.Theme.FontPath != nil && *fileConfig.Theme.FontPath != "" {
		defaultConfig.Theme.FontPath = *fileConfig.Theme.FontPath
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
