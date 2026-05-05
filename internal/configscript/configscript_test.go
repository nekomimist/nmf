package configscript

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

func TestLoadAppliesStarlarkConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.window(width = 1000, height = 720)
nmf.theme(dark = False, font_size = 16, font_name = "Noto Sans")
nmf.ui(show_hidden_files = True, item_spacing = 2)
nmf.sort(by = "extension", order = "desc", directories_first = False)
nmf.cursor_style(type = "border", thickness = 3)
nmf.cursor_memory(max_entries = 12)
nmf.navigation_history(max_entries = 9)
nmf.file_filter(max_entries = 7)
nmf.clear_directory_jumps()
nmf.directory_jump("p", "~/projects")
nmf.clear_keys()
nmf.key("C-P", "user.parent", event = "down")
nmf.clear_external_commands()
nmf.external_command(
    name = "Open in Vim",
    extensions = ["go", "md"],
    command = "vim",
    args = ["{file}"],
)
def parent(ctx):
    return None
nmf.command("user.parent", parent)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := testConfig()
	rt, err := Load(path, cfg, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if !rt.Loaded() {
		t.Fatal("runtime should report loaded init.star")
	}
	if cfg.Window.Width != 1000 || cfg.Window.Height != 720 {
		t.Fatalf("window = %+v, want 1000x720", cfg.Window)
	}
	if cfg.Theme.Dark || cfg.Theme.FontSize != 16 || cfg.Theme.FontName != "Noto Sans" {
		t.Fatalf("theme = %+v, want light 16 Noto Sans", cfg.Theme)
	}
	if !cfg.UI.ShowHiddenFiles || cfg.UI.ItemSpacing != 2 {
		t.Fatalf("ui = %+v, want hidden=true spacing=2", cfg.UI)
	}
	if cfg.UI.Sort.SortBy != "extension" || cfg.UI.Sort.SortOrder != "desc" || cfg.UI.Sort.DirectoriesFirst {
		t.Fatalf("sort = %+v, want extension desc dirs=false", cfg.UI.Sort)
	}
	if cfg.UI.CursorStyle.Type != "border" || cfg.UI.CursorStyle.Thickness != 3 {
		t.Fatalf("cursor style = %+v, want border thickness 3", cfg.UI.CursorStyle)
	}
	if cfg.UI.CursorMemory.MaxEntries != 12 || cfg.UI.NavigationHistory.MaxEntries != 9 || cfg.UI.FileFilter.MaxEntries != 7 {
		t.Fatalf("max entries not applied: cursor=%d history=%d filter=%d",
			cfg.UI.CursorMemory.MaxEntries,
			cfg.UI.NavigationHistory.MaxEntries,
			cfg.UI.FileFilter.MaxEntries,
		)
	}
	if len(cfg.UI.DirectoryJumps.Entries) != 1 || cfg.UI.DirectoryJumps.Entries[0].Directory != "~/projects" {
		t.Fatalf("directory jumps = %+v, want one projects entry", cfg.UI.DirectoryJumps.Entries)
	}
	if len(cfg.UI.KeyBindings) != 1 || cfg.UI.KeyBindings[0].Command != "user.parent" {
		t.Fatalf("key bindings = %+v, want user.parent", cfg.UI.KeyBindings)
	}
	if len(cfg.UI.ExternalCommands) != 1 || cfg.UI.ExternalCommands[0].Command != "vim" {
		t.Fatalf("external commands = %+v, want vim", cfg.UI.ExternalCommands)
	}
	if _, ok := rt.Commands["user.parent"]; !ok {
		t.Fatal("user.parent command was not registered")
	}
}

