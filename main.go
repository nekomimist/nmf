package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Global debug flag
var debugMode bool

// debugPrint prints debug messages only when debug mode is enabled
func debugPrint(format string, args ...interface{}) {
	if debugMode {
		log.Printf("DEBUG: "+format, args...)
	}
}

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

// FileColorConfig represents color settings for different file types
type FileColorConfig struct {
	Regular   [4]uint8 `json:"regular"`   // Regular files
	Directory [4]uint8 `json:"directory"` // Directories
	Symlink   [4]uint8 `json:"symlink"`   // Symbolic links
	Hidden    [4]uint8 `json:"hidden"`    // Hidden files
}

// UIConfig represents UI-related settings
type UIConfig struct {
	ShowHiddenFiles bool              `json:"showHiddenFiles"`
	SortBy          string            `json:"sortBy"`
	ItemSpacing     int               `json:"itemSpacing"`
	CursorStyle     CursorStyleConfig `json:"cursorStyle"`
	FileColors      FileColorConfig   `json:"fileColors"`
}

// CursorStyleConfig represents cursor appearance settings
type CursorStyleConfig struct {
	Type      string   `json:"type"`      // "underline", "border", "background", "icon", "font"
	Color     [4]uint8 `json:"color"`     // RGBA color values
	Thickness int      `json:"thickness"` // Line thickness for underline/border
}

// FileType represents the type of file
type FileType int

const (
	FileTypeRegular FileType = iota
	FileTypeDirectory
	FileTypeSymlink
	FileTypeHidden
)

// FileStatus represents the current status of a file in the directory watcher
type FileStatus int

const (
	StatusNormal   FileStatus = iota
	StatusAdded               // 新規追加されたファイル
	StatusDeleted             // 削除されたファイル
	StatusModified            // 変更されたファイル
)

// FileInfo represents a file or directory
type FileInfo struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	Modified time.Time
	FileType FileType
	Status   FileStatus // ファイルの現在のステータス
}

// determineFileType determines the file type based on file attributes
func determineFileType(path string, name string, isDir bool) FileType {
	// Check if it's a symlink first (works on both Linux and Windows)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return FileTypeSymlink
		}
	}

	// Check for directory
	if isDir {
		return FileTypeDirectory
	}

	// Check for hidden files (starting with .)
	if strings.HasPrefix(name, ".") {
		return FileTypeHidden
	}

	// Check for Windows hidden file attribute (to be implemented)
	if runtime.GOOS == "windows" && isWindowsHidden(path) {
		return FileTypeHidden
	}

	return FileTypeRegular
}

// getTextColor returns the text color based on file type
func getTextColor(fileType FileType, colors FileColorConfig) color.RGBA {
	switch fileType {
	case FileTypeDirectory:
		return color.RGBA{R: colors.Directory[0], G: colors.Directory[1], B: colors.Directory[2], A: colors.Directory[3]}
	case FileTypeSymlink:
		return color.RGBA{R: colors.Symlink[0], G: colors.Symlink[1], B: colors.Symlink[2], A: colors.Symlink[3]}
	case FileTypeHidden:
		return color.RGBA{R: colors.Hidden[0], G: colors.Hidden[1], B: colors.Hidden[2], A: colors.Hidden[3]}
	default: // FileTypeRegular
		return color.RGBA{R: colors.Regular[0], G: colors.Regular[1], B: colors.Regular[2], A: colors.Regular[3]}
	}
}

// getStatusBackgroundColor returns the background color based on file status
// Returns nil for normal status (no background)
func getStatusBackgroundColor(status FileStatus) *color.RGBA {
	switch status {
	case StatusAdded:
		return &color.RGBA{R: 0, G: 200, B: 0, A: 80} // Semi-transparent green background
	case StatusDeleted:
		return &color.RGBA{R: 128, G: 128, B: 128, A: 60} // Semi-transparent gray background
	case StatusModified:
		return &color.RGBA{R: 255, G: 200, B: 0, A: 80} // Semi-transparent orange background
	default: // StatusNormal
		return nil // No background
	}
}

