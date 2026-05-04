package main

import (
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/data/binding"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/secret"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

func NewFileManager(app fyne.App, path string, config *config.Config, configManager *config.Manager, customTheme *customtheme.CustomTheme) *FileManager {
	fm := &FileManager{
		window:         app.NewWindow("File Manager"),
		currentPath:    path,
		cursorPath:     "",
		selectedFiles:  make(map[string]bool),
		fileBinding:    binding.NewUntypedList(),
		config:         config,
		configManager:  configManager,
		customTheme:    customTheme,
		cursorRenderer: ui.NewCursorRenderer(config.UI.CursorStyle),
		keyManager:     keymanager.NewKeyManager(debugPrint),
	}

	// Busy overlay (hidden by default)
	fm.busyOverlay = ui.NewBusyOverlay()
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
	fm.dirWatcher = watcher.NewDirectoryWatcher(fm, debugPrint)

	// Install SMB credentials provider (cached + interactive prompt fallback)
	cached := fileinfo.NewCachedCredentialsProvider(ui.NewSMBCredentialsProvider(fm.window))
	fileinfo.SetCredentialsProvider(cached)

	// Initialize OS keyring (99designs). If unavailable, continue without persistent store.
	if store, err := secret.NewKeyringStore(); err == nil {
		fileinfo.SetSecretStore(store)
	}

	// Create incremental search overlay
	fm.searchOverlay = ui.NewIncrementalSearchOverlay([]fileinfo.FileInfo{}, fm.keyManager, debugPrint)
	fm.searchHandler = keymanager.NewIncrementalSearchKeyHandler(fm, debugPrint)

	// Setup KeyManager with main screen handler
	mainHandler := keymanager.NewMainScreenKeyHandler(fm, debugPrint, config.UI.KeyBindings)
	fm.keyManager.PushHandler(mainHandler)

	fm.setupUI()
	fm.LoadDirectory(path)

	// Start watching after initial load
	fm.dirWatcher.Start()

	// Register window in global registry
	windowRegistry.Store(fm.window, fm)
	atomic.AddInt32(&windowCount, 1)

	// Set window close handler
	fm.window.SetCloseIntercept(func() {
		fm.closeWindow()
	})

	return fm
}
