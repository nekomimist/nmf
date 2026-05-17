package configscript

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"nmf/internal/config"
	"nmf/internal/display"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
)

func TestLoadAppliesStarlarkConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.window(width = 1000, height = 720)
nmf.theme(dark = False, font_size = 16, font_name = "Noto Sans")
if nmf.dark_theme():
    nmf.theme(font_name = "wrong")
nmf.color("cursor", dark = [1, 2, 3, 4], light = "foreground")
nmf.color("selectionBackground", value = "selection", dark = None)
nmf.color("lineEditCursor", value = "primary")
nmf.color("lineEditSelection", value = [5, 6, 7, 8])
nmf.color("dialogListCursor", value = "selection")
nmf.ui(show_hidden_files = True, item_spacing = 2)
nmf.sort(by = "extension", order = "desc", directories_first = False)
nmf.cursor_style(type = "border", thickness = 3)
nmf.cursor_memory(max_entries = 12)
nmf.navigation_history(max_entries = 9)
nmf.file_filter(max_entries = 7)
nmf.clear_directory_jumps()
nmf.directory_jump("proj", "~/projects")
nmf.clear_keys()
nmf.key("C-P", "user.parent", event = "down")
nmf.clear_external_commands()
nmf.external_command(
    name = "Open in Vim",
    exts = ["go", "md"],
    cmd = "vim",
    args = ["{file}"],
    cwd = "{dir}",
    edit = True,
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
	if got := cfg.Theme.Colors["cursor"].Dark.RGBA; got != [4]uint8{1, 2, 3, 4} {
		t.Fatalf("cursor dark color = %+v, want RGBA override", got)
	}
	if got := cfg.Theme.Colors["cursor"].Light.Name; got != "foreground" {
		t.Fatalf("cursor light color = %q, want foreground", got)
	}
	if !cfg.Theme.Colors["selectionBackground"].DarkDefault || cfg.Theme.Colors["selectionBackground"].Dark != nil {
		t.Fatalf("selection dark color should be reset to default")
	}
	if got := cfg.Theme.Colors["selectionBackground"].Value.Name; got != "selection" {
		t.Fatalf("selection common color = %q, want selection", got)
	}
	if got := cfg.Theme.Colors["lineEditCursor"].Value.Name; got != "primary" {
		t.Fatalf("line edit cursor color = %q, want primary", got)
	}
	if got := cfg.Theme.Colors["lineEditSelection"].Value.RGBA; got != [4]uint8{5, 6, 7, 8} {
		t.Fatalf("line edit selection color = %+v, want RGBA override", got)
	}
	if got := cfg.Theme.Colors["dialogListCursor"].Value.Name; got != "selection" {
		t.Fatalf("dialog list cursor color = %q, want selection", got)
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
	if len(cfg.UI.DirectoryJumps.Entries) != 1 ||
		cfg.UI.DirectoryJumps.Entries[0].Shortcut != "proj" ||
		cfg.UI.DirectoryJumps.Entries[0].Directory != "~/projects" {
		t.Fatalf("directory jumps = %+v, want one projects entry", cfg.UI.DirectoryJumps.Entries)
	}
	if len(cfg.UI.KeyBindings) != 1 || cfg.UI.KeyBindings[0].Command != "user.parent" {
		t.Fatalf("key bindings = %+v, want user.parent", cfg.UI.KeyBindings)
	}
	if len(cfg.UI.ExternalCommands) != 1 || cfg.UI.ExternalCommands[0].Command != "vim" || cfg.UI.ExternalCommands[0].Cwd != "{dir}" || !cfg.UI.ExternalCommands[0].Edit {
		t.Fatalf("external commands = %+v, want vim", cfg.UI.ExternalCommands)
	}
	if _, ok := rt.Commands["user.parent"]; !ok {
		t.Fatal("user.parent command was not registered")
	}
}

func TestDisplayCanDriveStarlarkConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
d = nmf.display()
if d.available:
    nmf.window(width = d.work_width - int(d.scale * 100), height = d.work_height - int(d.display_scale * 80))
    nmf.theme(font_size = int(d.user_scale * 12))
else:
    nmf.window(width = 800, height = 600)
    nmf.theme(font_size = 14)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := testConfig()
	displayInfo := display.Info{
		Available:    true,
		Name:         "Primary",
		Width:        2560,
		Height:       1440,
		WorkWidth:    2500,
		WorkHeight:   1360,
		PixelWidth:   2560,
		PixelHeight:  1440,
		Scale:        3,
		DisplayScale: 2,
		UserScale:    1.5,
	}
	rt, err := LoadWithDisplay(path, cfg, displayInfo, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("LoadWithDisplay returned error: %v", err)
	}

	if !rt.Loaded() {
		t.Fatal("runtime should report loaded init.star")
	}
	if cfg.Window.Width != 2200 || cfg.Window.Height != 1200 {
		t.Fatalf("window = %+v, want 2200x1200", cfg.Window)
	}
	if cfg.Theme.FontSize != 18 {
		t.Fatalf("font size = %d, want 18", cfg.Theme.FontSize)
	}
}

