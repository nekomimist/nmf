package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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
}

func NewFileManager(app fyne.App, path string) *FileManager {
	fm := &FileManager{
		window:      app.NewWindow("File Manager"),
		currentPath: path,
		selectedIdx: -1,
		fileBinding: binding.NewUntypedList(),
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
	fm.window.Resize(fyne.NewSize(800, 600))

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
	newFM := NewFileManager(fyne.CurrentApp(), fm.currentPath)
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
	app := app.New()
	app.Settings().SetTheme(theme.DarkTheme())

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

	fm := NewFileManager(app, startPath)
	fm.window.ShowAndRun()
}
