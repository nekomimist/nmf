package fileinfo

import (
	"image/color"
	"testing"
	"time"
)

func TestDetermineFileType(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		filename string
		isDir    bool
		expected FileType
	}{
		{
			name:     "Regular file",
			path:     "/home/user/file.txt",
			filename: "file.txt",
			isDir:    false,
			expected: FileTypeRegular,
		},
		{
			name:     "Directory",
			path:     "/home/user/documents",
			filename: "documents",
			isDir:    true,
			expected: FileTypeDirectory,
		},
		{
			name:     "Hidden file (Unix)",
			path:     "/home/user/.bashrc",
			filename: ".bashrc",
			isDir:    false,
			expected: FileTypeHidden,
		},
		{
			name:     "Hidden directory (Unix)",
			path:     "/home/user/.config",
			filename: ".config",
			isDir:    true,
			expected: FileTypeDirectory, // Directory takes precedence
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DetermineFileType(tc.path, tc.filename, tc.isDir)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestGetTextColor(t *testing.T) {
	colors := FileColorConfig{
		Regular:   [4]uint8{220, 220, 220, 255},
		Directory: [4]uint8{135, 206, 250, 255},
		Symlink:   [4]uint8{255, 165, 0, 255},
		Hidden:    [4]uint8{105, 105, 105, 255},
	}

	testCases := []struct {
		fileType FileType
		expected color.RGBA
	}{
		{FileTypeRegular, color.RGBA{R: 220, G: 220, B: 220, A: 255}},
		{FileTypeDirectory, color.RGBA{R: 135, G: 206, B: 250, A: 255}},
		{FileTypeSymlink, color.RGBA{R: 255, G: 165, B: 0, A: 255}},
		{FileTypeHidden, color.RGBA{R: 105, G: 105, B: 105, A: 255}},
	}

	for _, tc := range testCases {
		result := GetTextColor(tc.fileType, colors)
		if result != tc.expected {
			t.Errorf("For file type %v, expected %v, got %v", tc.fileType, tc.expected, result)
		}
	}
}

func TestGetStatusBackgroundColor(t *testing.T) {
	testCases := []struct {
		status   FileStatus
		expected *color.RGBA
	}{
		{StatusNormal, nil},
		{StatusAdded, &color.RGBA{R: 0, G: 200, B: 0, A: 80}},
		{StatusDeleted, &color.RGBA{R: 128, G: 128, B: 128, A: 60}},
		{StatusModified, &color.RGBA{R: 255, G: 200, B: 0, A: 80}},
	}

	for _, tc := range testCases {
		result := GetStatusBackgroundColor(tc.status)
		if tc.expected == nil {
			if result != nil {
				t.Errorf("For status %v, expected nil, got %v", tc.status, result)
			}
		} else {
			if result == nil || *result != *tc.expected {
				t.Errorf("For status %v, expected %v, got %v", tc.status, tc.expected, result)
			}
		}
	}
}

func TestFormatFileSize(t *testing.T) {
	testCases := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tc := range testCases {
		result := FormatFileSize(tc.size)
		if result != tc.expected {
			t.Errorf("For size %d, expected %s, got %s", tc.size, tc.expected, result)
		}
	}
}

func TestColoredTextSegment(t *testing.T) {
	segment := &ColoredTextSegment{
		Text:          "test.txt",
		Color:         color.RGBA{R: 255, G: 255, B: 255, A: 255},
		Strikethrough: false,
	}

	// Test Inline method
	if !segment.Inline() {
		t.Error("ColoredTextSegment should be inline")
	}

	// Test Textual method
	if segment.Textual() != "test.txt" {
		t.Errorf("Expected 'test.txt', got '%s'", segment.Textual())
	}

	// Test SelectedText method
	if segment.SelectedText() != "test.txt" {
		t.Errorf("Expected 'test.txt', got '%s'", segment.SelectedText())
	}

	// Test strikethrough segment creation
	segmentDeleted := &ColoredTextSegment{
		Text:          "deleted.txt",
		Color:         color.RGBA{R: 128, G: 128, B: 128, A: 255},
		Strikethrough: true,
	}

	// Test that strikethrough segment has correct properties
	if segmentDeleted.Text != "deleted.txt" {
		t.Errorf("Expected text 'deleted.txt', got '%s'", segmentDeleted.Text)
	}

	if !segmentDeleted.Strikethrough {
		t.Error("Expected strikethrough to be true")
	}

	// Note: Visual() method requires Fyne app to be initialized, so we skip testing it
}

func TestFileInfo(t *testing.T) {
	now := time.Now()
	fileInfo := FileInfo{
		Name:     "test.txt",
		Path:     "/home/user/test.txt",
		IsDir:    false,
		Size:     1024,
		Modified: now,
		FileType: FileTypeRegular,
		Status:   StatusNormal,
	}

	if fileInfo.Name != "test.txt" {
		t.Errorf("Expected Name 'test.txt', got '%s'", fileInfo.Name)
	}

	if fileInfo.Path != "/home/user/test.txt" {
		t.Errorf("Expected Path '/home/user/test.txt', got '%s'", fileInfo.Path)
	}

	if fileInfo.IsDir {
		t.Error("Expected IsDir to be false")
	}

	if fileInfo.Size != 1024 {
		t.Errorf("Expected Size 1024, got %d", fileInfo.Size)
	}

	if !fileInfo.Modified.Equal(now) {
		t.Error("Expected Modified time to match")
	}

	if fileInfo.FileType != FileTypeRegular {
		t.Errorf("Expected FileType Regular, got %v", fileInfo.FileType)
	}

	if fileInfo.Status != StatusNormal {
		t.Errorf("Expected Status Normal, got %v", fileInfo.Status)
	}
}

func TestListItem(t *testing.T) {
	fileInfo := FileInfo{
		Name: "test.txt",
		Path: "/home/user/test.txt",
	}

	listItem := ListItem{
		Index:    5,
		FileInfo: fileInfo,
	}

	if listItem.Index != 5 {
		t.Errorf("Expected Index 5, got %d", listItem.Index)
	}

	if listItem.FileInfo.Name != "test.txt" {
		t.Errorf("Expected FileInfo.Name 'test.txt', got '%s'", listItem.FileInfo.Name)
	}
}
