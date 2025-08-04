package errors

import (
	"errors"
	"testing"
)

func TestErrorTypeString(t *testing.T) {
	testCases := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorTypeConfig, "config"},
		{ErrorTypeFileSystem, "filesystem"},
		{ErrorTypeUI, "ui"},
		{ErrorTypeWatcher, "watcher"},
		{ErrorTypeTheme, "theme"},
		{ErrorType(999), "unknown"}, // Invalid error type
	}

	for _, tc := range testCases {
		result := tc.errorType.String()
		if result != tc.expected {
			t.Errorf("For error type %v, expected '%s', got '%s'", tc.errorType, tc.expected, result)
		}
	}
}

func TestAppErrorError(t *testing.T) {
	// Test error with path
	err := &AppError{
		Type:      ErrorTypeFileSystem,
		Operation: "read_directory",
		Path:      "/home/user/documents",
		Message:   "permission denied",
		Err:       errors.New("access denied"),
	}

	expected := "filesystem error in read_directory [/home/user/documents]: permission denied"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}

	// Test error without path
	err2 := &AppError{
		Type:      ErrorTypeConfig,
		Operation: "load_config",
		Message:   "invalid JSON",
		Err:       errors.New("syntax error"),
	}

	expected2 := "config error in load_config: invalid JSON"
	if err2.Error() != expected2 {
		t.Errorf("Expected error message '%s', got '%s'", expected2, err2.Error())
	}
}

func TestAppErrorUnwrap(t *testing.T) {
	originalErr := errors.New("original error")
	appErr := &AppError{
		Type:      ErrorTypeUI,
		Operation: "render",
		Message:   "rendering failed",
		Err:       originalErr,
	}

	unwrapped := appErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Expected unwrapped error to be original error, got %v", unwrapped)
	}

	// Test with nil wrapped error
	appErr2 := &AppError{
		Type:      ErrorTypeUI,
		Operation: "render",
		Message:   "rendering failed",
		Err:       nil,
	}

	unwrapped2 := appErr2.Unwrap()
	if unwrapped2 != nil {
		t.Errorf("Expected unwrapped error to be nil, got %v", unwrapped2)
	}
}

func TestNewConfigError(t *testing.T) {
	originalErr := errors.New("file not found")
	appErr := NewConfigError("load_settings", "configuration file missing", originalErr)

	if appErr.Type != ErrorTypeConfig {
		t.Errorf("Expected error type config, got %v", appErr.Type)
	}
	if appErr.Operation != "load_settings" {
		t.Errorf("Expected operation 'load_settings', got '%s'", appErr.Operation)
	}
	if appErr.Message != "configuration file missing" {
		t.Errorf("Expected message 'configuration file missing', got '%s'", appErr.Message)
	}
	if appErr.Err != originalErr {
		t.Errorf("Expected wrapped error to be original error, got %v", appErr.Err)
	}
	if appErr.Path != "" {
		t.Errorf("Expected empty path, got '%s'", appErr.Path)
	}
}

func TestNewFileSystemError(t *testing.T) {
	originalErr := errors.New("permission denied")
	appErr := NewFileSystemError("read_file", "/home/user/secret.txt", "access denied", originalErr)

	if appErr.Type != ErrorTypeFileSystem {
		t.Errorf("Expected error type filesystem, got %v", appErr.Type)
	}
	if appErr.Operation != "read_file" {
		t.Errorf("Expected operation 'read_file', got '%s'", appErr.Operation)
	}
	if appErr.Path != "/home/user/secret.txt" {
		t.Errorf("Expected path '/home/user/secret.txt', got '%s'", appErr.Path)
	}
	if appErr.Message != "access denied" {
		t.Errorf("Expected message 'access denied', got '%s'", appErr.Message)
	}
	if appErr.Err != originalErr {
		t.Errorf("Expected wrapped error to be original error, got %v", appErr.Err)
	}
}

func TestNewUIError(t *testing.T) {
	originalErr := errors.New("widget creation failed")
	appErr := NewUIError("create_widget", "failed to initialize component", originalErr)

	if appErr.Type != ErrorTypeUI {
		t.Errorf("Expected error type UI, got %v", appErr.Type)
	}
	if appErr.Operation != "create_widget" {
		t.Errorf("Expected operation 'create_widget', got '%s'", appErr.Operation)
	}
	if appErr.Message != "failed to initialize component" {
		t.Errorf("Expected message 'failed to initialize component', got '%s'", appErr.Message)
	}
	if appErr.Err != originalErr {
		t.Errorf("Expected wrapped error to be original error, got %v", appErr.Err)
	}
}

func TestNewWatcherError(t *testing.T) {
	originalErr := errors.New("filesystem watch failed")
	appErr := NewWatcherError("start_watching", "/home/user/watched", "unable to monitor directory", originalErr)

	if appErr.Type != ErrorTypeWatcher {
		t.Errorf("Expected error type watcher, got %v", appErr.Type)
	}
	if appErr.Operation != "start_watching" {
		t.Errorf("Expected operation 'start_watching', got '%s'", appErr.Operation)
	}
	if appErr.Path != "/home/user/watched" {
		t.Errorf("Expected path '/home/user/watched', got '%s'", appErr.Path)
	}
	if appErr.Message != "unable to monitor directory" {
		t.Errorf("Expected message 'unable to monitor directory', got '%s'", appErr.Message)
	}
	if appErr.Err != originalErr {
		t.Errorf("Expected wrapped error to be original error, got %v", appErr.Err)
	}
}

func TestNewThemeError(t *testing.T) {
	originalErr := errors.New("font loading failed")
	appErr := NewThemeError("load_font", "custom font could not be loaded", originalErr)

	if appErr.Type != ErrorTypeTheme {
		t.Errorf("Expected error type theme, got %v", appErr.Type)
	}
	if appErr.Operation != "load_font" {
		t.Errorf("Expected operation 'load_font', got '%s'", appErr.Operation)
	}
	if appErr.Message != "custom font could not be loaded" {
		t.Errorf("Expected message 'custom font could not be loaded', got '%s'", appErr.Message)
	}
	if appErr.Err != originalErr {
		t.Errorf("Expected wrapped error to be original error, got %v", appErr.Err)
	}
}

func TestErrorChaining(t *testing.T) {
	// Test that errors.Is works with our custom error
	originalErr := errors.New("original")
	appErr := NewConfigError("test", "test message", originalErr)

	if !errors.Is(appErr, originalErr) {
		t.Error("errors.Is should work with AppError")
	}

	// Test that errors.As works with our custom error
	var appErrPtr *AppError
	if !errors.As(appErr, &appErrPtr) {
		t.Error("errors.As should work with AppError")
	}
	if appErrPtr.Type != ErrorTypeConfig {
		t.Error("errors.As should preserve the correct error type")
	}
}
