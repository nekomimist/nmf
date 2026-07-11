package configscript

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

	"nmf/internal/config"
	"nmf/internal/display"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	customtheme "nmf/internal/theme"
)

const (
	// FileName is the Starlark initialization file name.
	FileName = "init.star"

	commandPrefix       = "user."
	commandContextKey   = "nmf.commandContext"
	maxExecutionSteps   = 1_000_000
	maxLoadModuleDepth  = 32
	defaultModuleSuffix = ".star"
)

// Runtime holds Starlark-defined runtime behavior.
type Runtime struct {
	Commands keymanager.CommandRegistry
	Menus    map[string]*Menu

	cfg        *config.Config
	configDir  string
	display    display.Info
	debugPrint func(format string, args ...interface{})
	debugHook  func(config.DebugConfig) error
	loaded     bool

	moduleCache   map[string]starlark.StringDict
	loading       map[string]bool
	loadDepth     int
	keyCommandSeq int
	deprecated    map[string]bool
}

// Menu holds Starlark-defined menu metadata and entries.
type Menu struct {
	Name  string
	Title string
	Items []MenuItem
}

// MenuItem holds a Starlark-defined menu item action.
type MenuItem struct {
	Label     string
	Key       string
	Command   string
	Separator bool
	Callable  starlark.Callable
}

// ScriptPath returns the init.star path next to config.json.
func ScriptPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), FileName)
}

// Options configures Load.
type Options struct {
	Display    display.Info
	DebugPrint func(format string, args ...interface{})
	DebugHook  func(config.DebugConfig) error
}

// Load reads and executes init.star if it exists.
func Load(path string, cfg *config.Config, opts Options) (*Runtime, error) {
	rt := newRuntime(path, cfg, opts.Display, opts.DebugPrint)
	rt.debugHook = opts.DebugHook

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			rt.debugPrint("ConfigScript: init file not found path=%s", path)
			return rt, nil
		}
		return nil, fmt.Errorf("reading Starlark config %s: %w", path, err)
	}

	if err := rt.execFile(path, data); err != nil {
		return nil, err
	}
	rt.loaded = true
	rt.debugPrint("ConfigScript: loaded path=%s commands=%d", path, len(rt.Commands))
	return rt, nil
}

func newRuntime(path string, cfg *config.Config, displayInfo display.Info, debugPrint func(format string, args ...interface{})) *Runtime {
	configDir := filepath.Dir(path)
	if abs, err := filepath.Abs(configDir); err == nil {
		configDir = abs
	}
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}
	return &Runtime{
		Commands:    make(keymanager.CommandRegistry),
		Menus:       make(map[string]*Menu),
		cfg:         cfg,
		configDir:   configDir,
		display:     displayInfo,
		debugPrint:  debugPrint,
		moduleCache: make(map[string]starlark.StringDict),
		loading:     make(map[string]bool),
		deprecated:  make(map[string]bool),
	}
}

// warnDeprecated logs a deprecation warning via debugPrint the first time each
// feature key is seen for this Runtime.
func (rt *Runtime) warnDeprecated(feature string, format string, args ...interface{}) {
	if rt.deprecated == nil {
		rt.deprecated = make(map[string]bool)
	}
	if rt.deprecated[feature] {
		return
	}
	rt.deprecated[feature] = true
	rt.debugPrint("ConfigScript: WARNING "+format, args...)
}

// Loaded reports whether an init.star file was found and executed.
func (rt *Runtime) Loaded() bool {
	return rt != nil && rt.loaded
}

func (rt *Runtime) execFile(path string, data []byte) error {
	thread := rt.newThread("nmf init")
	globals, err := starlark.ExecFileOptions(rt.fileOptions(), thread, path, data, rt.predeclared())
	if err != nil {
		return fmt.Errorf("executing Starlark config %s: %s", path, formatStarlarkError(err))
	}
	rt.moduleCache[path] = globals
	return nil
}

func (rt *Runtime) fileOptions() *syntax.FileOptions {
	return &syntax.FileOptions{
		TopLevelControl: true,
	}
}

func (rt *Runtime) newThread(name string) *starlark.Thread {
	thread := &starlark.Thread{
		Name: name,
		Print: func(_ *starlark.Thread, msg string) {
			rt.debugPrint("ConfigScript: %s", msg)
		},
		Load: func(_ *starlark.Thread, module string) (starlark.StringDict, error) {
			return rt.loadModule(module)
		},
	}
	thread.SetMaxExecutionSteps(maxExecutionSteps)
	return thread
}

func (rt *Runtime) loadModule(module string) (starlark.StringDict, error) {
	path, err := rt.resolveModule(module)
	if err != nil {
		return nil, err
	}
	if globals, ok := rt.moduleCache[path]; ok {
		return globals, nil
	}
	if rt.loading[path] {
		return nil, fmt.Errorf("cyclic Starlark load: %s", module)
	}
	if rt.loadDepth >= maxLoadModuleDepth {
		return nil, fmt.Errorf("Starlark load depth exceeded at %s", module)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	rt.loading[path] = true
	rt.loadDepth++
	defer func() {
		rt.loadDepth--
		delete(rt.loading, path)
	}()

	thread := rt.newThread("nmf load " + module)
	globals, err := starlark.ExecFileOptions(rt.fileOptions(), thread, path, data, rt.predeclared())
	if err != nil {
		return nil, fmt.Errorf("%s", formatStarlarkError(err))
	}
	rt.moduleCache[path] = globals
	return globals, nil
}

func (rt *Runtime) resolveModule(module string) (string, error) {
	name := strings.TrimSpace(module)
	if name == "" {
		return "", fmt.Errorf("empty Starlark module name")
	}
	name = filepath.FromSlash(name)
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("absolute Starlark loads are not allowed: %s", module)
	}
	if filepath.Ext(name) == "" {
		name += defaultModuleSuffix
	}

	path := filepath.Clean(filepath.Join(rt.configDir, name))
	rel, err := filepath.Rel(rt.configDir, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("Starlark load outside config directory is not allowed: %s", module)
	}
	return path, nil
}

