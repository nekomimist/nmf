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

// MainScreenPorts names every required responsibility supplied by the
// composition root. NewMainScreenDependencies validates the complete set of
// ports; optional command integrations are separate constructor arguments.
type MainScreenPorts struct {
	CursorList  MainScreenCursorList
	Selection   MainScreenSelection
	Directory   MainScreenDirectory
	FileOpener  MainScreenFileOpener
	Windows     MainScreenWindows
	History     MainScreenHistory
	Filters     MainScreenFilters
	Application MainScreenApplication
	Commands    CommandFileManager
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
	cursorList  MainScreenCursorList
	selection   MainScreenSelection
	directory   MainScreenDirectory
	fileOpener  MainScreenFileOpener
	windows     MainScreenWindows
	history     MainScreenHistory
	filters     MainScreenFilters
	application MainScreenApplication
	commands    CommandFileManager

	// These command integrations are optional. A nil function means the host
	// does not provide that operation, and the configscript command returns
	// false instead of invoking it.
	runExternalCommand func(command string, args []string, edit bool, cwd string) bool
	setClipboard       func(text string) bool
}

// NewMainScreenDependencies constructs a complete dependency set. The opaque
// result prevents callers outside keymanager from omitting a required port;
// handler constructors validate the zero value as a second line of defense.
// External-command and clipboard integrations remain optional so hosts that do
// not support them can fail those configscript operations closed.
func NewMainScreenDependencies(
	ports MainScreenPorts,
	runExternalCommand func(command string, args []string, edit bool, cwd string) bool,
	setClipboard func(text string) bool,
) MainScreenDependencies {
	dependencies := MainScreenDependencies{
		cursorList:         ports.CursorList,
		selection:          ports.Selection,
		directory:          ports.Directory,
		fileOpener:         ports.FileOpener,
		windows:            ports.Windows,
		history:            ports.History,
		filters:            ports.Filters,
		application:        ports.Application,
		commands:           ports.Commands,
		runExternalCommand: runExternalCommand,
		setClipboard:       setClipboard,
	}
	dependencies.validate()
	return dependencies
}

func (d MainScreenDependencies) validate() {
	requireMainScreenDependency("CursorList", d.cursorList)
	requireMainScreenDependency("Selection", d.selection)
	requireMainScreenDependency("Directory", d.directory)
	requireMainScreenDependency("FileOpener", d.fileOpener)
	requireMainScreenDependency("Windows", d.windows)
	requireMainScreenDependency("History", d.history)
	requireMainScreenDependency("Filters", d.filters)
	requireMainScreenDependency("Application", d.application)
	requireMainScreenDependency("Commands", d.commands)
}

func requireMainScreenDependency(name string, dependency any) {
	if dependency == nil || isNilMainScreenDependency(dependency) {
		panic("keymanager: MainScreenDependencies requires " + name)
	}
}

func isNilMainScreenDependency(dependency any) bool {
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
