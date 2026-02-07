package fileinfo

import (
	"io"
	"os"
)

// SMBPathOps describes SMB-like path operations needed by job execution.
type SMBPathOps interface {
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	Lstat(path string) (os.FileInfo, error)
	Open(path string) (io.ReadCloser, error)
	OpenFile(path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
	MkdirAll(path string, perm os.FileMode) error
	Remove(path string) error
	Rename(oldpath, newpath string) error
	Readlink(path string) (string, error)
	Symlink(target, linkpath string) error
	Base(p string) string
	Join(elem ...string) string
}

// SMBSession is a reusable SMB path operation context (typically one mounted share).
type SMBSession interface {
	SMBPathOps
	Close() error
}

// SMBSessionOpener opens a reusable SMB session.
type SMBSessionOpener interface {
	OpenSession() (SMBSession, error)
}