func (rt *Runtime) predeclared() starlark.StringDict {
	return starlark.StringDict{
		"nmf": starlarkstruct.FromStringDict(starlark.String("nmf"), starlark.StringDict{
			"window":             starlark.NewBuiltin("nmf.window", rt.builtinWindow),
			"startup":            starlark.NewBuiltin("nmf.startup", rt.builtinStartup),
			"theme":              starlark.NewBuiltin("nmf.theme", rt.builtinTheme),
			"color":              starlark.NewBuiltin("nmf.color", rt.builtinColor),
			"dark_theme":         starlark.NewBuiltin("nmf.dark_theme", rt.builtinDarkTheme),
			"debug_logging":      starlark.NewBuiltin("nmf.debug_logging", rt.builtinDebugLogging),
			"ui":                 starlark.NewBuiltin("nmf.ui", rt.builtinUI),
			"copy":               starlark.NewBuiltin("nmf.copy", rt.builtinCopy),
			"viewer":             starlark.NewBuiltin("nmf.viewer", rt.builtinViewer),
			"archive":            starlark.NewBuiltin("nmf.archive", rt.builtinArchive),
			"sort":               starlark.NewBuiltin("nmf.sort", rt.builtinSort),
			"cursor_style":       starlark.NewBuiltin("nmf.cursor_style", rt.builtinCursorStyle),
			"cursor_memory":      starlark.NewBuiltin("nmf.cursor_memory", rt.builtinCursorMemory),
			"navigation_history": starlark.NewBuiltin("nmf.navigation_history", rt.builtinNavigationHistory),
			"file_filter":        starlark.NewBuiltin("nmf.file_filter", rt.builtinFileFilter),
			"directory_jump":     starlark.NewBuiltin("nmf.directory_jump", rt.builtinDirectoryJump),
			"clear_directory_jumps": starlark.NewBuiltin(
				"nmf.clear_directory_jumps",
				rt.builtinClearDirectoryJumps,
			),
			"key":        starlark.NewBuiltin("nmf.key", rt.builtinKey),
			"unkey":      starlark.NewBuiltin("nmf.unkey", rt.builtinUnkey),
			"clear_keys": starlark.NewBuiltin("nmf.clear_keys", rt.builtinClearKeys),
			"external_command": starlark.NewBuiltin(
				"nmf.external_command",
				rt.builtinExternalCommand,
			),
			"clear_external_commands": starlark.NewBuiltin(
				"nmf.clear_external_commands",
				rt.builtinClearExternalCommands,
			),
			"command":        starlark.NewBuiltin("nmf.command", rt.builtinCommand),
			"menu":           starlark.NewBuiltin("nmf.menu", rt.builtinMenu),
			"menu_item":      starlark.NewBuiltin("nmf.menu_item", rt.builtinMenuItem),
			"menu_separator": starlark.NewBuiltin("nmf.menu_separator", rt.builtinMenuSeparator),
			"clear_menu":     starlark.NewBuiltin("nmf.clear_menu", rt.builtinClearMenu),
			"show_menu":      starlark.NewBuiltin("nmf.show_menu", rt.builtinShowMenu),
			"run":            starlark.NewBuiltin("nmf.run", rt.builtinRun),
			"message":        starlark.NewBuiltin("nmf.message", rt.builtinMessage),
			"exec":           starlark.NewBuiltin("nmf.exec", rt.builtinExec),
			"mkdir":          starlark.NewBuiltin("nmf.mkdir", rt.builtinMkdir),
			"clipboard":      starlark.NewBuiltin("nmf.clipboard", rt.builtinClipboard),
			"set_clipboard":  starlark.NewBuiltin("nmf.set_clipboard", rt.builtinSetClipboard),
			"save_clipboard": starlark.NewBuiltin("nmf.save_clipboard", rt.builtinSaveClipboard),
			"load_directory": starlark.NewBuiltin("nmf.load_directory", rt.builtinLoadDirectory),
			"current_path":   starlark.NewBuiltin("nmf.current_path", rt.builtinCurrentPath),
			"current_sort":   starlark.NewBuiltin("nmf.current_sort", rt.builtinCurrentSort),
			"display":        starlark.NewBuiltin("nmf.display", rt.builtinDisplay),
			"debug":          starlark.NewBuiltin("nmf.debug", rt.builtinDebug),
			"getenv":         starlark.NewBuiltin("nmf.getenv", rt.builtinGetenv),
			"os":             starlark.NewBuiltin("nmf.os", rt.builtinOS),
			"hostname":       starlark.NewBuiltin("nmf.hostname", rt.builtinHostname),
		}),
	}
}

func (rt *Runtime) builtinWindow(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var widthValue starlark.Value
	var heightValue starlark.Value
	var xValue starlark.Value
	var yValue starlark.Value
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"width?", &widthValue,
		"height?", &heightValue,
		"x?", &xValue,
		"y?", &yValue,
	); err != nil {
		return nil, err
	}
	width, err := optionalStarlarkInt("width", widthValue, rt.cfg.Window.Width)
	if err != nil {
		return nil, err
	}
	height, err := optionalStarlarkInt("height", heightValue, rt.cfg.Window.Height)
	if err != nil {
		return nil, err
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("window width and height must be positive")
	}
	rt.cfg.Window.Width = width
	rt.cfg.Window.Height = height
	if xValue != nil || yValue != nil {
		if xValue == nil || yValue == nil {
			return nil, fmt.Errorf("window x and y must be set together")
		}
		x, err := optionalStarlarkInt("x", xValue, 0)
		if err != nil {
			return nil, err
		}
		y, err := optionalStarlarkInt("y", yValue, 0)
		if err != nil {
			return nil, err
		}
		rt.cfg.Window.X = &x
		rt.cfg.Window.Y = &y
	}
	return starlark.None, nil
}

func (rt *Runtime) builtinStartup(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	directory := rt.cfg.Startup.Directory
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "directory?", &directory); err != nil {
		return nil, err
	}
	rt.cfg.Startup.Directory = strings.TrimSpace(directory)
	return starlark.None, nil
}

func optionalStarlarkInt(name string, value starlark.Value, fallback int) (int, error) {
	if value == nil {
		return fallback, nil
	}
	i, err := starlark.AsInt32(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an int", name)
	}
	return int(i), nil
}

