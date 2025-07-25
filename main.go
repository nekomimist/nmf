package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	ShowHiddenFiles bool   `json:"showHiddenFiles"`
	SortBy          string `json:"sortBy"`
	ItemSpacing     int    `json:"itemSpacing"`
}

// FileInfo represents a file or directory
type FileInfo struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	Modified time.Time
}

// FileManager is the main file manager struct
type FileManager struct {
	window      fyne.Window
	currentPath string
	files       []FileInfo
	fileList    *widget.List
	pathLabel   *widget.Label
	selectedIdx int
	fileBinding binding.UntypedList
	config      *Config
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
			SortBy:          "name",
			ItemSpacing:     4,
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

// loadConfig loads configuration from file or returns default
func loadConfig() *Config {
	configPath := getConfigPath()

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Config file not found, using defaults: %v", err)
		return getDefaultConfig()
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("Error parsing config file, using defaults: %v", err)
		return getDefaultConfig()
	}

	return &config
}

// saveConfig saves configuration to file
func (config *Config) saveConfig() error {
	configPath := getConfigPath()

	// Create the config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %v", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config: %v", err)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	return nil
}

// CustomTheme implements fyne.Theme with configurable font settings
type CustomTheme struct {
	config     *Config
	customFont fyne.Resource
}

func NewCustomTheme(config *Config) *CustomTheme {
	theme := &CustomTheme{config: config}

	// Load custom font if specified
	if config.Theme.FontPath != "" {
		theme.loadCustomFont()
	}

	return theme
}

// loadCustomFont loads a custom font from the specified path
func (t *CustomTheme) loadCustomFont() {
	fontPath := t.config.Theme.FontPath

	// Check if font file exists
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		log.Printf("Custom font file not found: %s", fontPath)
		return
	}

	// Read font file
	fontData, err := ioutil.ReadFile(fontPath)
	if err != nil {
		log.Printf("Error reading font file %s: %v", fontPath, err)
		return
	}

	// Create font resource
	t.customFont = fyne.NewStaticResource(filepath.Base(fontPath), fontData)
	log.Printf("Loaded custom font: %s", fontPath)
}

// Color methods from default theme
func (t *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.config.Theme.Dark {
		return theme.DarkTheme().Color(name, variant)
	}
	return theme.DefaultTheme().Color(name, variant)
}

// Icon methods from default theme
func (t *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	if t.config.Theme.Dark {
		return theme.DarkTheme().Icon(name)
	}
	return theme.DefaultTheme().Icon(name)
}

// Font method with custom font support
func (t *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	// Return custom font if loaded and available
	if t.customFont != nil {
		return t.customFont
	}

	if t.config.Theme.Dark {
		return theme.DarkTheme().Font(style)
	}
	return theme.DefaultTheme().Font(style)
}

// Size method with custom font size and spacing support
func (t *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText && t.config.Theme.FontSize > 0 {
		return float32(t.config.Theme.FontSize)
	}

	// Custom item spacing support
	if t.config.UI.ItemSpacing > 0 {
		switch name {
		case theme.SizeNamePadding:
			return float32(t.config.UI.ItemSpacing)
		case theme.SizeNameInnerPadding:
			return float32(t.config.UI.ItemSpacing * 2)
		}
	}

	if t.config.Theme.Dark {
		return theme.DarkTheme().Size(name)
	}
	return theme.DefaultTheme().Size(name)
}

func NewFileManager(app fyne.App, path string, config *Config) *FileManager {
	fm := &FileManager{
		window:      app.NewWindow("File Manager"),
		currentPath: path,
		selectedIdx: -1,
		fileBinding: binding.NewUntypedList(),
		config:      config,
	}

	fm.setupUI()
	fm.loadDirectory(path)

	// Start file system watcher
	go fm.watchDirectory()

	return fm
}

