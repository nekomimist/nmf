package keymanager

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

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
	CommandOpenDefaultApp      = "open.defaultApp"
	CommandSelectToggle        = "selection.toggle"
	CommandSelectAll           = "selection.markAll"
	CommandSelectInvert        = "selection.invert"
	CommandSelectInvertWithDir = "selection.invertWithDirectories"
	CommandParentDirectory     = "directory.parent"
	CommandRefresh             = "directory.refresh"
	CommandHome                = "directory.home"
	CommandDirectoryCreate     = "directory.create"
	CommandClipboardTextFile   = "clipboard.createTextFile"
	CommandWindowNew           = "window.new"
	CommandWindowReopen        = "window.reopen"
	CommandWindowFocusLeft     = "window.focusLeft"
	CommandWindowFocusRight    = "window.focusRight"
	CommandWindowResetSize     = "window.resetSize"
	CommandWindowResetAllSizes = "window.resetAllSizes"
	CommandTreeShow            = "tree.show"
	CommandHistoryShow         = "history.show"
	CommandHistoryPinCurrent   = "history.pinCurrent"
	CommandDirectoryJumpShow   = "directoryJump.show"
	CommandFilterShow          = "filter.show"
	CommandFilterClear         = "filter.clear"
	CommandFilterToggle        = "filter.toggle"
	CommandSearchShow          = "search.show"
	CommandSortShow            = "sort.show"
	CommandJobsShow            = "jobs.show"
	CommandPathEdit            = "path.edit"
	CommandQuit                = "app.quit"
	CommandCopyShow            = "copy.show"
	CommandMoveShow            = "move.show"
	CommandArchiveExtract      = "archive.extract"
	CommandCompareShow         = "compare.show"
	CommandRenameShow          = "rename.show"
	CommandDeleteTrash         = "delete.trash"
	CommandDeletePermanent     = "delete.permanent"
	CommandExplorerContextShow = "explorerContext.show"
	CommandExternalCommandMenu = "externalCommand.menu"
	CommandViewerShow          = "viewer.show"
	CommandMaintenanceShow     = "maintenance.show"
	CommandNoop                = "noop"
)

const maxNestedCommandDepth = 32

// FileManagerInterface defines the interface needed by MainScreenKeyHandler.
type FileManagerInterface interface {
	GetCurrentCursorIndex() int
	SetCursorByIndex(index int)
	RefreshCursor()

	LoadDirectory(path string)
	GetCurrentPath() string
	GetFiles() []fileinfo.FileInfo
	CurrentSort() config.SortConfig
	ApplyTemporarySort(sortConfig config.SortConfig)

	GetSelectedFiles() map[string]bool
	GetAllSelectedFiles() []fileinfo.FileInfo
	SetFileSelected(path string, selected bool)
	RefreshFileList()

	SaveCursorPosition(dirPath string)

	OpenNewWindow()
	ReopenClosedWindow()
	FocusWindowLeft()
	FocusWindowRight()
	ResetWindowSize()
	ResetAllWindowSizes()
	ShowDirectoryTreeDialog()
	ShowNavigationHistoryDialog()
	PinCurrentHistoryPath()
	ShowDirectoryJumpDialog()

	ShowFilterDialog()
	ClearFilter()
	ToggleFilter()

	ShowIncrementalSearchDialog()
	ShowSortDialog()
	ShowJobsDialog()
	ShowPathEditDialog()
	ShowCreateDirectoryDialog()
	CreateDirectory(name string) bool
	ShowClipboardTextFileDialog()
	CreateClipboardTextFile(name string) bool
	ShowMessageDialog(title string, message string)
	QuitApplication()

	OpenFile(file *fileinfo.FileInfo)
	OpenFileDefaultApp(file *fileinfo.FileInfo)
	ShowCopyDialog()
	ShowMoveDialog()
	ShowExtractArchiveDialog()
	ShowCompareDialog()
	ShowRenameDialog()
	ShowDeleteDialog(permanent bool)
	ShowExplorerContextMenu()
	ShowExternalCommandMenu()
	ShowFileViewer()
	ShowMaintenanceDialog()
	ShowCommandMenu(title string, items []CommandMenuItem)
}

type externalCommandRunner interface {
	RunExternalCommand(command string, args []string, edit bool, cwd string) bool
}