func TestSaveTransformStripsStarlarkOverlayAndPreservesRuntimeState(t *testing.T) {
	base := testConfig()
	base.Window.Width = 900
	base.UI.Sort.SortBy = "name"
	base.UI.CursorMemory.MaxEntries = 100
	base.UI.KeyBindings = []config.KeyBindingEntry{{Key: "X", Command: "jobs.show"}}

	rt := &Runtime{saveMask: saveMask{
		window:                   true,
		uiSort:                   true,
		uiCursorMemoryMaxEntries: true,
		uiKeyBindings:            true,
	}}
	current := config.Clone(base)
	current.Window.Width = 1200
	current.UI.Sort.SortBy = "size"
	current.UI.CursorMemory.MaxEntries = 5
	current.UI.CursorMemory.Entries["/tmp"] = "file.txt"
	current.UI.CursorMemory.LastUsed["/tmp"] = time.Unix(10, 0)
	current.UI.KeyBindings = append(current.UI.KeyBindings, config.KeyBindingEntry{
		Key:     "C-P",
		Command: "user.parent",
	})

	saved := rt.SaveTransform(base)(current)
	if saved.Window.Width != 900 {
		t.Fatalf("saved window width = %d, want base 900", saved.Window.Width)
	}
	if saved.UI.Sort.SortBy != "name" {
		t.Fatalf("saved sort = %s, want base name", saved.UI.Sort.SortBy)
	}
	if saved.UI.CursorMemory.MaxEntries != 100 {
		t.Fatalf("saved cursor max entries = %d, want base 100", saved.UI.CursorMemory.MaxEntries)
	}
	if saved.UI.CursorMemory.Entries["/tmp"] != "file.txt" {
		t.Fatalf("runtime cursor memory was not preserved: %+v", saved.UI.CursorMemory.Entries)
	}
	if len(saved.UI.KeyBindings) != 1 || saved.UI.KeyBindings[0].Command != "jobs.show" {
		t.Fatalf("saved key bindings = %+v, want base binding only", saved.UI.KeyBindings)
	}
}

func TestUnkeyBindsKeyToNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.unkey("S-S")
nmf.unkey("C-S", event = "down")
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := testConfig()
	if _, err := Load(path, cfg, func(string, ...interface{}) {}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []config.KeyBindingEntry{
		{Key: "S-S", Command: keymanager.CommandNoop},
		{Key: "C-S", Command: keymanager.CommandNoop, Event: "down"},
	}
	if !reflect.DeepEqual(cfg.UI.KeyBindings, want) {
		t.Fatalf("key bindings = %#v, want %#v", cfg.UI.KeyBindings, want)
	}
}

func TestCustomCommandCanRunInternalCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def parent(ctx):
    nmf.run("directory.parent")
nmf.command("user.parent", parent)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var ran string
	rt.Commands["user.parent"](keymanager.CommandContext{
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
	})
	if ran != "directory.parent" {
		t.Fatalf("ran command = %q, want directory.parent", ran)
	}
}

func TestCustomCommandCanExecExternalCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    if nmf.exec("vim", args = [ctx.current_file]):
        nmf.run("directory.refresh")
nmf.command("user.edit", edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var gotCommand string
	var gotArgs []string
	var ran string
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
		RunExternalCommand: func(command string, args []string) bool {
			gotCommand = command
			gotArgs = args
			return true
		},
		FileManager: &configScriptFakeFileManager{
			currentPath: "/tmp",
			cursorIndex: 0,
			files: []fileinfo.FileInfo{
				{Name: "main.go", Path: "/tmp/main.go"},
			},
		},
	})
	if gotCommand != "vim" {
		t.Fatalf("exec command = %q, want vim", gotCommand)
	}
	if !reflect.DeepEqual(gotArgs, []string{"/tmp/main.go"}) {
		t.Fatalf("exec args = %#v, want current file", gotArgs)
	}
	if ran != "directory.refresh" {
		t.Fatalf("ran command = %q, want directory.refresh", ran)
	}
}

func TestCustomCommandExecUsesRawArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    nmf.exec("vim", args = ["{file}"])
nmf.command("user.edit", edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var gotArgs []string
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunExternalCommand: func(command string, args []string) bool {
			gotArgs = args
			return true
		},
	})
	if !reflect.DeepEqual(gotArgs, []string{"{file}"}) {
		t.Fatalf("exec args = %#v, want raw {file}", gotArgs)
	}
}

func TestCommandContextUsesExternalCommandTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def inspect(ctx):
    nmf.exec("inspect", args = [ctx.current_path, ctx.current_file, ctx.current_name] + ctx.selected_files)
nmf.command("user.inspect", inspect)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var gotArgs []string
	rt.Commands["user.inspect"](keymanager.CommandContext{
		FileManager: &configScriptFakeFileManager{
			currentPath: "/tmp/work",
			cursorIndex: 0,
			files: []fileinfo.FileInfo{
				{Name: "cursor.go", Path: "/tmp/work/cursor.go"},
				{Name: "notes.md", Path: "/tmp/work/notes.md"},
				{Name: "..", Path: "/tmp"},
				{Name: "deleted.txt", Path: "/tmp/work/deleted.txt", Status: fileinfo.StatusDeleted},
			},
			selectedFiles: map[string]bool{
				"/tmp/work/notes.md":    true,
				"/tmp/work/deleted.txt": true,
			},
		},
		RunExternalCommand: func(command string, args []string) bool {
			gotArgs = args
			return true
		},
	})

	want := []string{"/tmp/work", "/tmp/work/notes.md", "notes.md", "/tmp/work/notes.md"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("exec args = %#v, want %#v", gotArgs, want)
	}
}