// ColoredTextSegment is a custom RichText segment that supports custom colors and styles
type ColoredTextSegment struct {
	Text          string
	Color         color.RGBA
	Strikethrough bool
}

func (s *ColoredTextSegment) Inline() bool {
	return true
}

func (s *ColoredTextSegment) Textual() string {
	return s.Text
}

func (s *ColoredTextSegment) Update(o fyne.CanvasObject) {
	if text, ok := o.(*canvas.Text); ok {
		text.Text = s.Text
		text.Color = s.Color
		text.Refresh()
	}
}

func (s *ColoredTextSegment) Visual() fyne.CanvasObject {
	text := canvas.NewText(s.Text, s.Color)
	text.TextStyle = fyne.TextStyle{
		Bold:      false,
		Italic:    false,
		Monospace: false,
	}

	// For deleted files, we'll use a visual indication by prefixing with strikethrough-like chars
	if s.Strikethrough {
		text.Text = "⊠ " + s.Text
	}

	// Set appropriate text size from theme
	text.TextSize = fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
	return text
}

func (s *ColoredTextSegment) Select(pos1, pos2 fyne.Position) {
	// Selection handling - could be implemented if needed
}

func (s *ColoredTextSegment) SelectedText() string {
	return s.Text
}

func (s *ColoredTextSegment) Unselect() {
	// Unselection handling - could be implemented if needed
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

	// Simple full-width underline at bottom edge
	underline.Resize(fyne.NewSize(bounds.Width, thickness))
	underline.Move(fyne.NewPos(0, bounds.Height-thickness))

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
	pathEntry      *widget.Entry
	cursorPath     string          // Current cursor file path
	selectedFiles  map[string]bool // Set of selected file paths
	fileBinding    binding.UntypedList
	config         *Config
	cursorRenderer CursorRenderer    // Cursor display renderer
	shiftPressed   bool              // Track Shift key state
	dirWatcher     *DirectoryWatcher // Directory change watcher
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
			FileColors: FileColorConfig{
				Regular:   [4]uint8{220, 220, 220, 255}, // Light gray - regular files
				Directory: [4]uint8{135, 206, 250, 255}, // Light sky blue - directories
				Symlink:   [4]uint8{255, 165, 0, 255},   // Orange - symbolic links
				Hidden:    [4]uint8{105, 105, 105, 255}, // Dim gray - hidden files
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

// loadConfig loads configuration from file and merges with defaults
func loadConfig() *Config {
	// Start with default configuration
	config := getDefaultConfig()

	configPath := getConfigPath()
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Config file not found, using defaults: %v", err)
		return config
	}

	// Parse config file into a temporary config
	var fileConfig Config
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		log.Printf("Error parsing config file, using defaults: %v", err)
		return config
	}

	// Merge file config with defaults
	mergeConfigs(config, &fileConfig)
	return config
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
	if fileConfig.UI.SortBy != "" {
		defaultConfig.UI.SortBy = fileConfig.UI.SortBy
	}
	if fileConfig.UI.ItemSpacing != 0 {
		defaultConfig.UI.ItemSpacing = fileConfig.UI.ItemSpacing
	}

	// Merge CursorStyle config
	if fileConfig.UI.CursorStyle.Type != "" {
		defaultConfig.UI.CursorStyle.Type = fileConfig.UI.CursorStyle.Type
	}
	if fileConfig.UI.CursorStyle.Color != [4]uint8{0, 0, 0, 0} {
		defaultConfig.UI.CursorStyle.Color = fileConfig.UI.CursorStyle.Color
	}
	if fileConfig.UI.CursorStyle.Thickness != 0 {
		defaultConfig.UI.CursorStyle.Thickness = fileConfig.UI.CursorStyle.Thickness
	}

	// Merge FileColors config
	if fileConfig.UI.FileColors.Regular != [4]uint8{0, 0, 0, 0} {
		defaultConfig.UI.FileColors.Regular = fileConfig.UI.FileColors.Regular
	}
	if fileConfig.UI.FileColors.Directory != [4]uint8{0, 0, 0, 0} {
		defaultConfig.UI.FileColors.Directory = fileConfig.UI.FileColors.Directory
	}
	if fileConfig.UI.FileColors.Symlink != [4]uint8{0, 0, 0, 0} {
		defaultConfig.UI.FileColors.Symlink = fileConfig.UI.FileColors.Symlink
	}
	if fileConfig.UI.FileColors.Hidden != [4]uint8{0, 0, 0, 0} {
		defaultConfig.UI.FileColors.Hidden = fileConfig.UI.FileColors.Hidden
	}
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
	debugPrint("Loaded custom font: %s", fontPath)
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
		cursorPath:     "",
		selectedFiles:  make(map[string]bool),
		fileBinding:    binding.NewUntypedList(),
		config:         config,
		cursorRenderer: NewCursorRenderer(config.UI.CursorStyle),
		shiftPressed:   false,
	}

	// Create directory watcher
	fm.dirWatcher = NewDirectoryWatcher(fm)

	fm.setupUI()
	fm.loadDirectory(path)

	// Start watching after initial load
	fm.dirWatcher.Start()

	return fm
}

