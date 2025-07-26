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
	"fyne.io/fyne/v2/canvas"
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
	ShowHiddenFiles bool              `json:"showHiddenFiles"`
	SortBy          string            `json:"sortBy"`
	ItemSpacing     int               `json:"itemSpacing"`
	CursorStyle     CursorStyleConfig `json:"cursorStyle"`
}

// CursorStyleConfig represents cursor appearance settings
type CursorStyleConfig struct {
	Type      string   `json:"type"`      // "underline", "border", "background", "icon", "font"
	Color     [4]uint8 `json:"color"`     // RGBA color values
	Thickness int      `json:"thickness"` // Line thickness for underline/border
}

// FileInfo represents a file or directory
type FileInfo struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	Modified time.Time
}

// ListItem wraps FileInfo with index for rendering
type ListItem struct {
	Index    int
	FileInfo FileInfo
}

// CursorRenderer interface for different cursor display styles
type CursorRenderer interface {
	RenderCursor(bounds fyne.Size, textBounds fyne.Position, config CursorStyleConfig) fyne.CanvasObject
}

// UnderlineCursorRenderer renders cursor as an underline
type UnderlineCursorRenderer struct{}

func (r *UnderlineCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config CursorStyleConfig) fyne.CanvasObject {
	underline := canvas.NewRectangle(color.RGBA{
		R: config.Color[0],
		G: config.Color[1],
		B: config.Color[2],
		A: config.Color[3],
	})

	thickness := float32(config.Thickness)
	if thickness <= 0 {
		thickness = 2
	}

	// Calculate text area dimensions (rough estimation)
	textWidth := bounds.Width * 0.8           // Most of the width is text
	textHeight := bounds.Height * 0.8         // Most of the height is text
	textY := (bounds.Height - textHeight) / 2 // Center vertically

	// Position at the bottom of text
	underline.Resize(fyne.NewSize(textWidth, thickness))
	underline.Move(fyne.NewPos(textBounds.X, textY+textHeight-thickness))

	return underline
}

// BorderCursorRenderer renders cursor as a border
type BorderCursorRenderer struct{}

func (r *BorderCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config CursorStyleConfig) fyne.CanvasObject {
	borderColor := color.RGBA{
		R: config.Color[0],
		G: config.Color[1],
		B: config.Color[2],
		A: config.Color[3],
	}

	thickness := float32(config.Thickness)
	if thickness <= 0 {
		thickness = 1
	}

	// Create border using multiple rectangles
	top := canvas.NewRectangle(borderColor)
	top.Resize(fyne.NewSize(bounds.Width, thickness))
	top.Move(fyne.NewPos(0, 0))

	bottom := canvas.NewRectangle(borderColor)
	bottom.Resize(fyne.NewSize(bounds.Width, thickness))
	bottom.Move(fyne.NewPos(0, bounds.Height-thickness))

	left := canvas.NewRectangle(borderColor)
	left.Resize(fyne.NewSize(thickness, bounds.Height))
	left.Move(fyne.NewPos(0, 0))

	right := canvas.NewRectangle(borderColor)
	right.Resize(fyne.NewSize(thickness, bounds.Height))
	right.Move(fyne.NewPos(bounds.Width-thickness, 0))

	return container.NewWithoutLayout(top, bottom, left, right)
}

// BackgroundCursorRenderer renders cursor as background highlight
type BackgroundCursorRenderer struct{}

func (r *BackgroundCursorRenderer) RenderCursor(bounds fyne.Size, textBounds fyne.Position, config CursorStyleConfig) fyne.CanvasObject {
	background := canvas.NewRectangle(color.RGBA{
		R: config.Color[0],
		G: config.Color[1],
		B: config.Color[2],
		A: config.Color[3],
	})

	background.Resize(bounds)
	background.Move(fyne.NewPos(0, 0))

	return background
}

// NewCursorRenderer creates appropriate cursor renderer based on config
func NewCursorRenderer(config CursorStyleConfig) CursorRenderer {
	switch config.Type {
	case "underline", "":
		return &UnderlineCursorRenderer{}
	case "border":
		return &BorderCursorRenderer{}
	case "background":
		return &BackgroundCursorRenderer{}
	default:
		// Default to underline for unknown types
		return &UnderlineCursorRenderer{}
	}
}