func (rt *Runtime) builtinTheme(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	dark := rt.cfg.Theme.Dark
	fontSize := rt.cfg.Theme.FontSize
	fontName := rt.cfg.Theme.FontName
	fontPath := rt.cfg.Theme.FontPath
	monospaceFontName := rt.cfg.Theme.MonospaceFontName
	monospaceFontPath := rt.cfg.Theme.MonospaceFontPath
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"dark?", &dark,
		"font_size?", &fontSize,
		"font_name?", &fontName,
		"font_path?", &fontPath,
		"monospace_font_name?", &monospaceFontName,
		"monospace_font_path?", &monospaceFontPath,
	); err != nil {
		return nil, err
	}
	if fontSize < 0 {
		return nil, fmt.Errorf("font_size must be zero or positive")
	}
	rt.cfg.Theme.Dark = dark
	rt.cfg.Theme.FontSize = fontSize
	rt.cfg.Theme.FontName = strings.TrimSpace(fontName)
	rt.cfg.Theme.FontPath = fontPath
	rt.cfg.Theme.MonospaceFontName = strings.TrimSpace(monospaceFontName)
	rt.cfg.Theme.MonospaceFontPath = monospaceFontPath
	return starlark.None, nil
}

func (rt *Runtime) builtinColor(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var name string
	var value, dark, light starlark.Value
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"name", &name,
		"value?", &value,
		"dark?", &dark,
		"light?", &light,
	); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("color name must be a non-empty string")
	}
	if !customtheme.IsAppColorName(name) {
		return nil, fmt.Errorf("unknown app color name: %s", name)
	}

	updates := map[string]starlark.Value{}
	if value != nil {
		updates["value"] = value
	}
	if dark != nil {
		updates["dark"] = dark
	}
	if light != nil {
		updates["light"] = light
	}

	if len(updates) == 0 {
		return starlark.None, nil
	}
	if rt.cfg.Theme.Colors == nil {
		rt.cfg.Theme.Colors = make(map[string]config.ThemeColorConfig)
	}
	colorConfig := rt.cfg.Theme.Colors[name]
	for key, update := range updates {
		parsed, err := starlarkColorValue(update)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", fn.Name(), key, err)
		}
		switch key {
		case "value":
			colorConfig.Value = parsed
		case "dark":
			colorConfig.Dark = parsed
			colorConfig.DarkDefault = parsed == nil
		case "light":
			colorConfig.Light = parsed
			colorConfig.LightDefault = parsed == nil
		}
	}
	rt.cfg.Theme.Colors[name] = colorConfig
	return starlark.None, nil
}

func (rt *Runtime) builtinDarkTheme(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.Bool(rt.cfg.Theme.Dark), nil
}

func (rt *Runtime) builtinDebugLogging(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	enabled := rt.cfg.Debug.Enabled
	logDirectory := rt.cfg.Debug.LogDirectory
	maxFiles := rt.cfg.Debug.MaxLogFiles
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"enabled?", &enabled,
		"log_directory?", &logDirectory,
		"max_files?", &maxFiles,
	); err != nil {
		return nil, err
	}
	if maxFiles <= 0 {
		return nil, fmt.Errorf("max_files must be positive")
	}
	rt.cfg.Debug.Enabled = enabled
	rt.cfg.Debug.LogDirectory = strings.TrimSpace(logDirectory)
	rt.cfg.Debug.MaxLogFiles = maxFiles
	if rt.debugHook != nil {
		if err := rt.debugHook(rt.cfg.Debug); err != nil {
			return nil, err
		}
	}
	return starlark.None, nil
}

func (rt *Runtime) builtinUI(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	showHiddenFiles := rt.cfg.UI.ShowHiddenFiles
	itemSpacing := rt.cfg.UI.ItemSpacing
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"show_hidden_files?", &showHiddenFiles,
		"item_spacing?", &itemSpacing,
	); err != nil {
		return nil, err
	}
	if itemSpacing < 0 {
		return nil, fmt.Errorf("item_spacing must be zero or positive")
	}
	rt.cfg.UI.ShowHiddenFiles = showHiddenFiles
	rt.cfg.UI.ItemSpacing = itemSpacing
	return starlark.None, nil
}

func (rt *Runtime) builtinCopy(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	preserveTimestamps := rt.cfg.UI.Copy.PreserveTimestamps
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "preserve_timestamps?", &preserveTimestamps); err != nil {
		return nil, err
	}
	rt.cfg.UI.Copy.PreserveTimestamps = preserveTimestamps
	return starlark.None, nil
}

func (rt *Runtime) builtinViewer(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	maxWidth := rt.cfg.UI.Viewer.MaxWidth
	maxHeight := rt.cfg.UI.Viewer.MaxHeight
	defaultPane := rt.cfg.UI.Viewer.DefaultPane
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "max_width?", &maxWidth, "max_height?", &maxHeight, "default_pane?", &defaultPane); err != nil {
		return nil, err
	}
	if maxWidth < 0 || maxHeight < 0 {
		return nil, fmt.Errorf("viewer max_width and max_height must be zero or positive")
	}
	if defaultPane = config.NormalizeViewerDefaultPane(defaultPane); defaultPane == "" {
		return nil, fmt.Errorf("viewer default_pane must be one of auto, text, markdown, or hex")
	}
	rt.cfg.UI.Viewer.MaxWidth = maxWidth
	rt.cfg.UI.Viewer.MaxHeight = maxHeight
	rt.cfg.UI.Viewer.DefaultPane = defaultPane
	return starlark.None, nil
}

func (rt *Runtime) builtinArchive(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	zipNameEncoding := rt.cfg.UI.Archive.ZipNameEncoding
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "zip_name_encoding?", &zipNameEncoding); err != nil {
		return nil, err
	}
	if _, err := fileinfo.ResolveArchiveZipNameEncoding(zipNameEncoding); err != nil {
		return nil, err
	}
	rt.cfg.UI.Archive.ZipNameEncoding = strings.TrimSpace(zipNameEncoding)
	return starlark.None, nil
}