type clipboardWriter interface {
	SetClipboardText(text string) bool
}

// commandSpec couples a command implementation with its input attributes.
// transition marks commands that change the input owner (open a dialog or
// menu, move window focus, enter an input mode); they are executed through
// the KeyManager owner-transition gate. Declaring the attribute at the
// definition site keeps new commands from silently skipping the gate.
type commandSpec struct {
	fn         CommandFunc
	transition bool
}

// MainScreenKeyHandler handles keyboard events for the main file list screen.
type MainScreenKeyHandler struct {
	fileManager     FileManagerInterface
	debugPrint      func(format string, args ...interface{})
	commands        map[string]commandSpec
	bindings        []keyBinding
	runningCommands map[string]int
	runningDepth    int
	deferTransition func(label string, action func())
}

// NewMainScreenKeyHandler creates a new main screen key handler.
func NewMainScreenKeyHandler(fm FileManagerInterface, debugPrint func(format string, args ...interface{}), configuredBindings ...[]config.KeyBindingEntry) *MainScreenKeyHandler {
	var cfg []config.KeyBindingEntry
	if len(configuredBindings) > 0 {
		cfg = configuredBindings[0]
	}
	return NewMainScreenKeyHandlerWithCommands(fm, debugPrint, cfg, nil)
}

// NewMainScreenKeyHandlerWithCommands creates a handler with additional commands.
func NewMainScreenKeyHandlerWithCommands(fm FileManagerInterface, debugPrint func(format string, args ...interface{}), configuredBindings []config.KeyBindingEntry, extraCommands CommandRegistry) *MainScreenKeyHandler {
	mh := &MainScreenKeyHandler{
		fileManager:     fm,
		debugPrint:      debugPrint,
		runningCommands: make(map[string]int),
	}
	mh.commands = mh.defaultCommands()
	for id, command := range extraCommands {
		if _, exists := mh.commands[id]; exists {
			mh.debugPrint("MainScreen: WARNING extra command ignored existing command=%s", id)
			continue
		}
		// Extra (user/script) commands run without the transition gate, same
		// as before; a command that opens UI itself must gate internally.
		mh.commands[id] = commandSpec{fn: command}
	}
	mh.bindings = mh.buildBindings(configuredBindings)
	return mh
}

func (mh *MainScreenKeyHandler) GetName() string { return "MainScreen" }

// SetTransitionGate configures delayed execution for commands that change input owner.
func (mh *MainScreenKeyHandler) SetTransitionGate(deferTransition func(label string, action func())) {
	mh.deferTransition = deferTransition
}

// ActivationShortcuts returns the shortcuts the window canvas must register so
// Ctrl/Alt bindings keep working in the no-focus fallback state, where the
// driver routes shortcuts to the canvas instead of generating TypedKey events.
// Fyne's folded standard shortcuts are listed explicitly because the driver
// never reports those combinations as CustomShortcut.
func (mh *MainScreenKeyHandler) ActivationShortcuts() []fyne.Shortcut {
	shortcuts := []fyne.Shortcut{
		&fyne.ShortcutCopy{},
		&fyne.ShortcutCut{},
		&fyne.ShortcutPaste{},
		&fyne.ShortcutSelectAll{},
		&fyne.ShortcutUndo{},
		&fyne.ShortcutRedo{},
	}
	seen := make(map[string]struct{})
	for _, binding := range mh.bindings {
		mod := binding.spec.mod
		if !mod.CtrlPressed && !mod.AltPressed {
			continue
		}
		var modifier fyne.KeyModifier
		if mod.ShiftPressed {
			modifier |= fyne.KeyModifierShift
		}
		if mod.CtrlPressed {
			modifier |= fyne.KeyModifierControl
		}
		if mod.AltPressed {
			modifier |= fyne.KeyModifierAlt
		}
		shortcut := &desktop.CustomShortcut{KeyName: binding.spec.key, Modifier: modifier}
		if _, ok := seen[shortcut.ShortcutName()]; ok {
			continue
		}
		seen[shortcut.ShortcutName()] = struct{}{}
		shortcuts = append(shortcuts, shortcut)
	}
	return shortcuts
}