func (fm *FileManager) setupUI() {
	// Path entry for direct path input
	fm.pathEntry = widget.NewEntry()
	fm.pathEntry.SetText(fm.currentPath)
	fm.pathEntry.OnSubmitted = func(path string) {
		fm.navigateToPath(path)
	}

	// Create file list
	fm.fileList = widget.NewListWithData(
		fm.fileBinding,
		func() fyne.CanvasObject {
			// Create tappable icon (onTapped will be set in UpdateItem)
			icon := NewTappableIcon(theme.FolderIcon(), nil)
			// Use RichText for colored filename display
			nameRichText := widget.NewRichTextFromMarkdown("filename")
			info := widget.NewLabel("info")

			// Left side: icon + name (with minimal spacing)
			// Size icon based on text height for consistency
			textSize := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
			icon.Resize(fyne.NewSize(textSize, textSize))

			leftSide := container.NewHBox(
				icon,
				nameRichText,
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
					// Structure is now: [icon, nameRichText]
					if icon, ok := leftSide.Objects[0].(*TappableIcon); ok {
						nameRichText := leftSide.Objects[1].(*widget.RichText)

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

						// Get text color based on file type
						textColor := getTextColor(fileInfo.FileType, fm.config.UI.FileColors)

						// Create a custom text segment with text color only
						coloredSegment := &ColoredTextSegment{
							Text:          fileInfo.Name,
							Color:         textColor,
							Strikethrough: fileInfo.Status == StatusDeleted,
						}

						nameRichText.Segments = []widget.RichTextSegment{coloredSegment}
						nameRichText.Refresh()

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
			currentCursorIdx := fm.getCurrentCursorIndex()
			isCursor := index == currentCursorIdx
			isSelected := fm.selectedFiles[fileInfo.Path]

			// Clear all decoration elements first
			outerContainer.Objects = []fyne.CanvasObject{border}

			// Add status background if file has a status (covers entire item like selection)
			statusBGColor := getStatusBackgroundColor(fileInfo.Status)
			if statusBGColor != nil {
				statusBG := canvas.NewRectangle(*statusBGColor)
				statusBG.Resize(obj.Size())
				statusBG.Move(fyne.NewPos(0, 0))
				// Wrap status background in WithoutLayout container
				statusContainer := container.NewWithoutLayout(statusBG)
				outerContainer.Objects = append(outerContainer.Objects, statusContainer)
			}

			// Add selection background if selected (covers entire item)
			if isSelected {
				selectionBG := canvas.NewRectangle(color.RGBA{R: 100, G: 150, B: 200, A: 100})
				selectionBG.Resize(obj.Size())
				selectionBG.Move(fyne.NewPos(0, 0))
				// Wrap selection background in WithoutLayout container
				selectionContainer := container.NewWithoutLayout(selectionBG)
				outerContainer.Objects = append(outerContainer.Objects, selectionContainer)
			}

			// Add cursor if at cursor position (covers entire item like status/selection)
			if isCursor {
				cursor := fm.cursorRenderer.RenderCursor(obj.Size(), fyne.NewPos(0, 0), fm.config.UI.CursorStyle)

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
		fm.setCursorByIndex(id)
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

	// Layout without overlay
	content := container.NewBorder(
		container.NewVBox(toolbar, fm.pathEntry),
		nil, nil, nil,
		fm.fileList,
	)

	fm.window.SetContent(content)
	fm.window.Resize(fyne.NewSize(float32(fm.config.Window.Width), float32(fm.config.Window.Height)))

	// Setup window close handler to properly stop DirectoryWatcher
	fm.window.SetCloseIntercept(func() {
		debugPrint("Window close intercepted - initiating cleanup for path: %s", fm.currentPath)
		if fm.dirWatcher != nil {
			debugPrint("Stopping DirectoryWatcher...")
			fm.dirWatcher.Stop()
			debugPrint("DirectoryWatcher.Stop() completed successfully")
		} else {
			debugPrint("DirectoryWatcher was nil, skipping stop")
		}
		debugPrint("Proceeding with window close")
		fm.window.Close()
	})

	// Setup keyboard handling via desktop.Canvas
	dc, ok := (fm.window.Canvas()).(desktop.Canvas)
	if ok {

		dc.SetOnKeyDown(func(ev *fyne.KeyEvent) {
			switch ev.Name {
			case desktop.KeyShiftLeft, desktop.KeyShiftRight:
				fm.shiftPressed = true
				debugPrint("Shift key pressed (state: %t)", fm.shiftPressed)
			}
		})

		dc.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			if ev.Name == desktop.KeyShiftLeft || ev.Name == desktop.KeyShiftRight {
				fm.shiftPressed = false
				debugPrint("Shift key released (state: %t)", fm.shiftPressed)
			}
		})

		// Handle normal keys with key repeat support
		fm.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
			switch ev.Name {
			case fyne.KeyUp:
				currentIdx := fm.getCurrentCursorIndex()
				if fm.shiftPressed {
					// Move up 20 items or to the beginning
					debugPrint("Shift+Up detected via SetOnTypedKey!")
					newIdx := currentIdx - 20
					if newIdx < 0 {
						newIdx = 0
					}
					fm.setCursorByIndex(newIdx)
					fm.refreshCursor()
				} else {
					if currentIdx > 0 {
						fm.setCursorByIndex(currentIdx - 1)
						fm.refreshCursor()
					}
				}

			case fyne.KeyDown:
				currentIdx := fm.getCurrentCursorIndex()
				if fm.shiftPressed {
					// Move down 20 items or to the end
					debugPrint("Shift+Down detected via SetOnTypedKey!")
					newIdx := currentIdx + 20
					if newIdx >= len(fm.files) {
						newIdx = len(fm.files) - 1
					}
					fm.setCursorByIndex(newIdx)
					fm.refreshCursor()
				} else {
					if currentIdx < len(fm.files)-1 {
						fm.setCursorByIndex(currentIdx + 1)
						fm.refreshCursor()
					}
				}

			case fyne.KeyReturn:
				currentIdx := fm.getCurrentCursorIndex()
				if currentIdx >= 0 && currentIdx < len(fm.files) {
					file := fm.files[currentIdx]
					if file.IsDir {
						fm.loadDirectory(file.Path)
					}
				}

			case fyne.KeySpace:
				currentIdx := fm.getCurrentCursorIndex()
				if currentIdx >= 0 && currentIdx < len(fm.files) {
					file := fm.files[currentIdx]
					// Don't allow selection of parent directory entry
					if file.Name != ".." {
						// Toggle selection state of current cursor item
						fm.selectedFiles[file.Path] = !fm.selectedFiles[file.Path]
						fm.fileList.Refresh()
					}
				}

			case fyne.KeyBackspace:
				parent := filepath.Dir(fm.currentPath)
				if parent != fm.currentPath {
					fm.loadDirectory(parent)
				}

			case fyne.KeyComma:
				// Shift+Comma = '<' - Move to first item
				if fm.shiftPressed && len(fm.files) > 0 {
					fm.setCursorByIndex(0)
					fm.refreshCursor()
				}

			case fyne.KeyPeriod:
				// Shift+Period = '>' - Move to last item
				if fm.shiftPressed && len(fm.files) > 0 {
					fm.setCursorByIndex(len(fm.files) - 1)
					fm.refreshCursor()
				}
			}
		})
	}
}

// getCurrentCursorIndex returns the current cursor index based on cursor path
func (fm *FileManager) getCurrentCursorIndex() int {
	if fm.cursorPath == "" {
		return -1
	}
	for i, file := range fm.files {
		if file.Path == fm.cursorPath {
			return i
		}
	}
	return -1
}

// setCursorByIndex sets the cursor to the specified index
func (fm *FileManager) setCursorByIndex(index int) {
	if index >= 0 && index < len(fm.files) {
		fm.cursorPath = fm.files[index].Path
	} else {
		fm.cursorPath = ""
	}
}

// refreshCursor updates only the cursor display without affecting selection
func (fm *FileManager) refreshCursor() {
	cursorIdx := fm.getCurrentCursorIndex()
	if cursorIdx >= 0 {
		fm.fileList.ScrollTo(widget.ListItemID(cursorIdx))
	}
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
	ti.Refresh()
}

func (ti *TappableIcon) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(ti.icon)
}