func (rt *Runtime) builtinSort(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	sortBy := rt.cfg.UI.Sort.SortBy
	sortOrder := rt.cfg.UI.Sort.SortOrder
	directoriesFirst := rt.cfg.UI.Sort.DirectoriesFirst
	temporary := false
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"by?", &sortBy,
		"order?", &sortOrder,
		"directories_first?", &directoriesFirst,
		"temporary?", &temporary,
	); err != nil {
		return nil, err
	}
	sortConfig, err := validateSortConfig(sortBy, sortOrder, directoriesFirst)
	if err != nil {
		return nil, err
	}
	if temporary {
		ctx, err := commandContext(thread, fn.Name())
		if err != nil {
			return nil, err
		}
		if ctx.FileManager == nil {
			return nil, fmt.Errorf("%s with temporary=True requires a file manager", fn.Name())
		}
		ctx.FileManager.ApplyTemporarySort(sortConfig)
		return starlark.None, nil
	}
	if thread != nil && thread.Local(commandContextKey) != nil {
		return nil, fmt.Errorf("%s without temporary=True cannot be used while a custom command is running; use temporary=True", fn.Name())
	}
	rt.cfg.UI.Sort = sortConfig
	return starlark.None, nil
}

func (rt *Runtime) builtinCursorStyle(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	styleType := rt.cfg.UI.CursorStyle.Type
	thickness := rt.cfg.UI.CursorStyle.Thickness
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "type?", &styleType, "thickness?", &thickness); err != nil {
		return nil, err
	}
	if !config.IsValidCursorStyleType(styleType) {
		return nil, fmt.Errorf("cursor style type must be underline, border, background, icon, or font")
	}
	if thickness < 0 {
		return nil, fmt.Errorf("cursor thickness must be zero or positive")
	}
	rt.cfg.UI.CursorStyle.Type = styleType
	rt.cfg.UI.CursorStyle.Thickness = thickness
	return starlark.None, nil
}

func (rt *Runtime) builtinCursorMemory(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	maxEntries := rt.cfg.UI.CursorMemory.MaxEntries
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "max_entries?", &maxEntries); err != nil {
		return nil, err
	}
	if maxEntries <= 0 {
		return nil, fmt.Errorf("max_entries must be positive")
	}
	rt.cfg.UI.CursorMemory.MaxEntries = maxEntries
	return starlark.None, nil
}

func (rt *Runtime) builtinNavigationHistory(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	maxEntries := rt.cfg.UI.NavigationHistory.MaxEntries
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "max_entries?", &maxEntries); err != nil {
		return nil, err
	}
	if maxEntries <= 0 {
		return nil, fmt.Errorf("max_entries must be positive")
	}
	rt.cfg.UI.NavigationHistory.MaxEntries = maxEntries
	return starlark.None, nil
}

func (rt *Runtime) builtinFileFilter(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	maxEntries := rt.cfg.UI.FileFilter.MaxEntries
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "max_entries?", &maxEntries); err != nil {
		return nil, err
	}
	if maxEntries <= 0 {
		return nil, fmt.Errorf("max_entries must be positive")
	}
	rt.cfg.UI.FileFilter.MaxEntries = maxEntries
	return starlark.None, nil
}

func (rt *Runtime) builtinDirectoryJump(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var shortcut string
	var directory string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "shortcut", &shortcut, "directory", &directory); err != nil {
		return nil, err
	}
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("directory jump directory must not be empty")
	}
	rt.cfg.UI.DirectoryJumps.Entries = append(rt.cfg.UI.DirectoryJumps.Entries, config.DirectoryJumpEntry{
		Shortcut:  shortcut,
		Directory: directory,
	})
	return starlark.None, nil
}

func (rt *Runtime) builtinClearDirectoryJumps(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, "nmf.clear_directory_jumps"); err != nil {
		return nil, err
	}
	if err := starlark.UnpackArgs("nmf.clear_directory_jumps", args, kwargs); err != nil {
		return nil, err
	}
	rt.cfg.UI.DirectoryJumps.Entries = nil
	return starlark.None, nil
}

func (rt *Runtime) builtinKey(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	return rt.appendKeyBinding(fn.Name(), args, kwargs)
}

func (rt *Runtime) builtinUnkey(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	return rt.appendKeyBinding(fn.Name(), args, kwargs, keymanager.CommandNoop)
}

func (rt *Runtime) appendKeyBinding(fnName string, args starlark.Tuple, kwargs []starlark.Tuple, fixedCommand ...string) (starlark.Value, error) {
	var key string
	var command string
	var event string
	var target string
	var callable starlark.Callable
	var hasCallable bool
	if len(fixedCommand) > 0 {
		command = fixedCommand[0]
		if err := starlark.UnpackArgs(fnName, args, kwargs, "key", &key, "event?", &event, "target?", &target); err != nil {
			return nil, err
		}
	} else {
		commandValue := starlark.Value(starlark.None)
		fnValue := starlark.Value(starlark.None)
		if err := starlark.UnpackArgs(fnName, args, kwargs, "key", &key, "cmd?", &commandValue, "event?", &event, "fn?", &fnValue, "target?", &target); err != nil {
			return nil, err
		}
		var hasCommand bool
		var err error
		command, hasCommand, err = optionalString(commandValue, "cmd")
		if err != nil {
			return nil, err
		}
		callable, hasCallable, err = optionalCallable(fnValue, "fn")
		if err != nil {
			return nil, err
		}
		if hasCommand == hasCallable {
			return nil, fmt.Errorf("key binding must specify exactly one of cmd or fn")
		}
	}
	normalizedTarget, err := normalizeKeyBindingTarget(target)
	if err != nil {
		return nil, err
	}
	if hasCallable && normalizedTarget != keymanager.KeyBindingTargetMain {
		return nil, fmt.Errorf("fn key bindings are only supported for target main")
	}
	if hasCallable {
		command = rt.registerGeneratedKeyCommand(callable)
	}
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("key must not be empty")
	}
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("command must not be empty")
	}
	if event != "" {
		rt.warnDeprecated("key.event", "key binding event=%q is deprecated and ignored key=%s command=%s", event, key, command)
	}
	rt.cfg.UI.KeyBindings = append(rt.cfg.UI.KeyBindings, config.KeyBindingEntry{
		Target:  keyBindingTargetForConfig(normalizedTarget),
		Key:     key,
		Command: command,
	})
	return starlark.None, nil
}

