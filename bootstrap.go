package main

import (
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"nmf/internal/config"
	"nmf/internal/configscript"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

func NewFileManager(runtime *ApplicationRuntime, path string, config *config.Config, configManager *config.Manager, state *config.State, stateManager *config.StateManager, customTheme *customtheme.CustomTheme, configScript *configscript.Runtime) *FileManager {
	if runtime == nil || runtime.app == nil {
		panic("NewFileManager requires an application runtime")
	}
	fm := &FileManager{
		window:            runtime.app.NewWindow("File Manager"),
		currentPath:       path,
		cursorPath:        "",
		cursorIndex:       -1,
		selectedFiles:     make(map[string]bool),
		config:            config,
		configManager:     configManager,
		state:             state,
		stateManager:      stateManager,
		configScript:      configScript,
		initialWindowSize: fyne.NewSize(float32(config.Window.Width), float32(config.Window.Height)),
		windowActive:      true,
		activeSort:        state.EffectiveSort(config.UI.Sort),
		customTheme:       customTheme,
		cursorRenderer:    ui.NewCursorRenderer(config.UI.CursorStyle),
		keyManager:        keymanager.NewKeyManager(debugPrint),
		searchMatchers:    search.NewProvider(debugPrint),
		runtime:           runtime,
	}

	// Busy overlay (hidden by default)
	fm.busyOverlay = ui.NewBusyOverlay(customTheme)
	fm.busyDelay = 150 * time.Millisecond

	// Initialize async icon service and subscribe for updates
	fm.iconSvc = fileinfo.NewIconService(debugPrint)
	// Refresh the list when icons arrive. Icon notifications are emitted from
	// background workers, so widget refreshes must run on the Fyne call thread.
	fm.iconSvc.OnUpdated(func() {
		if fm.isWindowClosed() {
			return
		}
		fyne.Do(func() {
			if !fm.isWindowClosed() && fm.fileList != nil {
				canvas.Refresh(fm.fileList)
			}
		})
	})

	// Create directory watcher
	fm.dirWatcher = watcher.NewDirectoryWatcher(fm, runtime.watchHub, debugPrint)

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
	mainHandler.SetActions(keymanager.DialogActions{
		ShowDirectoryTreeDialog:     fm.ShowDirectoryTreeDialog,
		ShowNavigationHistoryDialog: fm.ShowNavigationHistoryDialog,
		ShowDirectoryJumpDialog:     fm.ShowDirectoryJumpDialog,
		ShowFilterDialog:            fm.ShowFilterDialog,
		ShowIncrementalSearchDialog: fm.ShowIncrementalSearchDialog,
		ShowSortDialog:              fm.ShowSortDialog,
		ShowJobsDialog:              fm.ShowJobsDialog,
		ShowPathEditDialog:          fm.ShowPathEditDialog,
		ShowCreateDirectoryDialog:   fm.ShowCreateDirectoryDialog,
		ShowClipboardTextFileDialog: fm.ShowClipboardTextFileDialog,
		ShowMessageDialog:           fm.ShowMessageDialog,
		ShowCopyDialog:              fm.ShowCopyDialog,
		ShowMoveDialog:              fm.ShowMoveDialog,
		ShowExtractArchiveDialog:    fm.ShowExtractArchiveDialog,
		ShowCompareDialog:           fm.ShowCompareDialog,
		ShowRenameDialog:            fm.ShowRenameDialog,
		ShowDeleteDialog:            fm.ShowDeleteDialog,
		ShowExplorerContextMenu:     fm.ShowExplorerContextMenu,
		ShowExternalCommandMenu:     fm.ShowExternalCommandMenu,
		ShowFileViewer:              fm.ShowFileViewer,
		ShowMaintenanceDialog:       fm.ShowMaintenanceDialog,
		ShowCommandMenu:             fm.ShowCommandMenu,
	})
	fm.mainKeyHandler = mainHandler
	fm.keyManager.PushHandler(mainHandler)

	fm.setupUI()
	runtime.registerWindowPrompts(fm)
	fm.LoadDirectory(path)

	// Register window in global registry
	registerFileManagerWindow(fm)
	atomic.AddInt32(&windowCount, 1)

	// Set window close handler
	fm.window.SetCloseIntercept(func() {
		fm.QuitApplication()
	})

	return fm
}
