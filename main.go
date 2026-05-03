package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2/app"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	customtheme "nmf/internal/theme"
)

// Global debug flag
var debugMode bool

// Global window registry for managing multiple windows
var (
	windowRegistry sync.Map // map[fyne.Window]*FileManager
	windowCount    int32    // atomic counter for window count
)

// debugPrint prints debug messages only when debug mode is enabled
func debugPrint(format string, args ...interface{}) {
	if debugMode {
		log.Printf("DEBUG: "+format, args...)
	}
}

func setupDebugLogging(path string) (*os.File, error) {
	if path == "" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating debug log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	debugMode = true
	log.SetOutput(io.MultiWriter(file, os.Stderr))
	debugPrint("Logger: debug log started path=%s time=%s", path, time.Now().Format(time.RFC3339))
	return file, nil
}

func main() {
	fileinfo.CleanupOldArchiveOpenTemps()

	// Parse command line flags
	var startPath string
	var debugLogPath string
	flag.BoolVar(&debugMode, "d", false, "Enable debug mode")
	flag.StringVar(&debugLogPath, "debug-log", "", "Write debug logs to the specified file")
	flag.StringVar(&startPath, "path", "", "Starting directory path")
	flag.Parse()

	debugLogFile, err := setupDebugLogging(debugLogPath)
	if err != nil {
		log.Fatalf("Error opening debug log '%s': %v", debugLogPath, err)
	}
	if debugLogFile != nil {
		defer func() {
			if err := debugLogFile.Close(); err != nil {
				log.Printf("Error closing debug log: %v", err)
			}
		}()
	}

	// If no path specified via flag, check remaining arguments
	if startPath == "" && flag.NArg() > 0 {
		startPath = flag.Arg(0)
	}

	// If still no path, use current working directory
	if startPath == "" {
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current directory: %v", err)
		}
		resolvedStartPath, _, err := resolveDirectoryPath(pwd)
		if err != nil {
			log.Fatalf("Error accessing current directory '%s': %v", pwd, err)
		}
		startPath = resolvedStartPath
	} else {
		if strings.HasPrefix(startPath, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("Error getting home directory: %v", err)
			}
			startPath = strings.Replace(startPath, "~", home, 1)
		}
		resolvedStartPath, _, err := resolveDirectoryPath(startPath)
		if err != nil {
			log.Fatalf("Error accessing path '%s': %v", startPath, err)
		}
		startPath = resolvedStartPath
	}

	// Load configuration
	configManager := config.NewManager(debugPrint)
	defer func() {
		if err := configManager.Close(); err != nil {
			log.Printf("Error closing config manager: %v", err)
		}
	}()
	config, err := configManager.Load()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	app := app.NewWithID(appID)
	app.SetIcon(appIconResource)

	// Apply custom theme
	customTheme := customtheme.NewCustomTheme(config, debugPrint)
	app.Settings().SetTheme(customTheme)

	// Install debug logger for jobs package (prints only in -d mode)
	jobs.SetDebug(debugPrint)

	fm := NewFileManager(app, startPath, config, configManager, customTheme)
	fm.window.ShowAndRun()
}

// resolveDirectoryPath resolves user input into a path suitable for LoadDirectory.
// Local paths are validated as existing directories. SMB paths are normalized to canonical smb:// form.
func resolveDirectoryPath(input string) (string, fileinfo.Parsed, error) {
	return fileinfo.ResolveDirectoryPath(input)
}