func (rt *Runtime) registerGeneratedKeyCommand(callable starlark.Callable) string {
	rt.keyCommandSeq++
	id := commandPrefix + "__key." + strconv.Itoa(rt.keyCommandSeq)
	rt.Commands[id] = func(ctx keymanager.CommandContext) {
		if err := rt.callCommand(id, callable, ctx); err != nil {
			rt.debugPrint("ConfigScript: command failed id=%s err=%s", id, formatStarlarkError(err))
		}
	}
	return id
}

func (rt *Runtime) builtinClearKeys(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, "nmf.clear_keys"); err != nil {
		return nil, err
	}
	var target string
	if err := starlark.UnpackArgs("nmf.clear_keys", args, kwargs, "target?", &target); err != nil {
		return nil, err
	}
	normalizedTarget, err := normalizeKeyBindingTarget(target)
	if err != nil {
		return nil, err
	}
	rt.cfg.UI.KeyBindings = removeKeyBindingsForTarget(rt.cfg.UI.KeyBindings, normalizedTarget)
	return starlark.None, nil
}

func normalizeKeyBindingTarget(target string) (string, error) {
	normalized := keymanager.NormalizeKeyBindingTarget(target)
	switch normalized {
	case keymanager.KeyBindingTargetMain, keymanager.KeyBindingTargetLineEdit, keymanager.KeyBindingTargetFileViewer:
		return normalized, nil
	default:
		return "", fmt.Errorf("key binding target must be one of main, lineEdit, or fileViewer")
	}
}

func keyBindingTargetForConfig(target string) string {
	if target == keymanager.KeyBindingTargetMain {
		return ""
	}
	return target
}

func removeKeyBindingsForTarget(entries []config.KeyBindingEntry, target string) []config.KeyBindingEntry {
	var kept []config.KeyBindingEntry
	for _, entry := range entries {
		if keymanager.NormalizeKeyBindingTarget(entry.Target) == target {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

func (rt *Runtime) builtinExternalCommand(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var name string
	var key string
	var command string
	var cwd string
	extensionsValue := starlark.Value(starlark.None)
	argsValue := starlark.Value(starlark.None)
	edit := false
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"name", &name,
		"cmd", &command,
		"exts?", &extensionsValue,
		"args?", &argsValue,
		"cwd?", &cwd,
		"edit?", &edit,
		"key?", &key,
	); err != nil {
		return nil, err
	}
	extensions, err := stringList(extensionsValue, "exts")
	if err != nil {
		return nil, err
	}
	commandArgs, err := stringList(argsValue, "args")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("external command name must not be empty")
	}
	key, err = normalizeCommandMenuKey(key)
	if err != nil {
		return nil, err
	}
	if !edit && strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("external command cmd must not be empty")
	}
	rt.cfg.UI.ExternalCommands = append(rt.cfg.UI.ExternalCommands, config.ExternalCommandEntry{
		Name:       name,
		Key:        key,
		Extensions: extensions,
		Command:    command,
		Args:       commandArgs,
		Cwd:        cwd,
		Edit:       edit,
	})
	return starlark.None, nil
}

func (rt *Runtime) builtinClearExternalCommands(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, "nmf.clear_external_commands"); err != nil {
		return nil, err
	}
	if err := starlark.UnpackArgs("nmf.clear_external_commands", args, kwargs); err != nil {
		return nil, err
	}
	rt.cfg.UI.ExternalCommands = nil
	return starlark.None, nil
}

func (rt *Runtime) builtinCommand(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var id string
	var value starlark.Value
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "id", &id, "fn", &value); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, commandPrefix) {
		return nil, fmt.Errorf("custom command id must start with %q", commandPrefix)
	}
	callable, ok := value.(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("command fn must be callable, got %s", value.Type())
	}
	rt.Commands[id] = func(ctx keymanager.CommandContext) {
		if err := rt.callCommand(id, callable, ctx); err != nil {
			rt.debugPrint("ConfigScript: command failed id=%s err=%s", id, formatStarlarkError(err))
		}
	}
	return starlark.None, nil
}

func (rt *Runtime) builtinMenu(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var name string
	var title string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &name, "title?", &title); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	title = strings.TrimSpace(title)
	if name == "" {
		return nil, fmt.Errorf("menu name must not be empty")
	}
	if title == "" {
		title = name
	}
	menu := rt.ensureMenu(name)
	menu.Title = title
	return starlark.None, nil
}

func (rt *Runtime) builtinMenuItem(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var menuName string
	var label string
	var key string
	commandValue := starlark.Value(starlark.None)
	fnValue := starlark.Value(starlark.None)
	if err := starlark.UnpackArgs(
		fn.Name(),
		args,
		kwargs,
		"menu", &menuName,
		"label", &label,
		"cmd?", &commandValue,
		"fn?", &fnValue,
		"key?", &key,
	); err != nil {
		return nil, err
	}
	menuName = strings.TrimSpace(menuName)
	label = strings.TrimSpace(label)
	if menuName == "" {
		return nil, fmt.Errorf("menu item menu must not be empty")
	}
	if label == "" {
		return nil, fmt.Errorf("menu item label must not be empty")
	}
	if key != "" {
		normalizedKey, err := normalizeMenuItemKey(key)
		if err != nil {
			return nil, err
		}
		key = normalizedKey
	}

	command, hasCommand, err := optionalString(commandValue, "cmd")
	if err != nil {
		return nil, err
	}
	callable, hasCallable, err := optionalCallable(fnValue, "fn")
	if err != nil {
		return nil, err
	}
	if hasCommand == hasCallable {
		return nil, fmt.Errorf("menu item must specify exactly one of cmd or fn")
	}

	menu := rt.ensureMenu(menuName)
	menu.Items = append(menu.Items, MenuItem{
		Label:    label,
		Key:      key,
		Command:  command,
		Callable: callable,
	})
	return starlark.None, nil
}

func (rt *Runtime) builtinMenuSeparator(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var menuName string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "menu", &menuName); err != nil {
		return nil, err
	}
	menuName = strings.TrimSpace(menuName)
	if menuName == "" {
		return nil, fmt.Errorf("menu separator menu must not be empty")
	}

	menu := rt.ensureMenu(menuName)
	menu.Items = append(menu.Items, MenuItem{Separator: true})
	return starlark.None, nil
}

