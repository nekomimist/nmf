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

// FileInfo represents a file or directory
type FileInfo struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	Modified time.Time
	FileType FileType
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

// getFileTypeColor returns the color for a given file type
func getFileTypeColor(fileType FileType, colors FileColorConfig) color.RGBA {
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

// ColoredTextSegment is a custom RichText segment that supports custom colors
type ColoredTextSegment struct {
	Text  string
	Color color.RGBA
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
	text.TextStyle = fyne.TextStyle{}
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

// KeyableWidget implements desktop.Keyable for handling key events
type KeyableWidget struct {
	widget.BaseWidget
	fm *FileManager
}

func NewKeyableWidget(fm *FileManager) *KeyableWidget {
	kw := &KeyableWidget{fm: fm}
	kw.ExtendBaseWidget(kw)
	return kw
}

func (kw *KeyableWidget) CreateRenderer() fyne.WidgetRenderer {
	// Invisible widget, just for key handling
	return widget.NewCard("", "", widget.NewLabel("")).CreateRenderer()
}

func (kw *KeyableWidget) KeyDown(key *fyne.KeyEvent) {
	switch key.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		kw.fm.shiftPressed = true
		log.Printf("DEBUG: Shift key pressed (state: %t)", kw.fm.shiftPressed)
	}
}

func (kw *KeyableWidget) KeyUp(key *fyne.KeyEvent) {
	switch key.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		kw.fm.shiftPressed = false
		log.Printf("DEBUG: Shift key released (state: %t)", kw.fm.shiftPressed)
	}
}

// Implement fyne.Focusable interface
func (kw *KeyableWidget) FocusGained() {
	log.Println("DEBUG: KeyableWidget gained focus")
}

func (kw *KeyableWidget) FocusLost() {
	log.Println("DEBUG: KeyableWidget lost focus")
}

func (kw *KeyableWidget) TypedRune(r rune) {
	fm := kw.fm
	switch r {
	case '<':
		// Move to first item
		if len(fm.files) > 0 {
			fm.cursorIdx = 0
			fm.refreshCursor()
		}
	case '>':
		// Move to last item
		if len(fm.files) > 0 {
			fm.cursorIdx = len(fm.files) - 1
			fm.refreshCursor()
		}
	}
}

func (kw *KeyableWidget) TypedKey(key *fyne.KeyEvent) {
	fm := kw.fm
	switch key.Name {
	case fyne.KeyUp:
		if fm.shiftPressed {
			// Move up 20 items or to the beginning
			log.Println("DEBUG: Shift+Up detected via TypedKey!")
			newIdx := fm.cursorIdx - 20
			if newIdx < 0 {
				newIdx = 0
			}
			fm.cursorIdx = newIdx
			fm.refreshCursor()
		} else {
			if fm.cursorIdx > 0 {
				fm.cursorIdx--
				fm.refreshCursor()
			}
		}
	case fyne.KeyDown:
		if fm.shiftPressed {
			// Move down 20 items or to the end
			log.Println("DEBUG: Shift+Down detected via TypedKey!")
			newIdx := fm.cursorIdx + 20
			if newIdx >= len(fm.files) {
				newIdx = len(fm.files) - 1
			}
			fm.cursorIdx = newIdx
			fm.refreshCursor()
		} else {
			if fm.cursorIdx < len(fm.files)-1 {
				fm.cursorIdx++
				fm.refreshCursor()
			}
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
			file := fm.files[fm.cursorIdx]
			// Don't allow selection of parent directory entry
			if file.Name != ".." {
				// Toggle selection state of current cursor item
				fm.selectedItems[fm.cursorIdx] = !fm.selectedItems[fm.cursorIdx]
				fm.fileList.Refresh()
			}
		}
	case fyne.KeyBackspace:
		parent := filepath.Dir(fm.currentPath)
		if parent != fm.currentPath {
			fm.loadDirectory(parent)
		}
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
	shiftPressed   bool           // Track Shift key state
	keyHandler     *KeyableWidget // Key event handler
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
		shiftPressed:   false,
	}

	// Create key handler for desktop.Keyable implementation
	fm.keyHandler = NewKeyableWidget(fm)

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

						// Get color based on file type and create custom text segment
						fileColor := getFileTypeColor(fileInfo.FileType, fm.config.UI.FileColors)

						// Create a custom text segment with color
						coloredSegment := &ColoredTextSegment{
							Text:  fileInfo.Name,
							Color: fileColor,
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

	// Layout with key handler (invisible but focusable)
	content := container.NewBorder(
		container.NewVBox(toolbar, fm.pathLabel),
		nil, nil, nil,
		container.NewMax(fm.fileList, fm.keyHandler), // Overlay key handler
	)

	fm.window.SetContent(content)
	fm.window.Resize(fyne.NewSize(float32(fm.config.Window.Width), float32(fm.config.Window.Height)))

	// Focus the key handler so it can receive key events
	fm.window.Canvas().Focus(fm.keyHandler)

	// All key handling is now done via KeyableWidget
}

// refreshCursor updates only the cursor display without affecting selection
func (fm *FileManager) refreshCursor() {
	if fm.cursorIdx >= 0 && fm.cursorIdx < len(fm.files) {
		fm.fileList.ScrollTo(widget.ListItemID(fm.cursorIdx))
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
	fm.selectedItems = make(map[int]bool)

	// Set cursor to first item if directory is not empty and refresh cursor
	if len(fm.files) > 0 {
		fm.cursorIdx = 0
		// Ensure focus is on key handler for keyboard navigation
		fm.window.Canvas().Focus(fm.keyHandler)
		// Refresh cursor display immediately
		fm.refreshCursor()
	} else {
		fm.cursorIdx = -1
	}
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
			// Save current cursor position to restore after refresh
			savedCursorIdx := fm.cursorIdx
			fm.loadDirectory(fm.currentPath)
			// Try to restore cursor position if still valid
			if savedCursorIdx >= 0 && savedCursorIdx < len(fm.files) {
				fm.cursorIdx = savedCursorIdx
				fm.refreshCursor()
			}
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