// FileManager is the main file manager struct
type FileManager struct {
	window         fyne.Window
	currentPath    string
	files          []FileInfo
	fileList       *widget.List
	pathLabel      *widget.Label
	cursorIdx      int          // Current cursor position
	selectedItems  map[int]bool // Set of selected items
	fileBinding    binding.UntypedList
	config         *Config
	cursorRenderer CursorRenderer // Cursor display renderer
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
			CursorStyle: CursorStyleConfig{
				Type:      "underline",
				Color:     [4]uint8{255, 255, 255, 255}, // White
				Thickness: 2,
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

	// Custom item spacing support with icon consideration
	if t.config.UI.ItemSpacing > 0 {
		switch name {
		case theme.SizeNamePadding:
			// Ensure minimum padding for icons but allow small spacing
			minPadding := float32(2) // Very minimal padding
			requested := float32(t.config.UI.ItemSpacing)
			if requested < minPadding {
				return minPadding
			}
			return requested
		case theme.SizeNameInnerPadding:
			return 0
		}
	}

	if t.config.Theme.Dark {
		return theme.DarkTheme().Size(name)
	}
	return theme.DefaultTheme().Size(name)
}

func NewFileManager(app fyne.App, path string, config *Config) *FileManager {
	fm := &FileManager{
		window:         app.NewWindow("File Manager"),
		currentPath:    path,
		cursorIdx:      -1,
		selectedItems:  make(map[int]bool),
		fileBinding:    binding.NewUntypedList(),
		config:         config,
		cursorRenderer: NewCursorRenderer(config.UI.CursorStyle),
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
			// Create tappable icon (onTapped will be set in UpdateItem)
			icon := NewTappableIcon(theme.FolderIcon(), nil)
			name := widget.NewLabel("filename")
			info := widget.NewLabel("info")

			// Left side: icon + name (with minimal spacing)
			// Size icon based on text height for consistency
			textSize := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
			icon.Resize(fyne.NewSize(textSize, textSize))

			leftSide := container.NewHBox(
				icon,
				name,
			)

			// Use border container to align name left and info right
			borderContainer := container.NewBorder(
				nil, nil, leftSide, info, nil,
			)

			// Use normal container with max layout to hold content and decorations
			return container.NewMax(borderContainer)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			dataItem := item.(binding.Untyped)
			data, _ := dataItem.Get()
			listItem := data.(ListItem)
			fileInfo := listItem.FileInfo
			index := listItem.Index

			// obj is a container with border and optional cursor/selection elements
			outerContainer := obj.(*fyne.Container)

			// Find the border container (should be first element)
			var border *fyne.Container
			if len(outerContainer.Objects) > 0 {
				if container, ok := outerContainer.Objects[0].(*fyne.Container); ok {
					border = container
				}
			}

			if border != nil {
				// Find leftSide and info widgets within border
				var leftSide *fyne.Container
				var infoLabel *widget.Label

				for _, obj := range border.Objects {
					if obj == nil {
						continue
					}
					if container, ok := obj.(*fyne.Container); ok {
						leftSide = container
					} else if label, ok := obj.(*widget.Label); ok {
						infoLabel = label
					}
				}

				if leftSide != nil && infoLabel != nil && len(leftSide.Objects) >= 2 {
					// Structure is now: [icon, nameLabel]
					if icon, ok := leftSide.Objects[0].(*TappableIcon); ok {
						nameLabel := leftSide.Objects[1].(*widget.Label)

						// Set icon resource
						if fileInfo.IsDir {
							icon.SetResource(theme.FolderIcon())
						} else {
							icon.SetResource(theme.FileIcon())
						}

						// Set onTapped handler for icon
						icon.onTapped = func() {
							if fileInfo.IsDir {
								fm.loadDirectory(fileInfo.Path)
							}
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
				}
			}

			// Handle 4 display states
			isCursor := index == fm.cursorIdx
			isSelected := fm.selectedItems[index]

			// Clear all decoration elements first
			outerContainer.Objects = []fyne.CanvasObject{border}

			// Add selection background if selected
			if isSelected {
				selectionBG := canvas.NewRectangle(color.RGBA{R: 100, G: 150, B: 200, A: 100})
				selectionBG.Resize(obj.Size())
				selectionBG.Move(fyne.NewPos(0, 0))
				// Wrap selection background in WithoutLayout container
				selectionContainer := container.NewWithoutLayout(selectionBG)
				outerContainer.Objects = append(outerContainer.Objects, selectionContainer)
			}

			// Add cursor if at cursor position
			if isCursor {
				// Calculate text position (icon width + padding)
				iconWidth := float32(16)                         // Fixed icon size
				padding := float32(fm.config.UI.ItemSpacing * 2) // Padding around icon
				textPos := fyne.NewPos(iconWidth+padding, 0)
				cursor := fm.cursorRenderer.RenderCursor(obj.Size(), textPos, fm.config.UI.CursorStyle)

				// Wrap cursor in a container that won't be affected by NewMax
				cursorContainer := container.NewWithoutLayout(cursor)
				outerContainer.Objects = append(outerContainer.Objects, cursorContainer)
			}
		},
	)

	// Hide separators for compact spacing if itemSpacing is small
	if fm.config.UI.ItemSpacing <= 2 {
		fm.fileList.HideSeparators = true
	}

	// Handle cursor movement (both mouse and keyboard)
	fm.fileList.OnSelected = func(id widget.ListItemID) {
		fm.cursorIdx = id
		// Clear list selection to avoid double cursor effect when switching back to keyboard
		fm.fileList.UnselectAll()
		fm.window.Canvas().Unfocus()
		fm.refreshCursor()
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
			if fm.cursorIdx > 0 {
				fm.cursorIdx--
				fm.refreshCursor()
			}
		case fyne.KeyDown:
			if fm.cursorIdx < len(fm.files)-1 {
				fm.cursorIdx++
				fm.refreshCursor()
			}
		case fyne.KeyReturn:
			if fm.cursorIdx >= 0 && fm.cursorIdx < len(fm.files) {
				file := fm.files[fm.cursorIdx]
				if file.IsDir {
					fm.loadDirectory(file.Path)
				}
			}
		case fyne.KeySpace:
			if fm.cursorIdx >= 0 && fm.cursorIdx < len(fm.files) {
				// Toggle selection state of current cursor item
				fm.selectedItems[fm.cursorIdx] = !fm.selectedItems[fm.cursorIdx]
				fm.fileList.Refresh()
			}
		case fyne.KeyBackspace:
			parent := filepath.Dir(fm.currentPath)
			if parent != fm.currentPath {
				fm.loadDirectory(parent)
			}
		}
	})
}

