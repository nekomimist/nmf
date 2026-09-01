package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"nmf/internal/browser"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
	"nmf/internal/watcher"
)

func NewFileManager(runtime *ApplicationRuntime, path string) *FileManager {
	if runtime == nil || runtime.app == nil || runtime.config == nil || runtime.state == nil {
		panic("NewFileManager requires an application runtime")
	}
	config := runtime.config
	state := runtime.state
	customTheme := runtime.customTheme
	keyManager := keymanager.NewKeyManager(debugPrint)
	fm := &FileManager{
		window:            runtime.app.NewWindow("File Manager"),
		browser:           browser.New(path, state.EffectiveSort(config.UI.Sort)),
		directoryLoader:   browser.NewDirectoryLoader(),
		directoryCache:    browser.NewDirectoryCache(directoryCacheTTL, directoryCacheMaxEntries),
		config:            config,
		state:             state,
		stateManager:      runtime.stateManager,
		initialWindowSize: fyne.NewSize(float32(config.Window.Width), float32(config.Window.Height)),
		windowActive:      true,
		customTheme:       customTheme,
		keyManager:        keyManager,
		searchMatchers:    runtime.searchMatchers,
		runtime:           runtime,
	}
	keyManager.SetInputActivityCallback(fm.noteInputActivity)

	// Busy overlay and its input guard are window-owned.
	fm.busy = ui.NewBusyController(fm.window, keyManager, customTheme, 150*time.Millisecond, debugPrint)

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
	if runtime.configScript != nil {
		scriptCommands = runtime.configScript.Commands
	}
	mainDependencies := newMainScreenDependencies(fm)
	mainHandler := keymanager.NewMainScreenKeyHandlerWithCommands(mainDependencies, debugPrint, config.UI.KeyBindings, scriptCommands)
	mainHandler.SetTransitionGate(fm.keyManager.BeginOwnerTransition)
	mainHandler.SetCommandGate(fm.mainScreenCommandAllowed)
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

	// Register this window with the application runtime.
	registerFileManagerWindow(fm)

	// Set window close handler
	fm.window.SetCloseIntercept(func() {
		fm.QuitApplication()
	})

	return fm
}

func newMainScreenDependencies(fm *FileManager) keymanager.MainScreenDependencies {
	return keymanager.NewMainScreenDependencies(
		keymanager.MainScreenPorts{
			CursorList:  fm,
			Selection:   fm,
			Directory:   fm,
			FileOpener:  fm,
			Windows:     fm,
			History:     fm,
			Filters:     fm,
			Application: fm,
			Commands:    fm,
		},
		fm.RunExternalCommand,
		fm.SetClipboardText,
	)
}