func TestDisplayUnavailableIsSafeInStarlarkConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
d = nmf.display()
if not d.available:
    nmf.window(width = 900, height = 650)
    nmf.theme(font_size = 14)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := testConfig()
	rt, err := LoadWithDisplay(path, cfg, display.Info{}, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("LoadWithDisplay returned error: %v", err)
	}

	if !rt.Loaded() {
		t.Fatal("runtime should report loaded init.star")
	}
	if cfg.Window.Width != 900 || cfg.Window.Height != 650 {
		t.Fatalf("window = %+v, want 900x650", cfg.Window)
	}
	if cfg.Theme.FontSize != 14 {
		t.Fatalf("font size = %d, want 14", cfg.Theme.FontSize)
	}
}

func TestDebugWritesToDebugLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.debug("display", 1200, True)
nmf.debug("font_size=" + str(18))
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	var logs []string
	if _, err := Load(path, testConfig(), func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []string{
		"ConfigScript: display 1200 True",
		"ConfigScript: font_size=18",
		"ConfigScript: loaded path=" + path + " commands=0",
	}
	if !reflect.DeepEqual(logs, want) {
		t.Fatalf("logs = %#v, want %#v", logs, want)
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

func TestKeyCanBindCallable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    nmf.exec("vim", args = [ctx.current_file])
nmf.key("E", fn = edit)
nmf.key("C-E", fn = edit, event = "down")
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := testConfig()
	rt, err := Load(path, cfg, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.UI.KeyBindings) != 2 {
		t.Fatalf("key bindings = %+v, want two generated bindings", cfg.UI.KeyBindings)
	}
	first := cfg.UI.KeyBindings[0]
	second := cfg.UI.KeyBindings[1]
	if first.Key != "E" || first.Command == "" || first.Event != "" {
		t.Fatalf("first binding = %+v, want E generated command", first)
	}
	if second.Key != "C-E" || second.Command == "" || second.Event != "down" {
		t.Fatalf("second binding = %+v, want C-E generated down command", second)
	}
	if first.Command == second.Command {
		t.Fatalf("generated commands should be unique, got %q", first.Command)
	}
	if _, ok := rt.Commands[first.Command]; !ok {
		t.Fatalf("missing generated command %q", first.Command)
	}

	var gotCommand string
	var gotArgs []string
	rt.Commands[first.Command](keymanager.CommandContext{
		FileManager: &configScriptFakeFileManager{
			currentPath: "/tmp",
			cursorIndex: 0,
			files: []fileinfo.FileInfo{
				{Name: "main.go", Path: "/tmp/main.go"},
			},
		},
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotCommand = command
			gotArgs = args
			return true
		},
	})
	if gotCommand != "vim" || !reflect.DeepEqual(gotArgs, []string{"/tmp/main.go"}) {
		t.Fatalf("exec = %q %#v, want vim current file", gotCommand, gotArgs)
	}
}

