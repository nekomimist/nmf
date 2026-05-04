package keymanager

import (
	"os"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

const (
	CommandCursorUp            = "cursor.up"
	CommandCursorDown          = "cursor.down"
	CommandCursorPageUp        = "cursor.pageUp"
	CommandCursorPageDown      = "cursor.pageDown"
	CommandCursorFirst         = "cursor.first"
	CommandCursorLast          = "cursor.last"
	CommandOpen                = "open"
	CommandSelectToggle        = "selection.toggle"
	CommandParentDirectory     = "directory.parent"
	CommandRefresh             = "directory.refresh"
	CommandHome                = "directory.home"
	CommandWindowNew           = "window.new"
	CommandTreeShow            = "tree.show"
	CommandHistoryShow         = "history.show"
	CommandDirectoryJumpShow   = "directoryJump.show"
	CommandFilterShow          = "filter.show"
	CommandFilterClear         = "filter.clear"
	CommandFilterToggle        = "filter.toggle"
	CommandSearchShow          = "search.show"
	CommandSortShow            = "sort.show"
	CommandJobsShow            = "jobs.show"
	CommandPathFocus           = "path.focus"
	CommandQuit                = "app.quit"
	CommandCopyShow            = "copy.show"
	CommandMoveShow            = "move.show"
	CommandRenameShow          = "rename.show"
	CommandDeleteTrash         = "delete.trash"
	CommandDeletePermanent     = "delete.permanent"
	CommandExplorerContextShow = "explorerContext.show"
	CommandExternalCommandMenu = "externalCommand.menu"
)

// FileManagerInterface defines the interface needed by MainScreenKeyHandler.
type FileManagerInterface interface {
	GetCurrentCursorIndex() int
	SetCursorByIndex(index int)
	RefreshCursor()

	LoadDirectory(path string)
	GetCurrentPath() string
	GetFiles() []fileinfo.FileInfo

	GetSelectedFiles() map[string]bool
	SetFileSelected(path string, selected bool)
	RefreshFileList()

	SaveCursorPosition(dirPath string)

	OpenNewWindow()
	ShowDirectoryTreeDialog()
	ShowNavigationHistoryDialog()
	ShowDirectoryJumpDialog()

	ShowFilterDialog()
	ClearFilter()
	ToggleFilter()

	ShowIncrementalSearchDialog()
	ShowSortDialog()
	ShowJobsDialog()
	FocusPathEntry()
	QuitApplication()

	OpenFile(file *fileinfo.FileInfo)
	ShowCopyDialog()
	ShowMoveDialog()
	ShowRenameDialog()
	ShowDeleteDialog(permanent bool)
	ShowExplorerContextMenu()
	ShowExternalCommandMenu()
}

// MainScreenKeyHandler handles keyboard events for the main file list screen.
type MainScreenKeyHandler struct {
	fileManager FileManagerInterface
	debugPrint  func(format string, args ...interface{})
	commands    CommandRegistry
	bindings    []keyBinding
}

// NewMainScreenKeyHandler creates a new main screen key handler.
func NewMainScreenKeyHandler(fm FileManagerInterface, debugPrint func(format string, args ...interface{}), configuredBindings ...[]config.KeyBindingEntry) *MainScreenKeyHandler {
	mh := &MainScreenKeyHandler{
		fileManager: fm,
		debugPrint:  debugPrint,
	}
	mh.commands = mh.defaultCommands()

	var cfg []config.KeyBindingEntry
	if len(configuredBindings) > 0 {
		cfg = configuredBindings[0]
	}
	mh.bindings = mh.buildBindings(cfg)
	return mh
}

func (mh *MainScreenKeyHandler) GetName() string { return "MainScreen" }

func (mh *MainScreenKeyHandler) OnKeyDown(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return mh.executeBinding(keyEventDown, ev, modifiers)
}

func (mh *MainScreenKeyHandler) OnKeyUp(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	return mh.executeBinding(keyEventUp, ev, modifiers)
}

func (mh *MainScreenKeyHandler) OnTypedKey(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if mh.executeBinding(keyEventTyped, ev, modifiers) {
		return true
	}
	return ev != nil && ev.Name == fyne.KeyTab
}

func (mh *MainScreenKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	return false
}

func (mh *MainScreenKeyHandler) executeBinding(event string, ev *fyne.KeyEvent, modifiers ModifierState) bool {
	for _, binding := range mh.bindings {
		if binding.event != event || !binding.matches(ev, modifiers) {
			continue
		}
		command, ok := mh.commands[binding.command]
		if !ok {
			mh.debugPrint("MainScreen: unknown command=%s key=%s", binding.command, ev.Name)
			return true
		}
		mh.debugPrint("MainScreen: command=%s key=%s event=%s", binding.command, ev.Name, event)
		command(CommandContext{Modifiers: modifiers})
		return true
	}
	return false
}

func (mh *MainScreenKeyHandler) buildBindings(configured []config.KeyBindingEntry) []keyBinding {
	entries := append(configured, defaultMainScreenBindings()...)
	bindings := make([]keyBinding, 0, len(entries))

	for _, entry := range entries {
		spec, err := parseKeySpec(entry.Key)
		if err != nil {
			mh.debugPrint("MainScreen: WARNING invalid key binding key=%q command=%s err=%v", entry.Key, entry.Command, err)
			continue
		}
		event := normalizeEventName(entry.Event, spec)
		if event == "" {
			mh.debugPrint("MainScreen: WARNING invalid key binding event=%q key=%q command=%s", entry.Event, entry.Key, entry.Command)
			continue
		}
		if _, ok := mh.commands[entry.Command]; !ok {
			mh.debugPrint("MainScreen: WARNING invalid key binding unknown command=%s key=%q", entry.Command, entry.Key)
			continue
		}
		bindings = append(bindings, keyBinding{
			spec:    spec,
			event:   event,
			command: entry.Command,
		})
	}

	return bindings
}

func defaultMainScreenBindings() []config.KeyBindingEntry {
	return []config.KeyBindingEntry{
		{Key: "Up", Command: CommandCursorUp, Event: keyEventTyped},
		{Key: "S-Up", Command: CommandCursorPageUp, Event: keyEventTyped},
		{Key: "Down", Command: CommandCursorDown, Event: keyEventTyped},
		{Key: "S-Down", Command: CommandCursorPageDown, Event: keyEventTyped},
		{Key: "Return", Command: CommandOpen, Event: keyEventTyped},
		{Key: "Space", Command: CommandSelectToggle, Event: keyEventTyped},
		{Key: "Backspace", Command: CommandParentDirectory, Event: keyEventTyped},
		{Key: "S-Comma", Command: CommandCursorFirst, Event: keyEventTyped},
		{Key: "Period", Command: CommandRefresh, Event: keyEventTyped},
		{Key: "S-Period", Command: CommandCursorLast, Event: keyEventTyped},
		{Key: "S-Backtick", Command: CommandHome, Event: keyEventTyped},
		{Key: "F2", Command: CommandRenameShow, Event: keyEventTyped},
		{Key: "R", Command: CommandRenameShow, Event: keyEventUp},
		{Key: "Tab", Command: CommandExplorerContextShow, Event: keyEventTyped},
		{Key: "F3", Command: CommandFilterToggle, Event: keyEventTyped},
		{Key: "Q", Command: CommandQuit, Event: keyEventTyped},
		{Key: "C", Command: CommandCopyShow, Event: keyEventTyped},
		{Key: "M", Command: CommandMoveShow, Event: keyEventTyped},
		{Key: "X", Command: CommandExternalCommandMenu, Event: keyEventTyped},
		{Key: "C-N", Command: CommandWindowNew, Event: keyEventDown},
		{Key: "C-T", Command: CommandTreeShow, Event: keyEventDown},
		{Key: "C-H", Command: CommandHistoryShow, Event: keyEventDown},
		{Key: "C-F", Command: CommandFilterShow, Event: keyEventDown},
		{Key: "C-S", Command: CommandSearchShow, Event: keyEventDown},
		{Key: "S-S", Command: CommandSortShow, Event: keyEventTyped},
		{Key: "C-L", Command: CommandPathFocus, Event: keyEventDown},
		{Key: "S-J", Command: CommandJobsShow, Event: keyEventTyped},
		{Key: "J", Command: CommandDirectoryJumpShow, Event: keyEventTyped},
		{Key: "Delete", Command: CommandDeleteTrash, Event: keyEventTyped},
		{Key: "S-Delete", Command: CommandDeletePermanent, Event: keyEventDown},
	}
}

func (mh *MainScreenKeyHandler) defaultCommands() CommandRegistry {
	return CommandRegistry{
		CommandCursorUp:            mh.cursorUp,
		CommandCursorDown:          mh.cursorDown,
		CommandCursorPageUp:        mh.cursorPageUp,
		CommandCursorPageDown:      mh.cursorPageDown,
		CommandCursorFirst:         mh.cursorFirst,
		CommandCursorLast:          mh.cursorLast,
		CommandOpen:                mh.openCurrent,
		CommandSelectToggle:        mh.toggleSelection,
		CommandParentDirectory:     mh.parentDirectory,
		CommandRefresh:             mh.refreshDirectory,
		CommandHome:                mh.homeDirectory,
		CommandWindowNew:           func(CommandContext) { mh.fileManager.OpenNewWindow() },
		CommandTreeShow:            func(CommandContext) { mh.fileManager.ShowDirectoryTreeDialog() },
		CommandHistoryShow:         func(CommandContext) { mh.fileManager.ShowNavigationHistoryDialog() },
		CommandDirectoryJumpShow:   func(CommandContext) { mh.fileManager.ShowDirectoryJumpDialog() },
		CommandFilterShow:          func(CommandContext) { mh.fileManager.ShowFilterDialog() },
		CommandFilterClear:         func(CommandContext) { mh.fileManager.ClearFilter() },
		CommandFilterToggle:        func(CommandContext) { mh.fileManager.ToggleFilter() },
		CommandSearchShow:          func(CommandContext) { mh.fileManager.ShowIncrementalSearchDialog() },
		CommandSortShow:            func(CommandContext) { mh.fileManager.ShowSortDialog() },
		CommandJobsShow:            func(CommandContext) { mh.fileManager.ShowJobsDialog() },
		CommandPathFocus:           func(CommandContext) { mh.fileManager.FocusPathEntry() },
		CommandQuit:                func(CommandContext) { mh.fileManager.QuitApplication() },
		CommandCopyShow:            func(CommandContext) { mh.fileManager.ShowCopyDialog() },
		CommandMoveShow:            func(CommandContext) { mh.fileManager.ShowMoveDialog() },
		CommandRenameShow:          mh.rename,
		CommandDeleteTrash:         func(CommandContext) { mh.fileManager.ShowDeleteDialog(false) },
		CommandDeletePermanent:     func(CommandContext) { mh.fileManager.ShowDeleteDialog(true) },
		CommandExplorerContextShow: func(CommandContext) { mh.fileManager.ShowExplorerContextMenu() },
		CommandExternalCommandMenu: func(CommandContext) { mh.fileManager.ShowExternalCommandMenu() },
	}
}

func (mh *MainScreenKeyHandler) cursorUp(CommandContext) {
	currentIdx := mh.fileManager.GetCurrentCursorIndex()
	if currentIdx > 0 {
		mh.fileManager.SetCursorByIndex(currentIdx - 1)
		mh.fileManager.RefreshCursor()
	}
}

func (mh *MainScreenKeyHandler) cursorDown(CommandContext) {
	currentIdx := mh.fileManager.GetCurrentCursorIndex()
	files := mh.fileManager.GetFiles()
	if currentIdx < len(files)-1 {
		mh.fileManager.SetCursorByIndex(currentIdx + 1)
		mh.fileManager.RefreshCursor()
	}
}

func (mh *MainScreenKeyHandler) cursorPageUp(CommandContext) {
	currentIdx := mh.fileManager.GetCurrentCursorIndex()
	files := mh.fileManager.GetFiles()
	if len(files) == 0 {
		return
	}
	newIdx := currentIdx - 20
	if newIdx < 0 {
		newIdx = 0
	}
	mh.fileManager.SetCursorByIndex(newIdx)
	mh.fileManager.RefreshCursor()
}

func (mh *MainScreenKeyHandler) cursorPageDown(CommandContext) {
	currentIdx := mh.fileManager.GetCurrentCursorIndex()
	files := mh.fileManager.GetFiles()
	if len(files) == 0 {
		return
	}
	newIdx := currentIdx + 20
	if newIdx >= len(files) {
		newIdx = len(files) - 1
	}
	mh.fileManager.SetCursorByIndex(newIdx)
	mh.fileManager.RefreshCursor()
}

func (mh *MainScreenKeyHandler) cursorFirst(CommandContext) {
	files := mh.fileManager.GetFiles()
	if len(files) > 0 {
		mh.fileManager.SetCursorByIndex(0)
		mh.fileManager.RefreshCursor()
	}
}

func (mh *MainScreenKeyHandler) cursorLast(CommandContext) {
	files := mh.fileManager.GetFiles()
	if len(files) > 0 {
		mh.fileManager.SetCursorByIndex(len(files) - 1)
		mh.fileManager.RefreshCursor()
	}
}

func (mh *MainScreenKeyHandler) openCurrent(CommandContext) {
	currentIdx := mh.fileManager.GetCurrentCursorIndex()
	files := mh.fileManager.GetFiles()
	if currentIdx >= 0 && currentIdx < len(files) {
		fileInfo := files[currentIdx]
		mh.fileManager.OpenFile(&fileInfo)
	}
}

func (mh *MainScreenKeyHandler) toggleSelection(CommandContext) {
	currentIdx := mh.fileManager.GetCurrentCursorIndex()
	files := mh.fileManager.GetFiles()
	if currentIdx < 0 || currentIdx >= len(files) {
		return
	}

	fileInfo := files[currentIdx]
	if fileInfo.Name == ".." || fileInfo.Status == fileinfo.StatusDeleted {
		return
	}

	selectedFiles := mh.fileManager.GetSelectedFiles()
	mh.fileManager.SetFileSelected(fileInfo.Path, !selectedFiles[fileInfo.Path])
	mh.fileManager.RefreshFileList()

	if currentIdx < len(files)-1 {
		mh.fileManager.SetCursorByIndex(currentIdx + 1)
		mh.fileManager.RefreshCursor()
	}
}

func (mh *MainScreenKeyHandler) parentDirectory(CommandContext) {
	parent := fileinfo.ParentPath(mh.fileManager.GetCurrentPath())
	if parent != mh.fileManager.GetCurrentPath() {
		mh.fileManager.LoadDirectory(parent)
	}
}

func (mh *MainScreenKeyHandler) refreshDirectory(CommandContext) {
	mh.fileManager.SaveCursorPosition(mh.fileManager.GetCurrentPath())
	mh.fileManager.LoadDirectory(mh.fileManager.GetCurrentPath())
}

func (mh *MainScreenKeyHandler) homeDirectory(CommandContext) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		mh.debugPrint("MainScreen: Failed to get home directory: %v", err)
		return
	}
	mh.fileManager.LoadDirectory(homeDir)
}

func (mh *MainScreenKeyHandler) rename(ctx CommandContext) {
	if !ctx.Modifiers.CtrlPressed && !ctx.Modifiers.ShiftPressed && !ctx.Modifiers.AltPressed {
		mh.fileManager.ShowRenameDialog()
	}
}
