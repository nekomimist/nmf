package errors

import (
	"fmt"
)

// ErrorType represents different types of errors that can occur
type ErrorType int

const (
	ErrorTypeConfig ErrorType = iota
	ErrorTypeFileSystem
	ErrorTypeUI
	ErrorTypeWatcher
	ErrorTypeTheme
)

// String returns a string representation of the error type
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeConfig:
		return "config"
	case ErrorTypeFileSystem:
		return "filesystem"
	case ErrorTypeUI:
		return "ui"
	case ErrorTypeWatcher:
		return "watcher"
	case ErrorTypeTheme:
		return "theme"
	default:
		return "unknown"
	}
}

// AppError represents a structured application error
type AppError struct {
	Type      ErrorType
	Operation string
	Path      string
	Message   string
	Err       error
}

func (e *AppError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s error in %s [%s]: %s", e.Type, e.Operation, e.Path, e.Message)
	}
	return fmt.Sprintf("%s error in %s: %s", e.Type, e.Operation, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new configuration error
func NewConfigError(operation, message string, err error) *AppError {
	return &AppError{
		Type:      ErrorTypeConfig,
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}

// NewFileSystemError creates a new filesystem error
func NewFileSystemError(operation, path, message string, err error) *AppError {
	return &AppError{
		Type:      ErrorTypeFileSystem,
		Operation: operation,
		Path:      path,
		Message:   message,
		Err:       err,
	}
}

// NewUIError creates a new UI error
func NewUIError(operation, message string, err error) *AppError {
	return &AppError{
		Type:      ErrorTypeUI,
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}

// NewWatcherError creates a new watcher error
func NewWatcherError(operation, path, message string, err error) *AppError {
	return &AppError{
		Type:      ErrorTypeWatcher,
		Operation: operation,
		Path:      path,
		Message:   message,
		Err:       err,
	}
}

// NewThemeError creates a new theme error
func NewThemeError(operation, message string, err error) *AppError {
	return &AppError{
		Type:      ErrorTypeTheme,
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}
