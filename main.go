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
	"nmf/internal/configscript"
	"nmf/internal/display"
	"nmf/internal/fileinfo"
	"nmf/internal/ime"
	"nmf/internal/jobs"
	"nmf/internal/shellmenu"
	customtheme "nmf/internal/theme"
)

// Global debug flag
var debugMode bool

// Global window registry for managing multiple windows
var (
	windowRegistry sync.Map // map[fyne.Window]*FileManager
	windowCount    int32    // atomic counter for window count
	windowOrderMu  sync.Mutex
	windowOrder    []*FileManager
	reopenPaths    []string
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
	debugPrint("App: version=%s", appVersion())
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
	cliDebugMode := debugMode

	var debugLogFile *os.File
	debugLogFile, err := setupDebugLogging(debugLogPath)
	if err != nil {
		log.Fatalf("Error opening debug log '%s': %v", debugLogPath, err)
	}
	if debugLogFile == nil {
		debugPrint("App: version=%s", appVersion())
	}
	defer func() {
		if debugLogFile != nil {
			if err := debugLogFile.Close(); err != nil {
				log.Printf("Error closing debug log: %v", err)
			}
		}
	}()

	// If no path specified via flag, check remaining arguments
	if startPath == "" && flag.NArg() > 0 {
		startPath = flag.Arg(0)
	}
	cliStartPath := startPath != ""

	// Load configuration
	configManager := config.NewManager(debugPrint)
	defer func() {
		if err := configManager.Close(); err != nil {
			log.Printf("Error closing config manager: %v", err)
		}
	}()
	cfg, err := configManager.Load()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	activeConfigLogDir := ""
	activeConfigLogMax := 0
	applyConfigDebug := func(debugCfg config.DebugConfig) error {
		if debugLogPath != "" {
			return nil
		}
		logDir := resolveDebugLogDirectory(configManager.ConfigPath(), debugCfg.LogDirectory)
		if debugCfg.Enabled && debugLogFile != nil && logDir == activeConfigLogDir && debugCfg.MaxLogFiles == activeConfigLogMax {
			return nil
		}
		if debugLogFile != nil {
			if err := debugLogFile.Close(); err != nil {
				return fmt.Errorf("closing debug log: %w", err)
			}
			debugLogFile = nil
			activeConfigLogDir = ""
			activeConfigLogMax = 0
		}
		if !debugCfg.Enabled {
			debugMode = cliDebugMode
			log.SetOutput(os.Stderr)
			return nil
		}
		file, _, err := setupRotatingDebugLogging(configManager.ConfigPath(), debugCfg)
		if err != nil {
			return err
		}
		debugLogFile = file
		activeConfigLogDir = logDir
		activeConfigLogMax = debugCfg.MaxLogFiles
		return nil
	}
	if err := applyConfigDebug(cfg.Debug); err != nil {
		log.Fatalf("Error opening configured debug log: %v", err)
	}
	persistentConfig := config.Clone(cfg)
	displayInfo := display.Primary(debugPrint)
	configScript, err := configscript.LoadWithDisplayAndDebugHook(configscript.ScriptPath(configManager.ConfigPath()), cfg, displayInfo, debugPrint, applyConfigDebug)
	if err != nil {
		log.Printf("Error loading Starlark configuration: %v", err)
		showStartupConfigScriptErrorAndExit(cfg, err)
		return
	}
	if configScript.Loaded() {
		configManager.SetSaveTransform(configScript.SaveTransform(persistentConfig))
	}
	if err := fileinfo.SetArchiveOptions(fileinfo.ArchiveOptions{ZipNameEncoding: cfg.UI.Archive.ZipNameEncoding}); err != nil {
		debugPrint("Config: Invalid archive ZIP name encoding %q: %v; using %s", cfg.UI.Archive.ZipNameEncoding, err, fileinfo.DefaultArchiveZipNameEncoding)
		_ = fileinfo.SetArchiveOptions(fileinfo.ArchiveOptions{ZipNameEncoding: fileinfo.DefaultArchiveZipNameEncoding})
	}
	startPath, err = selectStartupPath(startPath, cliStartPath, cfg)
	if err != nil {
		log.Fatalf("Error selecting startup path: %v", err)
	}
	resolvedStartPath, _, err := resolveDirectoryPath(startPath)
	if err != nil {
		log.Fatalf("Error accessing path '%s': %v", startPath, err)
	}
	startPath = resolvedStartPath
	ime.SetEnabled(cfg.UI.IME.Enabled)
	debugPrint("Config: IME integration enabled=%t", cfg.UI.IME.Enabled)

	fyneApp := app.NewWithID(appID)
	fyneApp.SetIcon(appIconResource)

	// Apply custom theme
	customTheme := customtheme.NewCustomTheme(cfg, debugPrint)
	fyneApp.Settings().SetTheme(customTheme)

	// Install debug logger for jobs package (prints only in -d mode)
	jobs.SetDebug(debugPrint)
	shellmenu.Debugf = debugPrint

	fm := NewFileManager(fyneApp, startPath, cfg, configManager, customTheme, configScript)
	fm.window.Show()
	applyInitialWindowPosition(fm.window, cfg.Window)
	fyneApp.Run()
}

// resolveDirectoryPath resolves user input into a path suitable for LoadDirectory.
// Local paths are validated as existing directories. SMB paths are normalized to canonical smb:// form.
func resolveDirectoryPath(input string) (string, fileinfo.Parsed, error) {
	return fileinfo.ResolveDirectoryPath(input)
}

func selectStartupPath(cliPath string, cliSpecified bool, cfg *config.Config) (string, error) {
	if cliSpecified {
		return expandHomePath(cliPath)
	}
	if cfg != nil && strings.TrimSpace(cfg.Startup.Directory) != "" {
		return expandHomePath(cfg.Startup.Directory)
	}
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	return pwd, nil
}

func expandHomePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return strings.Replace(path, "~", home, 1), nil
}