// navigateToPath handles path entry validation and navigation
func (fm *FileManager) navigateToPath(inputPath string) {
	// Trim whitespace from input
	path := strings.TrimSpace(inputPath)

	// Handle empty path - do nothing
	if path == "" {
		fm.pathEntry.SetText(fm.currentPath) // Reset to current path
		return
	}

	// Handle tilde expansion for home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			debugPrint("Error getting home directory: %v", err)
			fm.pathEntry.SetText(fm.currentPath) // Reset to current path
			return
		}
		path = strings.Replace(path, "~", home, 1)
	}

	// Convert to absolute path if it's relative
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			debugPrint("Error converting to absolute path: %v", err)
			fm.pathEntry.SetText(fm.currentPath) // Reset to current path
			return
		}
		path = absPath
	}

	// Validate the path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		debugPrint("Path does not exist: %s - %v", path, err)
		fm.pathEntry.SetText(fm.currentPath) // Reset to current path
		return
	}

	if !info.IsDir() {
		debugPrint("Path is not a directory: %s", path)
		fm.pathEntry.SetText(fm.currentPath) // Reset to current path
		return
	}

	// Path is valid, navigate to it
	fm.loadDirectory(path)

	// Remove focus from path entry after successful navigation
	fm.window.Canvas().Unfocus()
}