func (fm *FileManager) setupUI() {
	// Path label
	fm.pathLabel = widget.NewLabel(fm.currentPath)
	fm.pathLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Create file list
	fm.fileList = widget.NewListWithData(
		fm.fileBinding,
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.FolderIcon())
			name := widget.NewLabel("filename")
			info := widget.NewLabel("info")

			// Left side: icon + name
			leftSide := container.NewHBox(
				container.NewPadded(icon),
				name,
			)

			// Use border container to align name left and info right
			return container.NewBorder(
				nil, nil, leftSide, info, nil,
			)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			fileItem := item.(binding.Untyped)
			file, _ := fileItem.Get()
			fileInfo := file.(FileInfo)

			// obj is a border container
			border := obj.(*fyne.Container)

			// In a border container created with NewBorder(top, bottom, left, right, center),
			// the objects are stored as [top, bottom, left, right, center] but some may be nil
			// We need to find our leftSide and info widgets
			var leftSide *fyne.Container
			var infoLabel *widget.Label

			for _, obj := range border.Objects {
				if obj == nil {
					continue
				}
				if container, ok := obj.(*fyne.Container); ok {
					// This should be our leftSide container
					leftSide = container
				} else if label, ok := obj.(*widget.Label); ok {
					// This should be our info label
					infoLabel = label
				}
			}

			if leftSide != nil && infoLabel != nil {
				iconPadded := leftSide.Objects[0].(*fyne.Container)
				icon := iconPadded.Objects[0].(*widget.Icon)
				nameLabel := leftSide.Objects[1].(*widget.Label)

				if fileInfo.IsDir {
					icon.SetResource(theme.FolderIcon())
				} else {
					icon.SetResource(theme.FileIcon())
				}

				nameLabel.SetText(fileInfo.Name)

				if fileInfo.IsDir {
					infoLabel.SetText(fmt.Sprintf("<dir> %s %s",
						fileInfo.Modified.Format("2006-01-02"),
						fileInfo.Modified.Format("15:04:05")))
				} else {
					infoLabel.SetText(fmt.Sprintf("%s %s %s",
						formatFileSize(fileInfo.Size),
						fileInfo.Modified.Format("2006-01-02"),
						fileInfo.Modified.Format("15:04:05")))
				}
			}
		},
	)

	// Hide separators for compact spacing if itemSpacing is small
	if fm.config.UI.ItemSpacing <= 2 {
		fm.fileList.HideSeparators = true
	}

	// Handle double-click
	fm.fileList.OnSelected = func(id widget.ListItemID) {
		fm.selectedIdx = id
		if id >= 0 && id < len(fm.files) {
			file := fm.files[id]
			if file.IsDir {
				fm.loadDirectory(file.Path)
			}
		}
	}

	// Create toolbar
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.NavigateBackIcon(), func() {
			parent := filepath.Dir(fm.currentPath)
			if parent != fm.currentPath {
				fm.loadDirectory(parent)
			}
		}),
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			home, _ := os.UserHomeDir()
			fm.loadDirectory(home)
		}),
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() {
			fm.loadDirectory(fm.currentPath)
		}),
		widget.NewToolbarAction(theme.FolderNewIcon(), func() {
			fm.openNewWindow()
		}),
	)

	// Layout
	content := container.NewBorder(
		container.NewVBox(toolbar, fm.pathLabel),
		nil, nil, nil,
		fm.fileList,
	)

	fm.window.SetContent(content)
	fm.window.Resize(fyne.NewSize(float32(fm.config.Window.Width), float32(fm.config.Window.Height)))

	// Keyboard shortcuts
	fm.window.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		case fyne.KeyUp:
			if fm.selectedIdx > 0 {
				fm.fileList.Select(fm.selectedIdx - 1)
			}
		case fyne.KeyDown:
			if fm.selectedIdx < len(fm.files)-1 {
				fm.fileList.Select(fm.selectedIdx + 1)
			}
		case fyne.KeyReturn:
			if fm.selectedIdx >= 0 && fm.selectedIdx < len(fm.files) {
				file := fm.files[fm.selectedIdx]
				if file.IsDir {
					fm.loadDirectory(file.Path)
				}
			}
		case fyne.KeyBackspace:
			parent := filepath.Dir(fm.currentPath)
			if parent != fm.currentPath {
				fm.loadDirectory(parent)
			}
		}
	})
}

func (fm *FileManager) loadDirectory(path string) {
	fm.currentPath = path
	fm.pathLabel.SetText(path)
	fm.files = []FileInfo{}

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return
	}

	// Convert to FileInfo
	items := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileInfo := FileInfo{
			Name:     entry.Name(),
			Path:     filepath.Join(path, entry.Name()),
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
		}

		fm.files = append(fm.files, fileInfo)
		items = append(items, fileInfo)
	}

	// Update binding
	fm.fileBinding.Set(items)
	fm.selectedIdx = -1
}

func (fm *FileManager) watchDirectory() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastModTime := time.Now()

	for range ticker.C {
		info, err := os.Stat(fm.currentPath)
		if err != nil {
			continue
		}

		if info.ModTime().After(lastModTime) {
			lastModTime = info.ModTime()
			fm.loadDirectory(fm.currentPath)
		}
	}
}

func (fm *FileManager) openNewWindow() {
	newFM := NewFileManager(fyne.CurrentApp(), fm.currentPath, fm.config)
	newFM.window.Show()
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func main() {
	// Load configuration
	config := loadConfig()

	app := app.New()

	// Apply custom theme
	customTheme := NewCustomTheme(config)
	app.Settings().SetTheme(customTheme)

	// Parse command line arguments for starting directory
	var startPath string
	if len(os.Args) > 1 {
		// Use provided path argument
		startPath = os.Args[1]

		// Validate the path exists and is a directory
		if info, err := os.Stat(startPath); err != nil {
			log.Fatalf("Error accessing path '%s': %v", startPath, err)
		} else if !info.IsDir() {
			log.Fatalf("Path '%s' is not a directory", startPath)
		}

		// Convert to absolute path
		if absPath, err := filepath.Abs(startPath); err == nil {
			startPath = absPath
		}
	} else {
		// Default to current working directory
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current directory: %v", err)
		}
		startPath = pwd
	}

	fm := NewFileManager(app, startPath, config)
	fm.window.ShowAndRun()
}