func (mh *MainScreenKeyHandler) OnKeyActivated(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	if mh.executeBinding(ev, modifiers) {
		return true
	}
	return ev != nil && ev.Name == fyne.KeyTab
}

func (mh *MainScreenKeyHandler) OnTypedRune(r rune, modifiers ModifierState) bool {
	return false
}

func (mh *MainScreenKeyHandler) executeBinding(ev *fyne.KeyEvent, modifiers ModifierState) bool {
	for _, binding := range mh.bindings {
		if !binding.matches(ev, modifiers) {
			continue
		}
		key := fyne.KeyName("")
		if ev != nil {
			key = ev.Name
		}
		ctx := CommandContext{
			Modifiers:       modifiers,
			Key:             key,
			Event:           keyEventTyped,
			FileManager:     mh.fileManager,
			DeferTransition: mh.deferTransition,
		}
		ctx.RunCommand = func(command string) bool {
			return mh.executeCommand(command, ctx)
		}
		if runner, ok := mh.fileManager.(externalCommandRunner); ok {
			ctx.RunExternalCommand = runner.RunExternalCommand
		}
		if writer, ok := mh.fileManager.(clipboardWriter); ok {
			ctx.SetClipboard = writer.SetClipboardText
		}
		mh.executeCommand(binding.command, ctx)
		return true
	}
	return false
}

func (mh *MainScreenKeyHandler) executeCommand(commandID string, ctx CommandContext) bool {
	command, ok := mh.commands[commandID]
	if !ok {
		mh.debugPrint("MainScreen: unknown command=%s key=%s", commandID, ctx.Key)
		return false
	}
	if mh.runningDepth >= maxNestedCommandDepth {
		mh.debugPrint("MainScreen: command depth exceeded command=%s", commandID)
		return false
	}
	if mh.runningCommands[commandID] > 0 {
		mh.debugPrint("MainScreen: recursive command skipped command=%s", commandID)
		return false
	}

	run := func() {
		mh.debugPrint("MainScreen: command=%s key=%s event=%s", commandID, ctx.Key, ctx.Event)
		mh.runningDepth++
		mh.runningCommands[commandID]++
		defer func() {
			mh.runningDepth--
			mh.runningCommands[commandID]--
			if mh.runningCommands[commandID] == 0 {
				delete(mh.runningCommands, commandID)
			}
		}()

		command.fn(ctx)
	}

	if command.transition && mh.deferTransition != nil {
		mh.deferTransition(commandID, run)
		return true
	}

	run()
	return true
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
		if entry.Event != "" {
			mh.debugPrint("MainScreen: WARNING key binding event=%q is deprecated and ignored key=%q command=%s", entry.Event, entry.Key, entry.Command)
		}
		if _, ok := mh.commands[entry.Command]; !ok {
			mh.debugPrint("MainScreen: WARNING invalid key binding unknown command=%s key=%q", entry.Command, entry.Key)
			continue
		}
		bindings = append(bindings, keyBinding{
			spec:    spec,
			command: entry.Command,
		})
	}

	return bindings
}