func (fm *FileManager) loadDirectory(path string) {
	// Stop current directory watcher if running
	if fm.dirWatcher != nil {
		fm.dirWatcher.Stop()
	}

	fm.currentPath = path
	fm.pathEntry.SetText(path)
	fm.files = []FileInfo{}

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return
	}

	// Convert to ListItem (FileInfo with index)
	items := make([]interface{}, 0, len(entries)+1)
	index := 0

	// Add parent directory entry if not at root
	parent := filepath.Dir(path)
	if parent != path {
		parentInfo := FileInfo{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			Size:     0,
			Modified: time.Time{},
			FileType: FileTypeDirectory, // Parent directory is always a directory
			Status:   StatusNormal,
		}

		listItem := ListItem{
			Index:    index,
			FileInfo: parentInfo,
		}

		fm.files = append(fm.files, parentInfo)
		items = append(items, listItem)
		index++
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		fileType := determineFileType(fullPath, entry.Name(), entry.IsDir())

		fileInfo := FileInfo{
			Name:     entry.Name(),
			Path:     fullPath,
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			FileType: fileType,
			Status:   StatusNormal,
		}

		listItem := ListItem{
			Index:    index,
			FileInfo: fileInfo,
		}

		fm.files = append(fm.files, fileInfo)
		items = append(items, listItem)
		index++
	}

	// Update binding
	fm.fileBinding.Set(items)

	// Clear selections when changing directory
	fm.selectedFiles = make(map[string]bool)

	// Set cursor to first item if directory is not empty and refresh cursor
	if len(fm.files) > 0 {
		fm.setCursorByIndex(0)
		// Refresh cursor display immediately
		fm.refreshCursor()
	} else {
		fm.cursorPath = ""
	}

	// Restart directory watcher for new path
	if fm.dirWatcher != nil {
		fm.dirWatcher.Start()
	}
}

