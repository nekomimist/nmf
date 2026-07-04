package main

import (
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/data/binding"

	"nmf/internal/config"
	"nmf/internal/configscript"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
	"nmf/internal/secret"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

func NewFileManager(app fyne.App, path string, config *config.Config, configManager *config.Manager, customTheme *customtheme.CustomTheme, configScript *configscript.Runtime, watchHub *watcher.WatchHub) *FileManager {
	if watchHub == nil {
		watchHub = watcher.NewWatchHub(debugPrint)
	}
	fm := &FileManager{
		window:            app.NewWindow("File Manager"),
		currentPath:       path,
		cursorPath:        "",
		cursorIndex:       -1,
		selectedFiles:     make(map[string]bool),
		fileBinding:       binding.NewUntypedList(),
		config:            config,
		configManager:     configManager,
		configScript:      configScript,
		initialWindowSize: fyne.NewSize(float32(config.Window.Width), float32(config.Window.Height)),
		windowActive:      true,
		activeSort:        config.UI.Sort,
		customTheme:       customTheme,
		cursorRenderer:    ui.NewCursorRenderer(config.UI.CursorStyle),
		keyManager:        keymanager.NewKeyManager(debugPrint),
		searchMatchers:    search.NewProvider(debugPrint),
		watchHub:          watchHub,
	}

	// Busy overlay (hidden by default)
	fm.busyOverlay = ui.NewBusyOverlay(customTheme)
	fm.busyDelay = 150 * time.Millisecond

	// Initialize async icon service and subscribe for updates
	fm.iconSvc = fileinfo.NewIconService(debugPrint)
	// Refresh the list when icons arrive. Icon notifications are emitted from
	// background workers, so widget refreshes must run on the Fyne call thread.
	fm.iconSvc.OnUpdated(func() {
		fyne.Do(func() {
			if fm.fileList != nil {
				canvas.Refresh(fm.fileList)
			}
		})
	})

	// Create directory watcher
	fm.dirWatcher = watcher.NewDirectoryWatcher(fm, watchHub, debugPrint)

	// Install SMB credentials provider (cached + interactive prompt fallback)
	cached := fileinfo.NewCachedCredentialsProvider(ui.NewSMBCredentialsProvider(fm.window, fm.keyManager, config.UI.KeyBindings))
	fileinfo.SetCredentialsProvider(cached)
	fileinfo.SetArchivePasswordProvider(fileinfo.NewCachedArchivePasswordProvider(ui.NewArchivePasswordProvider(fm.window)))

	// Initialize OS keyring (99designs). If unavailable, continue without persistent store.
	if store, err := secret.NewKeyringStore(); err == nil {
		fileinfo.SetSecretStore(store)
	}

	// Create incremental search overlay
	fm.searchOverlay = ui.NewIncrementalSearchOverlay([]fileinfo.FileInfo{}, fm.keyManager, customTheme, debugPrint, fm.searchMatchers)
	fm.searchHandler = keymanager.NewIncrementalSearchKeyHandler(fm, debugPrint)
	fm.searchHandler.SetTransitionGate(fm.keyManager.BeginOwnerTransition)

	// Setup KeyManager with main screen handler
	keymanager.WarnUnknownKeyBindingTargets(config.UI.KeyBindings, debugPrint)
	var scriptCommands keymanager.CommandRegistry
	if configScript != nil {
		scriptCommands = configScript.Commands
	}
	mainHandler := keymanager.NewMainScreenKeyHandlerWithCommands(fm, debugPrint, config.UI.KeyBindings, scriptCommands)
	mainHandler.SetTransitionGate(fm.keyManager.BeginOwnerTransition)
	fm.mainKeyHandler = mainHandler
	fm.keyManager.PushHandler(mainHandler)

	fm.setupUI()
	fm.LoadDirectory(path)

	// Register window in global registry
	registerFileManagerWindow(fm)
	atomic.AddInt32(&windowCount, 1)

	// Set window close handler
	fm.window.SetCloseIntercept(func() {
		fm.closeWindow()
	})

	return fm
}
