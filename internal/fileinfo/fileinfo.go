package fileinfo

import (
	"fmt"
	"image/color"
	"os"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

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

// FileColorConfig represents color settings for different file types
type FileColorConfig struct {
	Regular   [4]uint8 `json:"regular"`   // Regular files
	Directory [4]uint8 `json:"directory"` // Directories
	Symlink   [4]uint8 `json:"symlink"`   // Symbolic links
	Hidden    [4]uint8 `json:"hidden"`    // Hidden files
}

// ListItem wraps FileInfo with index for rendering
type ListItem struct {
	Index    int
	FileInfo FileInfo
}

// DetermineFileType determines the file type based on file attributes
func DetermineFileType(path string, name string, isDir bool) FileType {
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

	// Check for Windows hidden file attribute
	if runtime.GOOS == "windows" && IsWindowsHidden(path) {
		return FileTypeHidden
	}

	return FileTypeRegular
}

// GetTextColor returns the text color based on file type
func GetTextColor(fileType FileType, colors FileColorConfig) color.RGBA {
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

// GetStatusBackgroundColor returns the background color based on file status
// Returns nil for normal status (no background)
func GetStatusBackgroundColor(status FileStatus) *color.RGBA {
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

// FormatFileSize formats file size in human-readable format
func FormatFileSize(size int64) string {
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
