package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nmf/internal/fileinfo"
)

func TestSetupDebugLoggingEmptyPathNoop(t *testing.T) {
	oldDebugMode := debugMode
	oldOutput := log.Writer()
	defer func() {
		debugMode = oldDebugMode
		log.SetOutput(oldOutput)
	}()

	debugMode = false
	var output bytes.Buffer
	log.SetOutput(&output)

	file, err := setupDebugLogging("")
	if err != nil {
		t.Fatalf("setupDebugLogging returned error: %v", err)
	}
	if file != nil {
		t.Fatal("expected no log file for empty path")
	}
	if debugMode {
		t.Fatal("expected debug mode to remain disabled")
	}
	if output.Len() != 0 {
		t.Fatalf("expected no log output, got %q", output.String())
	}
}

func TestSetupDebugLoggingCreatesParentAndEnablesDebug(t *testing.T) {
	oldDebugMode := debugMode
	oldOutput := log.Writer()
	defer func() {
		debugMode = oldDebugMode
		log.SetOutput(oldOutput)
	}()

	debugMode = false
	path := filepath.Join(t.TempDir(), "nested", "debug.log")
	file, err := setupDebugLogging(path)
	if err != nil {
		t.Fatalf("setupDebugLogging returned error: %v", err)
	}
	defer file.Close()

	if !debugMode {
		t.Fatal("expected debug mode to be enabled")
	}
	debugPrint("Test: hello")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read debug log: %v", err)
	}
	logText := string(data)
	if !strings.Contains(logText, "DEBUG: Logger: debug log started path="+path) {
		t.Fatalf("expected startup log in %q", logText)
	}
	if !strings.Contains(logText, "DEBUG: Test: hello") {
		t.Fatalf("expected debugPrint output in %q", logText)
	}
}

func TestSetupDebugLoggingAppends(t *testing.T) {
	oldDebugMode := debugMode
	oldOutput := log.Writer()
	defer func() {
		debugMode = oldDebugMode
		log.SetOutput(oldOutput)
	}()

	debugMode = false
	path := filepath.Join(t.TempDir(), "debug.log")
	if err := os.WriteFile(path, []byte("existing\n"), 0644); err != nil {
		t.Fatalf("failed to seed debug log: %v", err)
	}

	file, err := setupDebugLogging(path)
	if err != nil {
		t.Fatalf("setupDebugLogging returned error: %v", err)
	}
	defer file.Close()

	debugPrint("Test: appended")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read debug log: %v", err)
	}
	logText := string(data)
	if !strings.HasPrefix(logText, "existing\n") {
		t.Fatalf("expected existing content to be preserved, got %q", logText)
	}
	if !strings.Contains(logText, "DEBUG: Test: appended") {
		t.Fatalf("expected appended debug output in %q", logText)
	}
}

func TestSetupDebugLoggingInvalidPath(t *testing.T) {
	oldDebugMode := debugMode
	oldOutput := log.Writer()
	defer func() {
		debugMode = oldDebugMode
		log.SetOutput(oldOutput)
	}()

	debugMode = false
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create parent file: %v", err)
	}

	file, err := setupDebugLogging(filepath.Join(parentFile, "debug.log"))
	if err == nil {
		if file != nil {
			file.Close()
		}
		t.Fatal("expected invalid path to fail")
	}
	if debugMode {
		t.Fatal("expected debug mode to remain disabled")
	}
}

func TestResolveDirectoryPath_LocalDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	resolved, parsed, err := resolveDirectoryPath(tmpDir)
	if err != nil {
		t.Fatalf("expected directory to resolve: %v", err)
	}
	if parsed.Scheme != fileinfo.SchemeFile {
		t.Fatalf("expected file scheme, got %q", parsed.Scheme)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("expected absolute local path, got %q", resolved)
	}
}

func TestResolveDirectoryPath_LocalFileRejected(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if _, _, err := resolveDirectoryPath(filePath); err == nil {
		t.Fatalf("expected non-directory path to fail")
	}
}

func TestResolveDirectoryPath_SMBCanonicalDisplay(t *testing.T) {
	input := "smb://example.local/share/path/to/dir"

	resolved, parsed, err := resolveDirectoryPath(input)
	if err != nil {
		t.Fatalf("expected SMB path parse to succeed: %v", err)
	}
	if parsed.Scheme != fileinfo.SchemeSMB {
		t.Fatalf("expected smb scheme, got %q", parsed.Scheme)
	}
	if !strings.HasPrefix(strings.ToLower(resolved), "smb://example.local/share") {
		t.Fatalf("unexpected SMB resolved path: %q", resolved)
	}
}

func TestResolveDirectoryPath_EmptyRejected(t *testing.T) {
	if _, _, err := resolveDirectoryPath("   "); err == nil {
		t.Fatalf("expected empty path to fail")
	}
}

func TestSameDirectoryPath_LocalCleanedPath(t *testing.T) {
	tmpDir := t.TempDir()

	if !sameDirectoryPath(filepath.Join(tmpDir, "."), tmpDir) {
		t.Fatalf("expected cleaned local paths to match")
	}
}

func TestSameDirectoryPath_SMBNormalizedPath(t *testing.T) {
	if !sameDirectoryPath("smb://Example.Local/share/path/", "smb://example.local/share/path") {
		t.Fatalf("expected normalized SMB paths to match")
	}
}

func TestSameDirectoryPath_EmptyDoesNotMatch(t *testing.T) {
	if sameDirectoryPath("", "") {
		t.Fatalf("expected empty paths not to match")
	}
}
