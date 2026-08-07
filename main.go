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
	var profileDir string
	var configDirOverride string
	var stateDirOverride string
	flag.BoolVar(&debugMode, "d", false, "Enable debug mode")
	flag.StringVar(&debugLogPath, "debug-log", "", "Write debug logs to the specified file")
	flag.StringVar(&startPath, "path", "", "Starting directory path")
	flag.StringVar(&profileDir, "profile", "", "Use an isolated profile directory for config.json, init.star, logs, and state.json")
	flag.StringVar(&configDirOverride, "config-dir", "", "Override the config directory (config.json, init.star, default log directory); takes precedence over -profile")
	flag.StringVar(&stateDirOverride, "state-dir", "", "Override the state directory (state.json); takes precedence over -profile")
	flag.Parse()
	cliDebugMode := debugMode

	var debugLogFile *os.File
	debugLogFile, err := setupDebugLogging(debugLogPath)
	if err != nil {
		log.Printf("Error opening debug log '%s': %v", debugLogPath, err)
		showStartupErrorAndExit(nil, "startup error", startupFailureMessage(fmt.Sprintf("Failed to open debug log '%s'", debugLogPath), err))
		return
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

	// Resolve -profile/-config-dir/-state-dir into effective config.json and
	// state.json paths (empty means "use the OS default location").
	overrideConfigPath, overrideStatePath, err := resolveDataDirs(profileDir, configDirOverride, stateDirOverride)
	if err != nil {
		log.Printf("Error resolving data directory overrides: %v", err)
		showStartupErrorAndExit(nil, "startup error", startupFailureMessage("Invalid -profile/-config-dir/-state-dir", err))
		return
	}
	if overrideConfigPath != "" || overrideStatePath != "" {
		debugPrint("Config: data dir override config=%s state=%s", overrideConfigPath, overrideStatePath)
	}

	// Load configuration
	var configManager *config.Manager
	if overrideConfigPath != "" {
		if err := os.MkdirAll(filepath.Dir(overrideConfigPath), 0755); err != nil {
			log.Printf("Error creating config directory '%s': %v", filepath.Dir(overrideConfigPath), err)
			showStartupErrorAndExit(nil, "startup error", startupFailureMessage(fmt.Sprintf("Failed to create config directory '%s'", filepath.Dir(overrideConfigPath)), err))
			return
		}
		configManager = config.NewManagerWithPath(overrideConfigPath, debugPrint)
	} else {
		configManager = config.NewManager(debugPrint)
	}
	cfg, err := configManager.Load()
	if err != nil {
		log.Printf("Error loading configuration: %v", err)
		showStartupErrorAndExit(nil, "config.json error", startupFailureMessage("Failed to load config.json", err))
		return
	}

	// Load runtime state (state.json), migrating legacy config.json runtime
	// keys on first run. config.json is never written back to; state.json
	// takes over all cursor memory/navigation history/file filter/sort
	// persistence from here on.
	var stateManager *config.StateManager
	if overrideStatePath != "" {
		if err := os.MkdirAll(filepath.Dir(overrideStatePath), 0755); err != nil {
			log.Printf("Error creating state directory '%s': %v", filepath.Dir(overrideStatePath), err)
			showStartupErrorAndExit(nil, "startup error", startupFailureMessage(fmt.Sprintf("Failed to create state directory '%s'", filepath.Dir(overrideStatePath)), err))
			return
		}
		stateManager = config.NewStateManagerWithPath(overrideStatePath, debugPrint)
	} else {
		stateManager = config.NewStateManager(debugPrint)
	}
	state, err := stateManager.Load(configManager.ConfigPath())
	if err != nil {
		log.Printf("Error loading runtime state: %v", err)
		showStartupErrorAndExit(cfg, "state.json error", startupFailureMessage("Failed to load runtime state (state.json)", err))
		return
	}
	defer func() {
		if err := stateManager.Close(); err != nil {
			log.Printf("Error closing state manager: %v", err)
		}
	}()
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
		log.Printf("Error opening configured debug log: %v", err)
		showStartupErrorAndExit(cfg, "startup error", startupFailureMessage("Failed to open configured debug log", err))
		return
	}
	applyDisplayDPIWorkaround()
	displayInfo := display.Primary(debugPrint)
	configScript, err := configscript.Load(configscript.ScriptPath(configManager.ConfigPath()), cfg, configscript.Options{
		Display:    displayInfo,
		DebugPrint: debugPrint,
		DebugHook:  applyConfigDebug,
	})
	if err != nil {
		log.Printf("Error loading Starlark configuration: %v", err)
		showStartupConfigScriptErrorAndExit(cfg, err)
		return
	}
	if err := fileinfo.SetArchiveOptions(fileinfo.ArchiveOptions{ZipNameEncoding: cfg.UI.Archive.ZipNameEncoding}); err != nil {
		debugPrint("Config: Invalid archive ZIP name encoding %q: %v; using %s", cfg.UI.Archive.ZipNameEncoding, err, fileinfo.DefaultArchiveZipNameEncoding)
		_ = fileinfo.SetArchiveOptions(fileinfo.ArchiveOptions{ZipNameEncoding: fileinfo.DefaultArchiveZipNameEncoding})
	}
	startPath, err = selectStartupPath(startPath, cliStartPath, cfg)
	if err != nil {
		log.Printf("Error selecting startup path: %v", err)
		showStartupErrorAndExit(cfg, "startup error", startupFailureMessage("Failed to select startup path", err))
		return
	}
	resolvedStartPath, _, err := resolveDirectoryPath(startPath)
	if err != nil {
		log.Printf("Error accessing path '%s': %v", startPath, err)
		showStartupErrorAndExit(cfg, "startup error", startupFailureMessage(fmt.Sprintf("Failed to access path '%s'", startPath), err))
		return
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

	runtime := newApplicationRuntime(fyneApp)
	fm := NewFileManager(runtime, startPath, cfg, configManager, state, stateManager, customTheme, configScript)
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

// resolveDataDirs expands the -profile/-config-dir/-state-dir CLI overrides
// into effective config.json and state.json paths. configDir and stateDir
// each take precedence over profile for their own piece; an empty result
// means "use the OS-default location" (see getConfigPath/getStatePath).
// Paths are left relative to CWD when given relative, matching -path's
// existing behavior; only "~" is expanded.
func resolveDataDirs(profile, configDir, stateDir string) (configPath, statePath string, err error) {
	profile, err = expandHomePath(profile)
	if err != nil {
		return "", "", fmt.Errorf("resolving -profile: %w", err)
	}
	configDir, err = expandHomePath(configDir)
	if err != nil {
		return "", "", fmt.Errorf("resolving -config-dir: %w", err)
	}
	stateDir, err = expandHomePath(stateDir)
	if err != nil {
		return "", "", fmt.Errorf("resolving -state-dir: %w", err)
	}

	effectiveConfigDir := configDir
	if effectiveConfigDir == "" {
		effectiveConfigDir = profile
	}
	if effectiveConfigDir != "" {
		configPath = filepath.Join(effectiveConfigDir, "config.json")
	}

	effectiveStateDir := stateDir
	if effectiveStateDir == "" {
		effectiveStateDir = profile
	}
	if effectiveStateDir != "" {
		statePath = filepath.Join(effectiveStateDir, "state.json")
	}

	return configPath, statePath, nil
}
