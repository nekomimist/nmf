package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nmf/internal/config"
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

func TestSelectStartupPathUsesCLIPathFirst(t *testing.T) {
	got, err := selectStartupPath("/cli", true, &config.Config{
		Startup: config.StartupConfig{Directory: "/config"},
	})
	if err != nil {
		t.Fatalf("selectStartupPath returned error: %v", err)
	}
	if got != "/cli" {
		t.Fatalf("startup path = %q, want /cli", got)
	}
}

func TestSelectStartupPathUsesConfiguredDirectory(t *testing.T) {
	got, err := selectStartupPath("", false, &config.Config{
		Startup: config.StartupConfig{Directory: "/config"},
	})
	if err != nil {
		t.Fatalf("selectStartupPath returned error: %v", err)
	}
	if got != "/config" {
		t.Fatalf("startup path = %q, want /config", got)
	}
}

func TestSelectStartupPathFallsBackToWorkingDirectory(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	got, err := selectStartupPath("", false, &config.Config{})
	if err != nil {
		t.Fatalf("selectStartupPath returned error: %v", err)
	}
	if got != wd {
		t.Fatalf("startup path = %q, want working directory %q", got, wd)
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

func TestSetupRotatingDebugLoggingCreatesSessionLogAndPrunes(t *testing.T) {
	oldDebugMode := debugMode
	oldOutput := log.Writer()
	defer func() {
		debugMode = oldDebugMode
		log.SetOutput(oldOutput)
	}()

	debugMode = false
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	for i := 0; i < 3; i++ {
		path := filepath.Join(logDir, "nmf-20260101-00000"+string(rune('0'+i))+"-1.log")
		if err := os.WriteFile(path, []byte("old\n"), 0644); err != nil {
			t.Fatalf("failed to seed log: %v", err)
		}
		when := time.Unix(int64(i+1), 0)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatalf("failed to set log time: %v", err)
		}
	}

	file, path, err := setupRotatingDebugLogging(configPath, config.DebugConfig{
		Enabled:     true,
		MaxLogFiles: 2,
	})
	if err != nil {
		t.Fatalf("setupRotatingDebugLogging returned error: %v", err)
	}
	defer file.Close()

	if !debugMode {
		t.Fatal("expected debug mode to be enabled")
	}
	if !strings.HasPrefix(path, logDir+string(os.PathSeparator)) {
		t.Fatalf("log path = %q, want under %q", path, logDir)
	}
	debugPrint("Test: rotating")

	matches, err := filepath.Glob(filepath.Join(logDir, "nmf-*.log"))
	if err != nil {
		t.Fatalf("Glob failed: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("rotating logs = %#v, want 2 files", matches)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read new log: %v", err)
	}
	if !strings.Contains(string(data), "DEBUG: Test: rotating") {
		t.Fatalf("new log = %q, want debug output", string(data))
	}
}

func TestResolveDebugLogDirectory(t *testing.T) {
	configPath := filepath.Join("home", "nekomimist", "nmf", "config.json")

	if got := resolveDebugLogDirectory(configPath, ""); got != filepath.Join("home", "nekomimist", "nmf", "logs") {
		t.Fatalf("default log dir = %q", got)
	}
	if got := resolveDebugLogDirectory(configPath, "custom"); got != filepath.Join("home", "nekomimist", "nmf", "custom") {
		t.Fatalf("relative log dir = %q", got)
	}
	abs := filepath.Join(string(os.PathSeparator), "tmp", "nmf-logs")
	if got := resolveDebugLogDirectory(configPath, abs); got != abs {
		t.Fatalf("absolute log dir = %q", got)
	}
}

func TestOpenDebugSessionLogUsesSuffixOnCollision(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 6, 8, 21, 30, 0, 0, time.UTC)
	first := filepath.Join(dir, debugLogFileName(now, 123))
	if err := os.WriteFile(first, []byte("existing\n"), 0644); err != nil {
		t.Fatalf("failed to seed log: %v", err)
	}

	file, path, err := openDebugSessionLog(dir, now, 123)
	if err != nil {
		t.Fatalf("openDebugSessionLog returned error: %v", err)
	}
	defer file.Close()

	want := filepath.Join(dir, "nmf-20260608-213000-123-1.log")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
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