func defaultMainScreenBindings() []config.KeyBindingEntry {
	return []config.KeyBindingEntry{
		{Key: "Up", Command: CommandCursorUp},
		{Key: "S-Up", Command: CommandCursorPageUp},
		{Key: "Down", Command: CommandCursorDown},
		{Key: "S-Down", Command: CommandCursorPageDown},
		{Key: "Return", Command: CommandOpen},
		{Key: "S-Return", Command: CommandOpenDefaultApp},
		{Key: "Space", Command: CommandSelectToggle},
		{Key: "C-A", Command: CommandSelectAll},
		{Key: "I", Command: CommandSelectInvert},
		{Key: "S-I", Command: CommandSelectInvertWithDir},
		{Key: "Backspace", Command: CommandParentDirectory},
		{Key: "S-Comma", Command: CommandCursorFirst},
		{Key: "Period", Command: CommandRefresh},
		{Key: "S-Period", Command: CommandCursorLast},
		{Key: "S-Backtick", Command: CommandHome},
		{Key: "K", Command: CommandDirectoryCreate},
		{Key: "P", Command: CommandClipboardTextFile},
		{Key: "F2", Command: CommandRenameShow},
		{Key: "R", Command: CommandRenameShow},
		{Key: "Left", Command: CommandWindowFocusLeft},
		{Key: "Right", Command: CommandWindowFocusRight},
		{Key: "S-Q", Command: CommandWindowResetSize},
		{Key: "C-S-Q", Command: CommandWindowResetAllSizes},
		{Key: "Tab", Command: CommandExplorerContextShow},
		{Key: "F3", Command: CommandFilterToggle},
		{Key: "Q", Command: CommandQuit},
		{Key: "C", Command: CommandCopyShow},
		{Key: "U", Command: CommandArchiveExtract},
		{Key: "S-C", Command: CommandCompareShow},
		{Key: "M", Command: CommandMoveShow},
		{Key: "X", Command: CommandExternalCommandMenu},
		{Key: "V", Command: CommandViewerShow},
		{Key: "C-N", Command: CommandWindowNew},
		{Key: "C-T", Command: CommandTreeShow},
		{Key: "C-H", Command: CommandHistoryShow},
		{Key: "S-B", Command: CommandHistoryPinCurrent},
		{Key: "C-F", Command: CommandFilterShow},
		{Key: "C-S", Command: CommandSearchShow},
		{Key: "S-S", Command: CommandSortShow},
		{Key: "C-L", Command: CommandPathEdit},
		{Key: "S-J", Command: CommandJobsShow},
		{Key: "J", Command: CommandDirectoryJumpShow},
		{Key: "Delete", Command: CommandDeleteTrash},
		{Key: "S-Delete", Command: CommandDeletePermanent},
	}
}

