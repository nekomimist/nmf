package fileinfo

import (
	"os"
	"path/filepath"
)

// FileSystem interface abstracts file system operations for better testability
type FileSystem interface {
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	UserHomeDir() (string, error)
	Getwd() (string, error)
	IsAbs(path string) bool
	Abs(path string) (string, error)
}

// RealFileSystem implements FileSystem using real OS operations
type RealFileSystem struct{}

func (fs *RealFileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (fs *RealFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (fs *RealFileSystem) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

func (fs *RealFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (fs *RealFileSystem) Getwd() (string, error) {
	return os.Getwd()
}

func (fs *RealFileSystem) IsAbs(path string) bool {
	return filepath.IsAbs(path)
}

func (fs *RealFileSystem) Abs(path string) (string, error) {
	return filepath.Abs(path)
}