func (rt *Runtime) builtinClearMenu(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := rejectCommandContext(thread, fn.Name()); err != nil {
		return nil, err
	}
	var name string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("menu name must not be empty")
	}
	menu := rt.ensureMenu(name)
	menu.Items = nil
	return starlark.None, nil
}

func (rt *Runtime) builtinShowMenu(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &name); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("menu name must not be empty")
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.FileManager == nil {
		return nil, fmt.Errorf("%s requires a file manager", fn.Name())
	}

	menu, ok := rt.Menus[name]
	if !ok || len(menu.Items) == 0 {
		show := func() {
			if ctx.ShowCommandMenu != nil {
				ctx.ShowCommandMenu(name, []keymanager.CommandMenuItem{{
					Label:  fmt.Sprintf("Menu %q has no items.", name),
					Action: func() {},
				}})
			}
		}
		deferCommandTransition(ctx, "starlark.show_menu", show)
		return starlark.None, nil
	}

	items := make([]keymanager.CommandMenuItem, 0, len(menu.Items))
	for _, item := range menu.Items {
		entry := item
		if entry.Separator {
			items = append(items, keymanager.CommandMenuItem{Separator: true})
			continue
		}
		items = append(items, keymanager.CommandMenuItem{
			Label: entry.Label,
			Key:   entry.Key,
			Action: func() {
				if entry.Command != "" {
					if ctx.RunCommand != nil {
						ctx.RunCommand(entry.Command)
					}
					return
				}
				if err := rt.callCommand("menu."+name+"."+entry.Label, entry.Callable, ctx); err != nil {
					rt.debugPrint("ConfigScript: menu item failed menu=%s label=%s err=%s", name, entry.Label, formatStarlarkError(err))
				}
			},
		})
	}
	show := func() {
		if ctx.ShowCommandMenu != nil {
			ctx.ShowCommandMenu(menu.Title, items)
		}
	}
	deferCommandTransition(ctx, "starlark.show_menu", show)
	return starlark.None, nil
}

func normalizeMenuItemKey(key string) (string, error) {
	return normalizeCommandMenuKey(key)
}

func normalizeCommandMenuKey(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	runes := []rune(key)
	if len(runes) != 1 {
		return "", fmt.Errorf("command menu key must be a single printable character")
	}
	if unicode.IsSpace(runes[0]) || !unicode.IsPrint(runes[0]) {
		return "", fmt.Errorf("command menu key must be a non-space printable character")
	}
	return string(runes[0]), nil
}

// renameDeprecatedCommandKwarg rewrites a deprecated command= kwarg to cmd=,
// warning once per function. It errors if both command= and cmd= are given.
func (rt *Runtime) renameDeprecatedCommandKwarg(fn *starlark.Builtin, kwargs []starlark.Tuple) ([]starlark.Tuple, error) {
	idx := -1
	hasCmd := false
	for i, kw := range kwargs {
		key, ok := starlark.AsString(kw.Index(0))
		if !ok {
			continue
		}
		switch key {
		case "command":
			idx = i
		case "cmd":
			hasCmd = true
		}
	}
	if idx < 0 {
		return kwargs, nil
	}
	if hasCmd {
		return nil, fmt.Errorf("%s got both cmd and command; use cmd", fn.Name())
	}
	rt.warnDeprecated(fn.Name()+".command", "%s keyword command= is deprecated; use cmd=", fn.Name())
	rewritten := append([]starlark.Tuple(nil), kwargs...)
	rewritten[idx] = starlark.Tuple{starlark.String("cmd"), kwargs[idx].Index(1)}
	return rewritten, nil
}

func (rt *Runtime) builtinRun(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	kwargs, err := rt.renameDeprecatedCommandKwarg(fn, kwargs)
	if err != nil {
		return nil, err
	}
	var command string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &command); err != nil {
		return nil, err
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.RunCommand == nil {
		return starlark.False, nil
	}
	return starlark.Bool(ctx.RunCommand(command)), nil
}

func (rt *Runtime) builtinMessage(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var message string
	title := "Message"
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "message", &message, "title?", &title); err != nil {
		return nil, err
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.FileManager == nil {
		return starlark.False, nil
	}
	show := func() {
		if ctx.ShowMessageDialog != nil {
			ctx.ShowMessageDialog(title, message)
		}
	}
	deferCommandTransition(ctx, "starlark.message", show)
	return starlark.True, nil
}

func (rt *Runtime) builtinExec(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	kwargs, err := rt.renameDeprecatedCommandKwarg(fn, kwargs)
	if err != nil {
		return nil, err
	}
	var command string
	var cwd string
	argsValue := starlark.Value(starlark.None)
	edit := false
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &command, "args?", &argsValue, "edit?", &edit, "cwd?", &cwd); err != nil {
		return nil, err
	}
	commandArgs, err := stringList(argsValue, "args")
	if err != nil {
		return nil, err
	}
	if !edit && strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("exec command must not be empty")
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.RunExternalCommand == nil {
		return starlark.False, nil
	}
	if edit && ctx.DeferTransition != nil {
		ctx.DeferTransition("starlark.exec.edit", func() {
			ctx.RunExternalCommand(command, commandArgs, edit, cwd)
		})
		return starlark.True, nil
	}
	return starlark.Bool(ctx.RunExternalCommand(command, commandArgs, edit, cwd)), nil
}

func (rt *Runtime) builtinMkdir(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	edit := false
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name?", &name, "edit?", &edit); err != nil {
		return nil, err
	}
	if !edit && strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("mkdir name must not be empty")
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.FileManager == nil {
		return starlark.False, nil
	}
	if edit {
		show := func() {
			if ctx.ShowCreateDirectoryDialog != nil {
				ctx.ShowCreateDirectoryDialog()
			}
		}
		deferCommandTransition(ctx, "starlark.mkdir.edit", show)
		return starlark.True, nil
	}
	return starlark.Bool(ctx.FileManager.CreateDirectory(name)), nil
}

func (rt *Runtime) builtinClipboard(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	rt.warnDeprecated("clipboard", "nmf.clipboard is deprecated; use nmf.set_clipboard")
	return rt.builtinSetClipboard(thread, fn, args, kwargs)
}