func TestKeyRejectsInvalidCallableBindings(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "both command and fn", src: `
def f(ctx):
    pass
nmf.key("E", "directory.refresh", fn = f)
`},
		{name: "neither cmd nor fn", src: `nmf.key("E")`},
		{name: "non callable fn", src: `nmf.key("E", fn = "nope")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, FileName)
			if err := os.WriteFile(path, []byte(tt.src), 0644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			if _, err := Load(path, testConfig(), func(string, ...interface{}) {}); err == nil {
				t.Fatal("Load should reject invalid key binding")
			}
		})
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

func TestCustomCommandCanApplyTemporarySort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def sort_size(ctx):
    nmf.sort(by = "size", order = "desc", directories_first = False, temporary = True)
nmf.command("user.sort_size", sort_size)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cfg := testConfig()
	rt, err := Load(path, cfg, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{}
	rt.Commands["user.sort_size"](keymanager.CommandContext{FileManager: fm})

	want := config.SortConfig{SortBy: "size", SortOrder: "desc", DirectoriesFirst: false}
	if !fm.temporarySortApplied || !reflect.DeepEqual(fm.temporarySort, want) {
		t.Fatalf("temporary sort = applied %t config %+v, want %+v", fm.temporarySortApplied, fm.temporarySort, want)
	}
	if cfg.UI.Sort.SortBy != "name" || cfg.UI.Sort.SortOrder != "asc" || !cfg.UI.Sort.DirectoriesFirst {
		t.Fatalf("persistent sort changed to %+v, want default", cfg.UI.Sort)
	}
}

func TestCustomCommandCanReadCurrentSort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def toggle_sort(ctx):
    sort = nmf.current_sort()
    if sort.by == "modified":
        by = "name"
    else:
        by = "modified"
    nmf.sort(by = by, order = "desc", directories_first = sort.directories_first, temporary = True)
nmf.command("user.toggle_sort", toggle_sort)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{
		currentSort: config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true},
	}
	rt.Commands["user.toggle_sort"](keymanager.CommandContext{FileManager: fm})

	want := config.SortConfig{SortBy: "modified", SortOrder: "desc", DirectoriesFirst: true}
	if !reflect.DeepEqual(fm.currentSort, want) {
		t.Fatalf("current sort = %+v, want %+v", fm.currentSort, want)
	}
}

func TestCurrentSortRequiresCommandContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(`nmf.current_sort()`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := Load(path, testConfig(), func(string, ...interface{}) {}); err == nil {
		t.Fatal("Load should reject nmf.current_sort outside a command")
	}
}

func TestLoadRegistersStarlarkMenu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.menu("tools", title = "Tools")
nmf.menu_item("tools", "Refresh", cmd = "directory.refresh")
nmf.menu_separator("tools")
def edit(ctx):
    nmf.exec("vim", args = [ctx.current_file])
nmf.menu_item("tools", "Edit", fn = edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	menu := rt.Menus["tools"]
	if menu == nil {
		t.Fatal("tools menu was not registered")
	}
	if menu.Title != "Tools" {
		t.Fatalf("menu title = %q, want Tools", menu.Title)
	}
	if len(menu.Items) != 3 || menu.Items[0].Label != "Refresh" || menu.Items[0].Command != "directory.refresh" || !menu.Items[1].Separator || menu.Items[2].Label != "Edit" || menu.Items[2].Callable == nil {
		t.Fatalf("menu items = %+v, want command, separator, and callable items", menu.Items)
	}
}

func TestCustomCommandCanShowStarlarkMenu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.menu("tools", title = "Tools")
nmf.menu_item("tools", "Refresh", cmd = "directory.refresh")
nmf.menu_separator("tools")
def edit(ctx):
    nmf.exec("vim", args = [ctx.current_file])
nmf.menu_item("tools", "Edit", fn = edit)
def show_tools(ctx):
    nmf.show_menu("tools")
nmf.command("user.show_tools", show_tools)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{
		currentPath: "/tmp",
		cursorIndex: 0,
		files: []fileinfo.FileInfo{
			{Name: "main.go", Path: "/tmp/main.go"},
		},
	}
	var ran string
	var gotCommand string
	var gotArgs []string
	var gotEdit bool
	rt.Commands["user.show_tools"](keymanager.CommandContext{
		FileManager: fm,
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotCommand = command
			gotArgs = args
			gotEdit = edit
			return true
		},
	})

	if fm.menuTitle != "Tools" {
		t.Fatalf("menu title = %q, want Tools", fm.menuTitle)
	}
	if labels := fm.menuLabels(); !reflect.DeepEqual(labels, []string{"Refresh", "", "Edit"}) {
		t.Fatalf("menu labels = %#v, want Refresh/separator/Edit", labels)
	}
	if !fm.menuItems[1].Separator {
		t.Fatalf("menu item 1 = %+v, want separator", fm.menuItems[1])
	}

	fm.menuItems[0].Action()
	if ran != "directory.refresh" {
		t.Fatalf("ran command = %q, want directory.refresh", ran)
	}

	fm.menuItems[2].Action()
	if gotCommand != "vim" || !reflect.DeepEqual(gotArgs, []string{"/tmp/main.go"}) {
		t.Fatalf("exec = %q %#v, want vim current file", gotCommand, gotArgs)
	}
	if gotEdit {
		t.Fatal("exec edit = true, want false")
	}
}

func TestCustomCommandDefersStarlarkMenuDisplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.menu("tools", title = "Tools")
nmf.menu_item("tools", "Refresh", cmd = "directory.refresh")
def show_tools(ctx):
    nmf.show_menu("tools")
nmf.command("user.show_tools", show_tools)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{}
	var label string
	var deferred func()
	rt.Commands["user.show_tools"](keymanager.CommandContext{
		FileManager: fm,
		DeferTransition: func(gotLabel string, action func()) {
			label = gotLabel
			deferred = action
		},
	})

	if fm.menuTitle != "" {
		t.Fatalf("menu title before deferred action = %q, want empty", fm.menuTitle)
	}
	if label != "starlark.show_menu" || deferred == nil {
		t.Fatalf("deferred transition = label %q action nil=%t, want starlark.show_menu action", label, deferred == nil)
	}

	deferred()
	if fm.menuTitle != "Tools" {
		t.Fatalf("menu title after deferred action = %q, want Tools", fm.menuTitle)
	}
}

func TestShowMenuDisplaysInformationalItemForMissingMenu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def show_missing(ctx):
    nmf.show_menu("missing")
nmf.command("user.show_missing", show_missing)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{}
	rt.Commands["user.show_missing"](keymanager.CommandContext{FileManager: fm})

	if fm.menuTitle != "missing" || len(fm.menuItems) != 1 || fm.menuItems[0].Label == "" {
		t.Fatalf("missing menu display = title %q items %+v, want one informational item", fm.menuTitle, fm.menuItems)
	}
}

func TestMenuRegistrationRejectsInvalidItems(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "empty menu", src: `nmf.menu("")`},
		{name: "empty label", src: `nmf.menu_item("tools", "", cmd = "directory.refresh")`},
		{name: "empty separator menu", src: `nmf.menu_separator("")`},
		{name: "both actions", src: `
def f(ctx):
    pass
nmf.menu_item("tools", "Bad", cmd = "directory.refresh", fn = f)
`},
		{name: "no actions", src: `nmf.menu_item("tools", "Bad")`},
		{name: "non callable fn", src: `nmf.menu_item("tools", "Bad", fn = "nope")`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, FileName)
			if err := os.WriteFile(path, []byte(tt.src), 0644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			if _, err := Load(path, testConfig(), func(string, ...interface{}) {}); err == nil {
				t.Fatal("Load should reject invalid menu registration")
			}
		})
	}
}

func TestMenuRegistrationFailsInsideCustomCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def bad(ctx):
    nmf.menu("late")
nmf.command("user.bad", bad)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var logs []string
	rt.debugPrint = func(format string, args ...interface{}) {
		logs = append(logs, format)
	}
	rt.Commands["user.bad"](keymanager.CommandContext{})
	if len(logs) == 0 {
		t.Fatal("command should log menu registration failure")
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
	var gotEdit bool
	var gotCwd string
	var ran string
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotCommand = command
			gotArgs = args
			gotEdit = edit
			gotCwd = cwd
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
	if gotEdit {
		t.Fatal("exec edit = true, want false")
	}
	if gotCwd != "" {
		t.Fatalf("exec cwd = %q, want empty", gotCwd)
	}
	if ran != "directory.refresh" {
		t.Fatalf("ran command = %q, want directory.refresh", ran)
	}
}

func TestCustomCommandCanExecExternalCommandWithCwd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    nmf.exec("vim", args = [ctx.current_file], cwd = ctx.current_path)
nmf.command("user.edit", edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var gotCwd string
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotCwd = cwd
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
	if gotCwd != "/tmp" {
		t.Fatalf("exec cwd = %q, want /tmp", gotCwd)
	}
}

func TestCustomCommandCanExecExternalCommandWithEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    nmf.exec("vim", args = [ctx.current_file], edit = True)
nmf.command("user.edit", edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var gotEdit bool
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotEdit = edit
			return false
		},
		FileManager: &configScriptFakeFileManager{
			currentPath: "/tmp",
			cursorIndex: 0,
			files: []fileinfo.FileInfo{
				{Name: "main.go", Path: "/tmp/main.go"},
			},
		},
	})
	if !gotEdit {
		t.Fatal("exec edit = false, want true")
	}
}

func TestCustomCommandDefersEditableExec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    nmf.exec("vim", args = [ctx.current_file], edit = True)
nmf.command("user.edit", edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var label string
	var deferred func()
	var gotCommand string
	var gotArgs []string
	var gotEdit bool
	var gotCwd string
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotCommand = command
			gotArgs = args
			gotEdit = edit
			gotCwd = cwd
			return false
		},
		DeferTransition: func(gotLabel string, action func()) {
			label = gotLabel
			deferred = action
		},
		FileManager: &configScriptFakeFileManager{
			currentPath: "/tmp",
			cursorIndex: 0,
			files: []fileinfo.FileInfo{
				{Name: "main.go", Path: "/tmp/main.go"},
			},
		},
	})

	if gotCommand != "" || gotEdit {
		t.Fatalf("exec before deferred action = %q edit %t, want not run", gotCommand, gotEdit)
	}
	if label != "starlark.exec.edit" || deferred == nil {
		t.Fatalf("deferred transition = label %q action nil=%t, want starlark.exec.edit action", label, deferred == nil)
	}

	deferred()
	if gotCommand != "vim" || !reflect.DeepEqual(gotArgs, []string{"/tmp/main.go"}) || !gotEdit || gotCwd != "" {
		t.Fatalf("exec after deferred action = %q %#v edit %t cwd %q, want vim current file edit=true cwd empty", gotCommand, gotArgs, gotEdit, gotCwd)
	}
}

func TestCustomCommandDefersEditableExecWithCwd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    nmf.exec("vim", edit = True, cwd = ctx.current_path)
nmf.command("user.edit", edit)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var deferred func()
	var gotCwd string
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotCwd = cwd
			return false
		},
		DeferTransition: func(gotLabel string, action func()) {
			deferred = action
		},
		FileManager: &configScriptFakeFileManager{currentPath: "/tmp"},
	})

	if gotCwd != "" {
		t.Fatalf("exec cwd before deferred action = %q, want empty", gotCwd)
	}
	if deferred == nil {
		t.Fatal("deferred action is nil")
	}

	deferred()
	if gotCwd != "/tmp" {
		t.Fatalf("exec cwd after deferred action = %q, want /tmp", gotCwd)
	}
}

func TestCustomCommandCanExecEmptyCommandWithEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def edit(ctx):
    nmf.exec("", edit = True)
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
	var gotEdit bool
	rt.Commands["user.edit"](keymanager.CommandContext{
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotCommand = command
			gotEdit = edit
			return false
		},
	})
	if gotCommand != "" || !gotEdit {
		t.Fatalf("exec = command %q edit %t, want empty command edit=true", gotCommand, gotEdit)
	}
}

func TestExternalCommandAllowsEmptyCommandWithEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
nmf.external_command(
    name = "Prompt",
    cmd = "",
    cwd = "{dir}",
    edit = True,
)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cfg := testConfig()
	if _, err := Load(path, cfg, func(string, ...interface{}) {}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.UI.ExternalCommands) != 1 || cfg.UI.ExternalCommands[0].Command != "" || cfg.UI.ExternalCommands[0].Cwd != "{dir}" || !cfg.UI.ExternalCommands[0].Edit {
		t.Fatalf("external commands = %+v, want empty editable command", cfg.UI.ExternalCommands)
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
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
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
    nmf.exec("inspect", args = [ctx.current_path, ctx.current_file, ctx.current_name] + ctx.selected_files + ctx.all_selected_files)
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
			allSelectedFiles: []fileinfo.FileInfo{
				{Name: "other.txt", Path: "/tmp/other/other.txt"},
				{Name: "..", Path: "/tmp"},
				{Name: "deleted.txt", Path: "/tmp/other/deleted.txt", Status: fileinfo.StatusDeleted},
			},
		},
		RunExternalCommand: func(command string, args []string, edit bool, cwd string) bool {
			gotArgs = args
			return true
		},
	})

	want := []string{"/tmp/work", "/tmp/work/notes.md", "notes.md", "/tmp/work/notes.md", "/tmp/other/other.txt"}
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

func TestCustomCommandCanShowMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def warn(ctx):
    if nmf.message("Select at least two files.", title = "Compare"):
        nmf.run("directory.refresh")
nmf.command("user.warn", warn)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{}
	var ran string
	rt.Commands["user.warn"](keymanager.CommandContext{
		FileManager: fm,
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
	})

	if fm.showMessageCount != 1 || fm.messageTitle != "Compare" || fm.messageText != "Select at least two files." {
		t.Fatalf("message = count %d title %q text %q, want Compare warning", fm.showMessageCount, fm.messageTitle, fm.messageText)
	}
	if ran != "directory.refresh" {
		t.Fatalf("ran command = %q, want directory.refresh", ran)
	}
}

func TestCustomCommandDefersStarlarkMessageDisplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def warn(ctx):
    nmf.message("Need two files")
nmf.command("user.warn", warn)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{}
	var label string
	var deferred func()
	rt.Commands["user.warn"](keymanager.CommandContext{
		FileManager: fm,
		DeferTransition: func(l string, action func()) {
			label = l
			deferred = action
		},
	})

	if label != "starlark.message" || deferred == nil {
		t.Fatalf("deferred transition = label %q action nil=%t, want starlark.message action", label, deferred == nil)
	}
	if fm.showMessageCount != 0 {
		t.Fatalf("message shown before deferred action count = %d, want 0", fm.showMessageCount)
	}
	deferred()
	if fm.showMessageCount != 1 || fm.messageTitle != "Message" || fm.messageText != "Need two files" {
		t.Fatalf("message = count %d title %q text %q, want default-title message", fm.showMessageCount, fm.messageTitle, fm.messageText)
	}
}

func TestMessageRequiresCommandContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `nmf.message("hello")`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err == nil || !strings.Contains(err.Error(), "nmf.message can only be used while a custom command is running") {
		t.Fatalf("Load error = %v, want command-context error", err)
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

func TestCustomCommandCanCreateDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def create(ctx):
    if nmf.mkdir("new-dir"):
        nmf.run("directory.refresh")
nmf.command("user.create", create)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{createDirResult: true}
	var ran string
	rt.Commands["user.create"](keymanager.CommandContext{
		FileManager: fm,
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
	})

	if fm.createDirName != "new-dir" {
		t.Fatalf("created directory name = %q, want new-dir", fm.createDirName)
	}
	if ran != "directory.refresh" {
		t.Fatalf("ran command = %q, want directory.refresh", ran)
	}
}

func TestCustomCommandCanSetClipboard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def copy_current(ctx):
    nmf.clipboard(ctx.current_path + "\n" + ctx.current_name)
nmf.command("user.copy_current", copy_current)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var got string
	rt.Commands["user.copy_current"](keymanager.CommandContext{
		FileManager: &configScriptFakeFileManager{
			currentPath: "/tmp/work",
			cursorIndex: 0,
			files: []fileinfo.FileInfo{
				{Name: "main.go", Path: "/tmp/work/main.go"},
			},
		},
		SetClipboard: func(text string) bool {
			got = text
			return true
		},
	})

	if got != "/tmp/work\nmain.go" {
		t.Fatalf("clipboard text = %q, want current path and name", got)
	}
}

func TestClipboardReturnsFalseWithoutWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def copy_text(ctx):
    if nmf.clipboard("hello"):
        nmf.run("directory.refresh")
nmf.command("user.copy_text", copy_text)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var ran string
	rt.Commands["user.copy_text"](keymanager.CommandContext{
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
	})
	if ran != "" {
		t.Fatalf("ran command = %q, want none", ran)
	}
}

func TestClipboardRequiresCommandContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(`nmf.clipboard("hello")`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := Load(path, testConfig(), func(string, ...interface{}) {}); err == nil {
		t.Fatal("Load should reject nmf.clipboard outside a command")
	}
}

func TestClipboardRejectsNonString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def copy_text(ctx):
    nmf.clipboard(1)
nmf.command("user.copy_text", copy_text)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var logs []string
	rt.debugPrint = func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}
	rt.Commands["user.copy_text"](keymanager.CommandContext{
		SetClipboard: func(text string) bool { return true },
	})
	if len(logs) == 0 {
		t.Fatal("command should log clipboard argument failure")
	}
}

func TestCustomCommandCanSaveClipboardTextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def save(ctx):
    if nmf.save_clipboard("clip.txt"):
        nmf.run("directory.refresh")
nmf.command("user.save", save)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{clipboardFileResult: true}
	var ran string
	rt.Commands["user.save"](keymanager.CommandContext{
		FileManager: fm,
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
	})

	if fm.clipboardFileName != "clip.txt" {
		t.Fatalf("clipboard file name = %q, want clip.txt", fm.clipboardFileName)
	}
	if ran != "directory.refresh" {
		t.Fatalf("ran command = %q, want directory.refresh", ran)
	}
}

func TestSaveClipboardReturnsFalseWithoutFileManager(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def save(ctx):
    if nmf.save_clipboard("clip.txt"):
        nmf.run("directory.refresh")
nmf.command("user.save", save)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var ran string
	rt.Commands["user.save"](keymanager.CommandContext{
		RunCommand: func(command string) bool {
			ran = command
			return true
		},
	})
	if ran != "" {
		t.Fatalf("ran command = %q, want none", ran)
	}
}

func TestSaveClipboardRequiresCommandContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(`nmf.save_clipboard("clip.txt")`), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := Load(path, testConfig(), func(string, ...interface{}) {}); err == nil {
		t.Fatal("Load should reject nmf.save_clipboard outside a command")
	}
}

func TestSaveClipboardRequiresNameWhenNotEditing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def save(ctx):
    nmf.save_clipboard()
nmf.command("user.save", save)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	var logs []string
	rt.debugPrint = func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}
	rt.Commands["user.save"](keymanager.CommandContext{FileManager: &configScriptFakeFileManager{}})
	if len(logs) == 0 {
		t.Fatal("command should log save_clipboard argument failure")
	}
}

func TestCustomCommandDefersEditableMkdir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def create(ctx):
    nmf.mkdir(edit = True)
nmf.command("user.create", create)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{}
	var label string
	var deferred func()
	rt.Commands["user.create"](keymanager.CommandContext{
		FileManager: fm,
		DeferTransition: func(gotLabel string, action func()) {
			label = gotLabel
			deferred = action
		},
	})

	if fm.showCreateDirCount != 0 {
		t.Fatalf("ShowCreateDirectoryDialog count before deferred action = %d, want 0", fm.showCreateDirCount)
	}
	if label != "starlark.mkdir.edit" || deferred == nil {
		t.Fatalf("deferred transition = label %q action nil=%t, want starlark.mkdir.edit action", label, deferred == nil)
	}

	deferred()
	if fm.showCreateDirCount != 1 {
		t.Fatalf("ShowCreateDirectoryDialog count after deferred action = %d, want 1", fm.showCreateDirCount)
	}
}

func TestCustomCommandDefersEditableSaveClipboard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	src := `
def save(ctx):
    nmf.save_clipboard(edit = True)
nmf.command("user.save", save)
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	rt, err := Load(path, testConfig(), func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	fm := &configScriptFakeFileManager{}
	var label string
	var deferred func()
	rt.Commands["user.save"](keymanager.CommandContext{
		FileManager: fm,
		DeferTransition: func(gotLabel string, action func()) {
			label = gotLabel
			deferred = action
		},
	})

	if fm.showClipboardFileCount != 0 {
		t.Fatalf("ShowClipboardTextFileDialog count before deferred action = %d, want 0", fm.showClipboardFileCount)
	}
	if label != "starlark.save_clipboard.edit" || deferred == nil {
		t.Fatalf("deferred transition = label %q action nil=%t, want starlark.save_clipboard.edit action", label, deferred == nil)
	}

	deferred()
	if fm.showClipboardFileCount != 1 {
		t.Fatalf("ShowClipboardTextFileDialog count after deferred action = %d, want 1", fm.showClipboardFileCount)
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
		{
			name: "non string cwd",
			src:  `nmf.exec("vim", cwd = 1)`,
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
	currentPath            string
	cursorIndex            int
	files                  []fileinfo.FileInfo
	selectedFiles          map[string]bool
	allSelectedFiles       []fileinfo.FileInfo
	currentSort            config.SortConfig
	temporarySort          config.SortConfig
	temporarySortApplied   bool
	createDirName          string
	createDirResult        bool
	showCreateDirCount     int
	clipboardFileName      string
	clipboardFileResult    bool
	showClipboardFileCount int
	messageTitle           string
	messageText            string
	showMessageCount       int
	menuTitle              string
	menuItems              []keymanager.CommandMenuItem
}

func (f *configScriptFakeFileManager) GetCurrentCursorIndex() int    { return f.cursorIndex }
func (f *configScriptFakeFileManager) SetCursorByIndex(index int)    { f.cursorIndex = index }
func (f *configScriptFakeFileManager) RefreshCursor()                {}
func (f *configScriptFakeFileManager) LoadDirectory(path string)     { f.currentPath = path }
func (f *configScriptFakeFileManager) GetCurrentPath() string        { return f.currentPath }
func (f *configScriptFakeFileManager) GetFiles() []fileinfo.FileInfo { return f.files }
func (f *configScriptFakeFileManager) CurrentSort() config.SortConfig {
	if f.currentSort.SortBy == "" {
		return config.SortConfig{SortBy: "name", SortOrder: "asc", DirectoriesFirst: true}
	}
	return f.currentSort
}
func (f *configScriptFakeFileManager) ApplyTemporarySort(sortConfig config.SortConfig) {
	f.currentSort = sortConfig
	f.temporarySort = sortConfig
	f.temporarySortApplied = true
}
func (f *configScriptFakeFileManager) GetSelectedFiles() map[string]bool { return f.selectedFiles }
func (f *configScriptFakeFileManager) GetAllSelectedFiles() []fileinfo.FileInfo {
	if f.allSelectedFiles != nil {
		return f.allSelectedFiles
	}
	files := f.GetFiles()
	selected := f.GetSelectedFiles()
	targets := make([]fileinfo.FileInfo, 0, len(selected))
	for _, fi := range files {
		if selected[fi.Path] {
			targets = append(targets, fi)
		}
	}
	return targets
}
func (f *configScriptFakeFileManager) SetFileSelected(path string, selected bool) {
	if f.selectedFiles == nil {
		f.selectedFiles = make(map[string]bool)
	}
	f.selectedFiles[path] = selected
}
func (f *configScriptFakeFileManager) RefreshFileList()                  {}
func (f *configScriptFakeFileManager) SaveCursorPosition(dirPath string) {}
func (f *configScriptFakeFileManager) OpenNewWindow()                    {}
func (f *configScriptFakeFileManager) FocusWindowLeft()                  {}
func (f *configScriptFakeFileManager) FocusWindowRight()                 {}
func (f *configScriptFakeFileManager) ShowDirectoryTreeDialog()          {}
func (f *configScriptFakeFileManager) ShowNavigationHistoryDialog()      {}
func (f *configScriptFakeFileManager) ShowDirectoryJumpDialog()          {}
func (f *configScriptFakeFileManager) ShowFilterDialog()                 {}
func (f *configScriptFakeFileManager) ClearFilter()                      {}
func (f *configScriptFakeFileManager) ToggleFilter()                     {}
func (f *configScriptFakeFileManager) ShowIncrementalSearchDialog()      {}
func (f *configScriptFakeFileManager) ShowSortDialog()                   {}
func (f *configScriptFakeFileManager) ShowJobsDialog()                   {}
func (f *configScriptFakeFileManager) ShowPathEditDialog()               {}
func (f *configScriptFakeFileManager) ShowCreateDirectoryDialog()        { f.showCreateDirCount++ }
func (f *configScriptFakeFileManager) CreateDirectory(name string) bool {
	f.createDirName = name
	return f.createDirResult
}
func (f *configScriptFakeFileManager) ShowClipboardTextFileDialog() { f.showClipboardFileCount++ }
func (f *configScriptFakeFileManager) CreateClipboardTextFile(name string) bool {
	f.clipboardFileName = name
	return f.clipboardFileResult
}
func (f *configScriptFakeFileManager) ShowMessageDialog(title string, message string) {
	f.messageTitle = title
	f.messageText = message
	f.showMessageCount++
}
func (f *configScriptFakeFileManager) QuitApplication()                 {}
func (f *configScriptFakeFileManager) OpenFile(file *fileinfo.FileInfo) {}
func (f *configScriptFakeFileManager) ShowCopyDialog()                  {}
func (f *configScriptFakeFileManager) ShowMoveDialog()                  {}
func (f *configScriptFakeFileManager) ShowRenameDialog()                {}
func (f *configScriptFakeFileManager) ShowDeleteDialog(permanent bool)  {}
func (f *configScriptFakeFileManager) ShowExplorerContextMenu()         {}
func (f *configScriptFakeFileManager) ShowExternalCommandMenu()         {}
func (f *configScriptFakeFileManager) ShowFileViewer()                  {}
func (f *configScriptFakeFileManager) ShowCommandMenu(title string, items []keymanager.CommandMenuItem) {
	f.menuTitle = title
	f.menuItems = items
}
func (f *configScriptFakeFileManager) menuLabels() []string {
	labels := make([]string, len(f.menuItems))
	for i, item := range f.menuItems {
		labels[i] = item.Label
	}
	return labels
}