// DirectoryWatcher handles incremental directory change detection
type DirectoryWatcher struct {
	fm            *FileManager
	previousFiles map[string]FileInfo // Previous state for comparison
	ticker        *time.Ticker
	stopChan      chan bool
	changeChan    chan *PendingChanges // Channel for thread-safe change communication
	stopped       bool                 // Track if watcher is already stopped
}

// PendingChanges represents file changes waiting to be applied
type PendingChanges struct {
	Added    []FileInfo
	Deleted  []FileInfo
	Modified []FileInfo
}

// NewDirectoryWatcher creates a new directory watcher
func NewDirectoryWatcher(fm *FileManager) *DirectoryWatcher {
	return &DirectoryWatcher{
		fm:            fm,
		previousFiles: make(map[string]FileInfo),
		stopChan:      make(chan bool),
		changeChan:    make(chan *PendingChanges, 10), // Buffered channel
	}
}

// Start begins watching the current directory for changes
func (dw *DirectoryWatcher) Start() {
	if dw.ticker != nil && !dw.stopped {
		return // Already running
	}

	dw.stopped = false

	// Recreate channels if they were closed
	if dw.stopChan == nil {
		dw.stopChan = make(chan bool)
	}
	if dw.changeChan == nil {
		dw.changeChan = make(chan *PendingChanges, 10)
	}

	dw.ticker = time.NewTicker(2 * time.Second)
	dw.updateSnapshot() // Take initial snapshot

	// Start directory monitoring goroutine
	ticker := dw.ticker // Capture ticker reference for this goroutine
	go func() {
		defer ticker.Stop() // Clean up ticker when goroutine exits
		for {
			select {
			case <-ticker.C:
				dw.checkForChanges()
			case <-dw.stopChan:
				return
			}
		}
	}()

	// Start change processing goroutine
	go func() {
		for {
			select {
			case changes := <-dw.changeChan:
				// Apply data changes (binding auto-updates UI)
				dw.applyDataChanges(changes.Added, changes.Deleted, changes.Modified)
			case <-dw.stopChan:
				return
			}
		}
	}()
}

// Stop stops the directory watcher
func (dw *DirectoryWatcher) Stop() {
	if dw.stopped {
		return // Already stopped, do nothing
	}

	dw.stopped = true
	dw.ticker = nil // Just clear reference, goroutine will handle cleanup

	// Close channels safely
	close(dw.stopChan)
	dw.stopChan = nil

	// Close change channel too
	close(dw.changeChan)
	dw.changeChan = nil
}

// updateSnapshot updates the current file snapshot
func (dw *DirectoryWatcher) updateSnapshot() {
	dw.previousFiles = make(map[string]FileInfo)

	// Take snapshot of current files (excluding ".." entry)
	for _, file := range dw.fm.files {
		if file.Name != ".." {
			dw.previousFiles[file.Path] = file
		}
	}
}