func (rt *Runtime) builtinSetClipboard(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "text", &text); err != nil {
		return nil, err
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.SetClipboard == nil {
		return starlark.False, nil
	}
	return starlark.Bool(ctx.SetClipboard(text)), nil
}

func (rt *Runtime) builtinSaveClipboard(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	edit := false
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name?", &name, "edit?", &edit); err != nil {
		return nil, err
	}
	if !edit && strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("save_clipboard name must not be empty")
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.FileManager == nil {
		return starlark.False, nil
	}
	if edit {
		show := func() {
			if ctx.ShowClipboardTextFileDialog != nil {
				ctx.ShowClipboardTextFileDialog()
			}
		}
		deferCommandTransition(ctx, "starlark.save_clipboard.edit", show)
		return starlark.True, nil
	}
	return starlark.Bool(ctx.FileManager.CreateClipboardTextFile(name)), nil
}

func deferCommandTransition(ctx keymanager.CommandContext, label string, action func()) {
	if ctx.DeferTransition != nil {
		ctx.DeferTransition(label, action)
		return
	}
	action()
}

func (rt *Runtime) builtinLoadDirectory(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path string
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "path", &path); err != nil {
		return nil, err
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.FileManager == nil {
		return nil, fmt.Errorf("%s requires a file manager", fn.Name())
	}
	ctx.FileManager.LoadDirectory(path)
	return starlark.None, nil
}

func (rt *Runtime) builtinCurrentPath(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.FileManager == nil {
		return nil, fmt.Errorf("%s requires a file manager", fn.Name())
	}
	return starlark.String(ctx.FileManager.GetCurrentPath()), nil
}

func (rt *Runtime) builtinCurrentSort(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	ctx, err := commandContext(thread, fn.Name())
	if err != nil {
		return nil, err
	}
	if ctx.FileManager == nil {
		return nil, fmt.Errorf("%s requires a file manager", fn.Name())
	}
	return sortConfigValue(ctx.FileManager.CurrentSort()), nil
}

func (rt *Runtime) builtinDisplay(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return displayInfoValue(rt.display), nil
}

func (rt *Runtime) builtinDebug(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(kwargs) > 0 {
		return nil, fmt.Errorf("%s does not accept keyword arguments", fn.Name())
	}

	parts := make([]string, args.Len())
	for i := 0; i < args.Len(); i++ {
		parts[i] = debugValueString(args.Index(i))
	}
	rt.debugPrint("ConfigScript: %s", strings.Join(parts, " "))
	return starlark.None, nil
}

func (rt *Runtime) builtinGetenv(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	defaultValue := starlark.Value(starlark.None)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "name", &name, "default?", &defaultValue); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("environment variable name must not be empty")
	}
	if value, ok := os.LookupEnv(name); ok {
		return starlark.String(value), nil
	}
	if defaultValue == starlark.None {
		return starlark.None, nil
	}
	defaultString, ok := starlark.AsString(defaultValue)
	if !ok {
		return nil, fmt.Errorf("default must be a string or None, got %s", defaultValue.Type())
	}
	return starlark.String(defaultString), nil
}

func (rt *Runtime) builtinOS(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return starlark.String(runtime.GOOS), nil
}

func (rt *Runtime) builtinHostname(_ *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs); err != nil {
		return nil, err
	}
	name, err := os.Hostname()
	if err != nil {
		rt.debugPrint("ConfigScript: hostname err=%s", err)
		return starlark.String(""), nil
	}
	return starlark.String(name), nil
}

func (rt *Runtime) callCommand(id string, callable starlark.Callable, ctx keymanager.CommandContext) error {
	thread := rt.newThread("nmf command " + id)
	thread.SetLocal(commandContextKey, ctx)
	_, err := starlark.Call(thread, callable, starlark.Tuple{commandContextValue(ctx)}, nil)
	return err
}

func commandContext(thread *starlark.Thread, fnName string) (keymanager.CommandContext, error) {
	value := thread.Local(commandContextKey)
	ctx, ok := value.(keymanager.CommandContext)
	if !ok {
		return keymanager.CommandContext{}, fmt.Errorf("%s can only be used while a custom command is running", fnName)
	}
	return ctx, nil
}

func rejectCommandContext(thread *starlark.Thread, fnName string) error {
	if thread != nil && thread.Local(commandContextKey) != nil {
		return fmt.Errorf("%s cannot be used while a custom command is running", fnName)
	}
	return nil
}

func commandContextValue(ctx keymanager.CommandContext) starlark.Value {
	fields := starlark.StringDict{
		"key":   starlark.String(ctx.Key),
		"event": starlark.String(ctx.Event),
		"shift": starlark.Bool(ctx.Modifiers.ShiftPressed),
		"ctrl":  starlark.Bool(ctx.Modifiers.CtrlPressed),
		"alt":   starlark.Bool(ctx.Modifiers.AltPressed),
	}
	if ctx.FileManager != nil {
		dir, file, name, paths := commandContextTargets(ctx.FileManager)
		fields["current_path"] = starlark.String(dir)
		fields["current_file"] = starlark.String(file)
		fields["current_name"] = starlark.String(name)

		values := make([]starlark.Value, len(paths))
		for i, path := range paths {
			values[i] = starlark.String(path)
		}
		fields["selected_files"] = starlark.NewList(values)

		allSelectedFiles := commandContextAllSelectedFiles(ctx.FileManager)
		allSelectedValues := make([]starlark.Value, len(allSelectedFiles))
		for i, path := range allSelectedFiles {
			allSelectedValues[i] = starlark.String(path)
		}
		fields["all_selected_files"] = starlark.NewList(allSelectedValues)
	}
	return starlarkstruct.FromStringDict(starlark.String("ctx"), fields)
}

func sortConfigValue(sortConfig config.SortConfig) starlark.Value {
	return starlarkstruct.FromStringDict(starlark.String("sort"), starlark.StringDict{
		"by":                starlark.String(sortConfig.SortBy),
		"order":             starlark.String(sortConfig.SortOrder),
		"directories_first": starlark.Bool(sortConfig.DirectoriesFirst),
	})
}