func (mh *MainScreenKeyHandler) defaultCommands() map[string]commandSpec {
	return map[string]commandSpec{
		CommandCursorUp:            {fn: mh.cursorUp},
		CommandCursorDown:          {fn: mh.cursorDown},
		CommandCursorPageUp:        {fn: mh.cursorPageUp},
		CommandCursorPageDown:      {fn: mh.cursorPageDown},
		CommandCursorFirst:         {fn: mh.cursorFirst},
		CommandCursorLast:          {fn: mh.cursorLast},
		CommandOpen:                {fn: mh.openCurrent},
		CommandOpenDefaultApp:      {fn: mh.openCurrentDefaultApp},
		CommandSelectToggle:        {fn: mh.toggleSelection},
		CommandSelectAll:           {fn: mh.selectAll},
		CommandSelectInvert:        {fn: func(CommandContext) { mh.invertSelection(false) }},
		CommandSelectInvertWithDir: {fn: func(CommandContext) { mh.invertSelection(true) }},
		CommandParentDirectory:     {fn: mh.parentDirectory},
		CommandRefresh:             {fn: mh.refreshDirectory},
		CommandHome:                {fn: mh.homeDirectory},
		CommandWindowNew:           {fn: func(CommandContext) { mh.fileManager.OpenNewWindow() }, transition: true},
		CommandWindowReopen:        {fn: func(CommandContext) { mh.fileManager.ReopenClosedWindow() }, transition: true},
		CommandWindowFocusLeft:     {fn: func(CommandContext) { mh.fileManager.FocusWindowLeft() }, transition: true},
		CommandWindowFocusRight:    {fn: func(CommandContext) { mh.fileManager.FocusWindowRight() }, transition: true},
		CommandWindowResetSize:     {fn: func(CommandContext) { mh.fileManager.ResetWindowSize() }},
		CommandWindowResetAllSizes: {fn: func(CommandContext) { mh.fileManager.ResetAllWindowSizes() }},
		CommandTreeShow:            {fn: func(CommandContext) { mh.fileManager.ShowDirectoryTreeDialog() }, transition: true},
		CommandHistoryShow:         {fn: func(CommandContext) { mh.fileManager.ShowNavigationHistoryDialog() }, transition: true},
		CommandHistoryPinCurrent:   {fn: func(CommandContext) { mh.fileManager.PinCurrentHistoryPath() }, transition: true},
		CommandDirectoryJumpShow:   {fn: func(CommandContext) { mh.fileManager.ShowDirectoryJumpDialog() }, transition: true},
		CommandFilterShow:          {fn: func(CommandContext) { mh.fileManager.ShowFilterDialog() }, transition: true},
		CommandFilterClear:         {fn: func(CommandContext) { mh.fileManager.ClearFilter() }},
		CommandFilterToggle:        {fn: func(CommandContext) { mh.fileManager.ToggleFilter() }},
		CommandSearchShow:          {fn: func(CommandContext) { mh.fileManager.ShowIncrementalSearchDialog() }, transition: true},
		CommandSortShow:            {fn: func(CommandContext) { mh.fileManager.ShowSortDialog() }, transition: true},
		CommandJobsShow:            {fn: func(CommandContext) { mh.fileManager.ShowJobsDialog() }, transition: true},
		CommandPathEdit:            {fn: func(CommandContext) { mh.fileManager.ShowPathEditDialog() }, transition: true},
		CommandDirectoryCreate:     {fn: func(CommandContext) { mh.fileManager.ShowCreateDirectoryDialog() }, transition: true},
		CommandClipboardTextFile:   {fn: func(CommandContext) { mh.fileManager.ShowClipboardTextFileDialog() }, transition: true},
		CommandQuit:                {fn: func(CommandContext) { mh.fileManager.QuitApplication() }, transition: true},
		CommandCopyShow:            {fn: func(CommandContext) { mh.fileManager.ShowCopyDialog() }, transition: true},
		CommandMoveShow:            {fn: func(CommandContext) { mh.fileManager.ShowMoveDialog() }, transition: true},
		// transition was missing from the old shouldDeferCommand switch; the
		// extract dialog is an input-owner change like copy/move.
		CommandArchiveExtract:      {fn: func(CommandContext) { mh.fileManager.ShowExtractArchiveDialog() }, transition: true},
		CommandCompareShow:         {fn: func(CommandContext) { mh.fileManager.ShowCompareDialog() }, transition: true},
		CommandRenameShow:          {fn: mh.rename, transition: true},
		CommandDeleteTrash:         {fn: func(CommandContext) { mh.fileManager.ShowDeleteDialog(false) }, transition: true},
		CommandDeletePermanent:     {fn: func(CommandContext) { mh.fileManager.ShowDeleteDialog(true) }, transition: true},
		CommandExplorerContextShow: {fn: func(CommandContext) { mh.fileManager.ShowExplorerContextMenu() }, transition: true},
		CommandExternalCommandMenu: {fn: func(CommandContext) { mh.fileManager.ShowExternalCommandMenu() }, transition: true},
		CommandViewerShow:          {fn: func(CommandContext) { mh.fileManager.ShowFileViewer() }, transition: true},
		CommandMaintenanceShow:     {fn: func(CommandContext) { mh.fileManager.ShowMaintenanceDialog() }, transition: true},
		CommandNoop:                {fn: func(CommandContext) {}},
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

func (mh *MainScreenKeyHandler) openCurrentDefaultApp(CommandContext) {
	currentIdx := mh.fileManager.GetCurrentCursorIndex()
	files := mh.fileManager.GetFiles()
	if currentIdx >= 0 && currentIdx < len(files) {
		fileInfo := files[currentIdx]
		mh.fileManager.OpenFileDefaultApp(&fileInfo)
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

func (mh *MainScreenKeyHandler) selectAll(CommandContext) {
	selected := 0
	for _, fileInfo := range mh.fileManager.GetFiles() {
		if fileInfo.Name == ".." || fileInfo.Status == fileinfo.StatusDeleted {
			continue
		}
		mh.fileManager.SetFileSelected(fileInfo.Path, true)
		selected++
	}
	if selected > 0 {
		mh.fileManager.RefreshFileList()
	}
}

func (mh *MainScreenKeyHandler) invertSelection(includeDirectories bool) {
	changed := false
	selectedFiles := mh.fileManager.GetSelectedFiles()
	for _, fileInfo := range mh.fileManager.GetFiles() {
		if fileInfo.Name == ".." || fileInfo.Status == fileinfo.StatusDeleted {
			continue
		}
		if fileInfo.IsDir && !includeDirectories {
			if selectedFiles[fileInfo.Path] {
				mh.fileManager.SetFileSelected(fileInfo.Path, false)
				changed = true
			}
			continue
		}
		mh.fileManager.SetFileSelected(fileInfo.Path, !selectedFiles[fileInfo.Path])
		changed = true
	}
	if changed {
		mh.fileManager.RefreshFileList()
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