// refreshCursor updates only the cursor display without affecting selection
func (fm *FileManager) refreshCursor() {
	fm.fileList.Refresh()
}

// TappableIcon is a custom icon widget that can handle tap events
type TappableIcon struct {
	widget.BaseWidget
	icon     *widget.Icon
	onTapped func()
}

func NewTappableIcon(resource fyne.Resource, onTapped func()) *TappableIcon {
	icon := widget.NewIcon(resource)
	ti := &TappableIcon{
		icon:     icon,
		onTapped: onTapped,
	}
	ti.ExtendBaseWidget(ti)
	return ti
}

func (ti *TappableIcon) Tapped(_ *fyne.PointEvent) {
	if ti.onTapped != nil {
		ti.onTapped()
	}
}

func (ti *TappableIcon) SetResource(resource fyne.Resource) {
	ti.icon.SetResource(resource)
}

func (ti *TappableIcon) CreateRenderer() fyne.WidgetRenderer {
	return ti.icon.CreateRenderer()
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

	// Convert to ListItem (FileInfo with index)
	items := make([]interface{}, 0, len(entries))
	for i, entry := range entries {
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

		listItem := ListItem{
			Index:    i,
			FileInfo: fileInfo,
		}

		fm.files = append(fm.files, fileInfo)
		items = append(items, listItem)
	}

	// Update binding
	fm.fileBinding.Set(items)
	// Set cursor to first item if directory is not empty
	if len(fm.files) > 0 {
		fm.cursorIdx = 0
	} else {
		fm.cursorIdx = -1
	}
	// Clear selections when changing directory
	fm.selectedItems = make(map[int]bool)
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
