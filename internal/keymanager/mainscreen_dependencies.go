package keymanager

import (
	"reflect"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

// MainScreenCursorList is the list/cursor view used by keyboard navigation.
type MainScreenCursorList interface {
	GetCurrentCursorIndex() int
	SetCursorByIndex(index int)
	RefreshCursor()
	FileCount() int
	CurrentFile() (index int, file fileinfo.FileInfo, ok bool)
}

// MainScreenSelection provides transactional list-mark operations.
type MainScreenSelection interface {
	ToggleFileSelection(path string)
	SelectAllFiles() bool
	InvertFileSelection(includeDirectories bool) bool
	RefreshFileList()
}

// MainScreenDirectory provides directory navigation and cursor persistence.
type MainScreenDirectory interface {
	LoadDirectory(path string)
	GetCurrentPath() string
	SaveCursorPosition(dirPath string)
}

// MainScreenFileOpener handles the two main-list open modes.
type MainScreenFileOpener interface {
	OpenFile(file *fileinfo.FileInfo)
	OpenFileDefaultApp(file *fileinfo.FileInfo)
}

// MainScreenWindows contains cross-window commands.
type MainScreenWindows interface {
	OpenNewWindow()
	ReopenClosedWindow()
	FocusWindowLeft()
	FocusWindowRight()
	ResetWindowSize()
	ResetAllWindowSizes()
}

// MainScreenHistory contains navigation-history commands.
type MainScreenHistory interface {
	HistoryBack()
	PinCurrentHistoryPath()
}

// MainScreenFilters contains filter state commands.
type MainScreenFilters interface {
	ClearFilter()
	ToggleFilter()
}

// MainScreenApplication contains application lifecycle commands.
type MainScreenApplication interface {
	QuitApplication()
}

// CommandContextReader supplies the immutable context exposed to custom
// commands and Starlark command helpers.
type CommandContextReader interface {
	GetCurrentPath() string
	CommandTargets() []fileinfo.FileInfo
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

// MainScreenDependencies names the responsibilities used by the key handler.
// The handler constructor validates the required ports once before use.
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
	// External processes and the clipboard are optional host integrations.
	RunExternalCommand func(command string, args []string, edit bool, cwd string) bool
	SetClipboard       func(text string) bool
}

func (d MainScreenDependencies) validate() {
	check := func(name string, port any) {
		v := reflect.ValueOf(port)
		if v.IsValid() {
			switch v.Kind() {
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
				if !v.IsNil() {
					return
				}
			default:
				return
			}
		}
		panic("keymanager: missing main screen port " + name)
	}
	check("CursorList", d.CursorList)
	check("Selection", d.Selection)
	check("Directory", d.Directory)
	check("FileOpener", d.FileOpener)
	check("Windows", d.Windows)
	check("History", d.History)
	check("Filters", d.Filters)
	check("Application", d.Application)
	check("Commands", d.Commands)
}