func TestCommandContextTargetsFallbackToCursor(t *testing.T) {
	fm := &configScriptFakeFileManager{
		currentPath: "/tmp/work",
		cursorIndex: 1,
		files: []fileinfo.FileInfo{
			{Name: "..", Path: "/tmp"},
			{Name: "cursor.go", Path: "/tmp/work/cursor.go"},
		},
	}

	dir, file, name, files := commandContextTargets(fm)
	if dir != "/tmp/work" || file != "/tmp/work/cursor.go" || name != "cursor.go" {
		t.Fatalf("targets = dir %q file %q name %q, want cursor values", dir, file, name)
	}
	if !reflect.DeepEqual(files, []string{"/tmp/work/cursor.go"}) {
		t.Fatalf("files = %#v, want cursor file", files)
	}
}

func TestCommandContextTargetsEmptyWhenNoTarget(t *testing.T) {
	fm := &configScriptFakeFileManager{
		currentPath: "/tmp/work",
		cursorIndex: 0,
		files: []fileinfo.FileInfo{
			{Name: "..", Path: "/tmp"},
		},
	}

	dir, file, name, files := commandContextTargets(fm)
	if dir != "/tmp/work" || file != "" || name != "" {
		t.Fatalf("targets = dir %q file %q name %q, want only dir", dir, file, name)
	}
	if len(files) != 0 {
		t.Fatalf("files = %#v, want empty", files)
	}
}

func TestCustomCommandExecReturnsFalseWithoutRunner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    if nmf.exec("vim"):
        nmf.run("directory.refresh")
nmf.command("user.edit", edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var ran string
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
	})
	if ran != "" {
		t.Fatalf("ran command = %q, want none", ran)
	}
}

func TestExecRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "empty command",
			src:  `nmf.exec(" ")`,
		},
		{
			name: "non string arg",
			src:  `nmf.exec("vim", args = [1])`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, FileName)
			if err := os.WriteFile(path, []byte(tt.src), 0644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			if _, err := Load(path, testConfig(), func(string, ...interface{}) {}); err == nil {
				t.Fatal("Load should reject invalid nmf.exec arguments")
			}
		})
	}
}

func TestGetenvReadsEnvironmentAndDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	t.Setenv("NMF_TEST_CONFIGSCRIPT_FONT", "Env Font")
	missingName := "NMF_TEST_CONFIGSCRIPT_MISSING"
	oldMissing, hadMissing := os.LookupEnv(missingName)
	if err := os.Unsetenv(missingName); err != nil {
		t.Fatalf("Unsetenv failed: %v", err)
	}
	t.Cleanup(func() {
		if hadMissing {
			_ = os.Setenv(missingName, oldMissing)
		} else {
			_ = os.Unsetenv(missingName)
		}
	})

	src := `
font = nmf.getenv("NMF_TEST_CONFIGSCRIPT_FONT")
fallback = nmf.getenv("NMF_TEST_CONFIGSCRIPT_MISSING", "fallback.ttf")
missing = nmf.getenv("NMF_TEST_CONFIGSCRIPT_MISSING")

nmf.theme(font_name = font, font_path = fallback)
if missing == None:
    nmf.ui(show_hidden_files = True)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cfg := testConfig()
	if _, err := Load(path, cfg, func(string, ...interface{}) {}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Theme.FontName != "Env Font" {
		t.Fatalf("font name = %q, want Env Font", cfg.Theme.FontName)
	}
	if cfg.Theme.FontPath != "fallback.ttf" {
		t.Fatalf("font path = %q, want fallback.ttf", cfg.Theme.FontPath)
	}
	if !cfg.UI.ShowHiddenFiles {
		t.Fatal("missing env without default should return None")
	}
}

func TestSystemInfoBuiltins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.theme(font_name = nmf.os(), font_path = nmf.hostname())
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cfg := testConfig()
	if _, err := Load(path, cfg, func(string, ...interface{}) {}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Theme.FontName != runtime.GOOS {
		t.Fatalf("os = %q, want %q", cfg.Theme.FontName, runtime.GOOS)
	}
	wantHostname, err := os.Hostname()
	if err == nil && cfg.Theme.FontPath != wantHostname {
		t.Fatalf("hostname = %q, want %q", cfg.Theme.FontPath, wantHostname)
	}
}

func TestLoadRejectsModuleOutsideConfigDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(`load("../other.star", "x")`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := Load(path, testConfig(), func(string, ...interface{}) {}); err == nil {
		t.Fatal("Load should reject modules outside config dir")
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Window: config.WindowConfig{Width: 800, Height: 600},
		Theme:  config.ThemeConfig{Dark: true, FontSize: 14},
		UI: config.UIConfig{
			Sort: config.SortConfig{
				SortBy:           "name",
				SortOrder:        "asc",
				DirectoriesFirst: true,
			},
			ItemSpacing: 4,
			CursorStyle: config.CursorStyleConfig{
				Type:      "underline",
				Thickness: 2,
			},
			CursorMemory: config.CursorMemoryConfig{
				MaxEntries: 100,
				Entries:    map[string]string{},
				LastUsed:   map[string]time.Time{},
			},
			NavigationHistory: config.NavigationHistoryConfig{
				MaxEntries: 50,
				Entries:    []string{},
				LastUsed:   map[string]time.Time{},
			},
			FileFilter: config.FileFilterConfig{
				MaxEntries: 30,
				Entries:    []config.FilterEntry{},
			},
			DirectoryJumps: config.DirectoryJumpsConfig{
				Entries: []config.DirectoryJumpEntry{{Shortcut: "d", Directory: "~/Downloads"}},
			},
			KeyBindings:      []config.KeyBindingEntry{},
			ExternalCommands: []config.ExternalCommandEntry{},
		},
	}
}

type configScriptFakeFileManager struct {
	currentPath   string
	cursorIndex   int
	files         []fileinfo.FileInfo
	selectedFiles map[string]bool
}

func (f *configScriptFakeFileManager) GetCurrentCursorIndex() int        { return f.cursorIndex }
func (f *configScriptFakeFileManager) SetCursorByIndex(index int)        { f.cursorIndex = index }
func (f *configScriptFakeFileManager) RefreshCursor()                    {}
func (f *configScriptFakeFileManager) LoadDirectory(path string)         { f.currentPath = path }
func (f *configScriptFakeFileManager) GetCurrentPath() string            { return f.currentPath }
func (f *configScriptFakeFileManager) GetFiles() []fileinfo.FileInfo     { return f.files }
func (f *configScriptFakeFileManager) GetSelectedFiles() map[string]bool { return f.selectedFiles }
func (f *configScriptFakeFileManager) SetFileSelected(path string, selected bool) {
	if f.selectedFiles == nil {
		f.selectedFiles = make(map[string]bool)
	}
	f.selectedFiles[path] = selected
}
func (f *configScriptFakeFileManager) RefreshFileList()                  {}
func (f *configScriptFakeFileManager) SaveCursorPosition(dirPath string) {}
func (f *configScriptFakeFileManager) OpenNewWindow()                    {}
func (f *configScriptFakeFileManager) ShowDirectoryTreeDialog()          {}
func (f *configScriptFakeFileManager) ShowNavigationHistoryDialog()      {}
func (f *configScriptFakeFileManager) ShowDirectoryJumpDialog()          {}
func (f *configScriptFakeFileManager) ShowFilterDialog()                 {}
func (f *configScriptFakeFileManager) ClearFilter()                      {}
func (f *configScriptFakeFileManager) ToggleFilter()                     {}
func (f *configScriptFakeFileManager) ShowIncrementalSearchDialog()      {}
func (f *configScriptFakeFileManager) ShowSortDialog()                   {}
func (f *configScriptFakeFileManager) ShowJobsDialog()                   {}
func (f *configScriptFakeFileManager) FocusPathEntry()                   {}
func (f *configScriptFakeFileManager) QuitApplication()                  {}
func (f *configScriptFakeFileManager) OpenFile(file *fileinfo.FileInfo)  {}
func (f *configScriptFakeFileManager) ShowCopyDialog()                   {}
func (f *configScriptFakeFileManager) ShowMoveDialog()                   {}
func (f *configScriptFakeFileManager) ShowRenameDialog()                 {}
func (f *configScriptFakeFileManager) ShowDeleteDialog(permanent bool)   {}
func (f *configScriptFakeFileManager) ShowExplorerContextMenu()          {}
func (f *configScriptFakeFileManager) ShowExternalCommandMenu()          {}