// checkForChanges detects and handles file system changes
func (dw *DirectoryWatcher) checkForChanges() {
	// Read current directory state
	entries, err := os.ReadDir(dw.fm.currentPath)
	if err != nil {
		return // Skip this check if directory read fails
	}

	currentFiles := make(map[string]FileInfo)

	// Build current file map
	for _, entry := range entries {
		fullPath := filepath.Join(dw.fm.currentPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileType := determineFileType(fullPath, entry.Name(), entry.IsDir())
		fileInfo := FileInfo{
			Name:     entry.Name(),
			Path:     fullPath,
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			FileType: fileType,
			Status:   StatusNormal,
		}
		currentFiles[fullPath] = fileInfo
	}

	// Detect changes
	added, deleted, modified := dw.detectChanges(currentFiles)

	// Apply changes if any detected
	if len(added) > 0 || len(deleted) > 0 || len(modified) > 0 {
		// Send changes to processing channel (if not stopped)
		if !dw.stopped && dw.changeChan != nil {
			select {
			case dw.changeChan <- &PendingChanges{
				Added:    added,
				Deleted:  deleted,
				Modified: modified,
			}:
				// Changes sent successfully
			default:
				// Channel full, skip this update
				debugPrint("Change channel full, skipping update")
			}
		}
	}
}

// detectChanges compares current and previous states to find differences
func (dw *DirectoryWatcher) detectChanges(currentFiles map[string]FileInfo) (added, deleted, modified []FileInfo) {
	// Find added files
	for path, file := range currentFiles {
		if _, exists := dw.previousFiles[path]; !exists {
			file.Status = StatusAdded
			added = append(added, file)
		} else {
			// Check for modifications
			prevFile := dw.previousFiles[path]
			if !file.Modified.Equal(prevFile.Modified) || file.Size != prevFile.Size {
				file.Status = StatusModified
				modified = append(modified, file)
			}
		}
	}

	// Find deleted files
	for path, file := range dw.previousFiles {
		if _, exists := currentFiles[path]; !exists {
			file.Status = StatusDeleted
			deleted = append(deleted, file)
		}
	}

	return added, deleted, modified
}

// applyDataChanges applies detected changes to the file manager data (thread-safe)
func (dw *DirectoryWatcher) applyDataChanges(added, deleted, modified []FileInfo) {
	debugPrint("Applying changes: %d added, %d deleted, %d modified", len(added), len(deleted), len(modified))

	// Track if we need to rebuild the binding
	needsBindingUpdate := len(added) > 0 || len(deleted) > 0 || len(modified) > 0

	// Handle deleted files - mark as deleted but keep in list
	for _, deletedFile := range deleted {
		for i, file := range dw.fm.files {
			if file.Path == deletedFile.Path {
				dw.fm.files[i].Status = StatusDeleted
				// Remove from selections if selected
				delete(dw.fm.selectedFiles, deletedFile.Path)
				break
			}
		}
	}

	// Handle modified files - update status
	for _, modifiedFile := range modified {
		for i, file := range dw.fm.files {
			if file.Path == modifiedFile.Path {
				dw.fm.files[i] = modifiedFile
				break
			}
		}
	}

	// Handle added files - append to end
	for _, addedFile := range added {
		dw.fm.files = append(dw.fm.files, addedFile)
	}

	// Update binding to reflect all changes (this auto-refreshes UI)
	if needsBindingUpdate {
		items := make([]interface{}, len(dw.fm.files))
		for i, file := range dw.fm.files {
			items[i] = ListItem{
				Index:    i,
				FileInfo: file,
			}
		}
		dw.fm.fileBinding.Set(items)
	}

	// Update snapshot for next comparison
	dw.updateSnapshot()
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
	// Parse command line flags
	var startPath string
	flag.BoolVar(&debugMode, "d", false, "Enable debug mode")
	flag.StringVar(&startPath, "path", "", "Starting directory path")
	flag.Parse()

	// If no path specified via flag, check remaining arguments
	if startPath == "" && flag.NArg() > 0 {
		startPath = flag.Arg(0)
	}

	// If still no path, use current working directory
	if startPath == "" {
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current directory: %v", err)
		}
		startPath = pwd
	} else {
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
	}

	// Load configuration
	config := loadConfig()

	app := app.New()

	// Apply custom theme
	customTheme := NewCustomTheme(config)
	app.Settings().SetTheme(customTheme)

	fm := NewFileManager(app, startPath, config)
	fm.window.ShowAndRun()
}
