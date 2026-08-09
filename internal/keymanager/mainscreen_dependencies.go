package keymanager

import (
	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

// MainScreenCursorList is the list/cursor view used by keyboard navigation.
type MainScreenCursorList interface {
	GetCurrentCursorIndex() int
	SetCursorByIndex(index int)
	RefreshCursor()
	GetFiles() []fileinfo.FileInfo
	FileCount() int
	FileAt(index int) (fileinfo.FileInfo, bool)
}

type MainScreenSelection interface {
	GetSelectedFiles() map[string]bool
	SetFileSelected(path string, selected bool)
	RefreshFileList()
}

type MainScreenDirectory interface {
	LoadDirectory(path string)
	GetCurrentPath() string
	SaveCursorPosition(dirPath string)
}

type MainScreenFileOpener interface {
	OpenFile(file *fileinfo.FileInfo)
	OpenFileDefaultApp(file *fileinfo.FileInfo)
}

type MainScreenWindows interface {
	OpenNewWindow()
	ReopenClosedWindow()
	FocusWindowLeft()
	FocusWindowRight()
	ResetWindowSize()
	ResetAllWindowSizes()
}

type MainScreenHistory interface {
	PinCurrentHistoryPath()
}

type MainScreenFilters interface {
	ClearFilter()
	ToggleFilter()
}

type MainScreenApplication interface {
	QuitApplication()
}

// CommandContextReader supplies the immutable context exposed to custom
// commands and Starlark command helpers.
type CommandContextReader interface {
	GetCurrentCursorIndex() int
	GetCurrentPath() string
	GetFiles() []fileinfo.FileInfo
	GetSelectedFiles() map[string]bool
	GetAllSelectedFiles() []fileinfo.FileInfo
	CurrentSort() config.SortConfig
}

// CommandOperations are the direct mutations available to custom commands.
type CommandOperations interface {
	LoadDirectory(path string)
	ApplyTemporarySort(sortConfig config.SortConfig)
	CreateDirectory(name string) bool
	CreateClipboardTextFile(name string) bool
}

// CommandFileManager is intentionally independent of the main-screen ports:
// configscript needs command context and mutations, not window/focus/filter UI.
type CommandFileManager interface {
	CommandContextReader
	CommandOperations
}

// MainScreenDependencies makes each responsibility consumed by the main key
// handler explicit. A composition root may provide one object for several
// ports, but tests and future controllers do not need to implement unrelated
// methods.
type MainScreenDependencies struct {
	CursorList  MainScreenCursorList
	Selection   MainScreenSelection
	Directory   MainScreenDirectory
	FileOpener  MainScreenFileOpener
	Windows     MainScreenWindows
	History     MainScreenHistory
	Filters     MainScreenFilters
	Application MainScreenApplication
	Commands    CommandFileManager

	RunExternalCommand func(command string, args []string, edit bool, cwd string) bool
	SetClipboard       func(text string) bool
}