func displayInfoValue(info display.Info) starlark.Value {
	return starlarkstruct.FromStringDict(starlark.String("display"), starlark.StringDict{
		"available":     starlark.Bool(info.Available),
		"name":          starlark.String(info.Name),
		"width":         starlark.MakeInt(info.Width),
		"height":        starlark.MakeInt(info.Height),
		"work_width":    starlark.MakeInt(info.WorkWidth),
		"work_height":   starlark.MakeInt(info.WorkHeight),
		"pixel_width":   starlark.MakeInt(info.PixelWidth),
		"pixel_height":  starlark.MakeInt(info.PixelHeight),
		"scale":         starlark.Float(info.Scale),
		"display_scale": starlark.Float(info.DisplayScale),
		"user_scale":    starlark.Float(info.UserScale),
	})
}

func debugValueString(value starlark.Value) string {
	if s, ok := starlark.AsString(value); ok {
		return s
	}
	return value.String()
}

func (rt *Runtime) ensureMenu(name string) *Menu {
	if rt.Menus == nil {
		rt.Menus = make(map[string]*Menu)
	}
	menu, ok := rt.Menus[name]
	if !ok {
		menu = &Menu{Name: name, Title: name}
		rt.Menus[name] = menu
	}
	return menu
}

func commandContextTargets(fm keymanager.FileManagerInterface) (dir string, file string, name string, files []string) {
	dir = fileinfo.CommandArgumentPath(fm.GetCurrentPath())
	targets := commandContextTargetPaths(fm)
	files = make([]string, len(targets))
	for i, target := range targets {
		files[i] = fileinfo.CommandArgumentPath(target.Path)
	}
	if len(targets) > 0 {
		file = files[0]
		name = fileinfo.BaseName(targets[0].Path)
	}
	return dir, file, name, files
}

func commandContextTargetPaths(fm keymanager.FileManagerInterface) []fileinfo.FileInfo {
	files := fm.GetFiles()
	selected := fm.GetSelectedFiles()
	targets := make([]fileinfo.FileInfo, 0, len(selected))
	for _, fi := range files {
		if !selected[fi.Path] || !isTargetFileInfo(fi) {
			continue
		}
		targets = append(targets, fi)
	}
	if len(targets) > 0 {
		return targets
	}

	idx := fm.GetCurrentCursorIndex()
	if idx >= 0 && idx < len(files) && isTargetFileInfo(files[idx]) {
		return []fileinfo.FileInfo{files[idx]}
	}
	return nil
}

func commandContextAllSelectedFiles(fm keymanager.FileManagerInterface) []string {
	files := fm.GetAllSelectedFiles()
	paths := make([]string, 0, len(files))
	for _, fi := range files {
		if !isTargetFileInfo(fi) {
			continue
		}
		paths = append(paths, fileinfo.CommandArgumentPath(fi.Path))
	}
	return paths
}

func isTargetFileInfo(fi fileinfo.FileInfo) bool {
	return fi.Name != ".." && fi.Status != fileinfo.StatusDeleted
}

func stringList(value starlark.Value, name string) ([]string, error) {
	if value == nil || value == starlark.None {
		return nil, nil
	}
	iterable := starlark.Iterate(value)
	if iterable == nil {
		return nil, fmt.Errorf("%s must be a list or tuple of strings", name)
	}
	defer iterable.Done()

	var result []string
	var item starlark.Value
	for iterable.Next(&item) {
		s, ok := starlark.AsString(item)
		if !ok {
			return nil, fmt.Errorf("%s must contain only strings, got %s", name, item.Type())
		}
		result = append(result, s)
	}
	return result, nil
}

func starlarkColorValue(value starlark.Value) (*config.ThemeColorValue, error) {
	if value == nil || value == starlark.None {
		return nil, nil
	}
	if name, ok := starlark.AsString(value); ok {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("color name must not be empty")
		}
		return &config.ThemeColorValue{Name: name}, nil
	}

	iterable := starlark.Iterate(value)
	if iterable == nil {
		return nil, fmt.Errorf("color must be a name, RGBA list, RGBA tuple, or None")
	}
	defer iterable.Done()

	var rgba [4]uint8
	var item starlark.Value
	i := 0
	for iterable.Next(&item) {
		if i >= len(rgba) {
			return nil, fmt.Errorf("RGBA color must have exactly 4 values")
		}
		n, err := starlark.AsInt32(item)
		if err != nil || n < 0 || n > 255 {
			return nil, fmt.Errorf("RGBA values must be integers from 0 to 255")
		}
		rgba[i] = uint8(n)
		i++
	}
	if i != len(rgba) {
		return nil, fmt.Errorf("RGBA color must have exactly 4 values")
	}
	return &config.ThemeColorValue{RGBA: rgba, IsRGBA: true}, nil
}

func optionalString(value starlark.Value, name string) (string, bool, error) {
	if value == nil || value == starlark.None {
		return "", false, nil
	}
	result, ok := starlark.AsString(value)
	if !ok {
		return "", false, fmt.Errorf("%s must be a string or None, got %s", name, value.Type())
	}
	result = strings.TrimSpace(result)
	if result == "" {
		return "", false, fmt.Errorf("%s must not be empty", name)
	}
	return result, true, nil
}

func optionalCallable(value starlark.Value, name string) (starlark.Callable, bool, error) {
	if value == nil || value == starlark.None {
		return nil, false, nil
	}
	callable, ok := value.(starlark.Callable)
	if !ok {
		return nil, false, fmt.Errorf("%s must be callable or None, got %s", name, value.Type())
	}
	return callable, true, nil
}

func formatStarlarkError(err error) string {
	var evalErr *starlark.EvalError
	if errors.As(err, &evalErr) {
		return evalErr.Backtrace()
	}
	return err.Error()
}

func validateSortConfig(sortBy string, sortOrder string, directoriesFirst bool) (config.SortConfig, error) {
	if !config.IsValidSortBy(sortBy) {
		return config.SortConfig{}, fmt.Errorf("sort by must be one of name, size, modified, or extension")
	}
	if !config.IsValidSortOrder(sortOrder) {
		return config.SortConfig{}, fmt.Errorf("sort order must be asc or desc")
	}
	return config.SortConfig{
		SortBy:           sortBy,
		SortOrder:        sortOrder,
		DirectoriesFirst: directoriesFirst,
	}, nil
}
