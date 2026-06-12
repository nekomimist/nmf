# Starlark Configuration

> [!NOTE]
> Starlark configuration support is currently experimental.
> Its API and behavior may change as the feature evolves, and future releases
> may include breaking changes.

NMF can load an optional Starlark initialization file after `config.json`.
This gives users a programmable configuration layer while preserving the
existing JSON file and runtime state persistence.

## Load Order

Startup loads configuration in this order:

1. Built-in defaults.
2. `config.json` from the OS-specific config directory.
3. `init.star` from the same directory, if present.

The Starlark file is an overlay. Values set by `init.star` affect the running
app, but they are not written back into `config.json` by routine runtime saves.
Runtime state such as cursor memory and navigation history is still saved.

Paths:

- Linux/Unix: `$XDG_CONFIG_HOME/nekomimist/nmf/init.star`, or
  `~/.config/nekomimist/nmf/init.star`
- macOS: `~/Library/Application Support/nekomimist/nmf/init.star`
- Windows: `%APPDATA%\nekomimist\nmf\init.star`

If `init.star` does not exist, startup behaves exactly like JSON-only
configuration.

## Example

```python
nmf.window(width = 1000, height = 720, x = 100, y = 80)
nmf.startup(directory = "~/projects")

nmf.theme(
    dark = True,
    font_size = 14,
    font_name = nmf.getenv("NMF_FONT", "Noto Sans CJK JP"),
    monospace_font_name = nmf.getenv("NMF_MONO_FONT", ""),
)

display = nmf.display()
if display.available and display.work_height >= 1300:
    nmf.debug("large display", display.work_width, display.work_height)
    nmf.window(width = int(display.work_width * 0.65), height = int(display.work_height * 0.75))
    nmf.theme(font_size = 18)

if nmf.dark_theme():
    nmf.color("cursor", value = "foreground")
else:
    nmf.color("cursor", value = [0, 0, 0, 255])

nmf.ui(show_hidden_files = True, item_spacing = 2)
nmf.debug_logging(enabled = True, log_directory = "logs", max_files = 10)
nmf.copy(preserve_timestamps = False)
nmf.archive(zip_name_encoding = "shift_jis")
nmf.sort(by = "extension", order = "asc", directories_first = True)
nmf.cursor_style(type = "border", thickness = 2)

nmf.clear_directory_jumps()
nmf.directory_jump("p", "~/projects")
nmf.directory_jump("d", "~/Downloads")

if nmf.os() == "windows":
    nmf.directory_jump("w", "C:/Work")

nmf.clear_external_commands()
nmf.external_command(
    name = "Open in Vim",
    key = "V",
    exts = ["go", "md"],
    cmd = "vim",
    args = ["{file}"],
    cwd = "{dir}",
    edit = True,
)

def open_parent_and_refresh(ctx):
    nmf.run("directory.parent")
    nmf.run("directory.refresh")

nmf.command("user.open_parent_and_refresh", open_parent_and_refresh)
nmf.key("C-P", "user.open_parent_and_refresh")

def edit_current(ctx):
    nmf.exec("vim", args = [ctx.current_file], cwd = ctx.current_path)

nmf.key("E", fn = edit_current)

def create_directory(ctx):
    nmf.mkdir(edit = True)

nmf.key("K", fn = create_directory)

def copy_selected(ctx):
    nmf.clipboard("\n".join(ctx.selected_files))

nmf.key("C-Y", fn = copy_selected)

def save_clipboard(ctx):
    nmf.save_clipboard(edit = True)

nmf.key("P", fn = save_clipboard)

def open_media(ctx):
    if ctx.current_name.endswith(".mp4"):
        nmf.exec("mpv", args = [ctx.current_file])
    elif ctx.current_name.endswith(".png"):
        nmf.exec("imv", args = [ctx.current_file])

nmf.command("user.open_media", open_media)

def compare_supported(ctx):
    files = [path for path in ctx.selected_files if path.endswith(".go")]
    if files:
        nmf.exec("meld", args = files)

nmf.command("user.compare_supported", compare_supported)

def toggle_name_modified(ctx):
    sort = nmf.current_sort()
    if sort.by == "modified":
        by = "name"
        order = "asc"
    else:
        by = "modified"
        order = "desc"
    nmf.sort(by = by, order = order, directories_first = sort.directories_first, temporary = True)

nmf.command("user.toggle_name_modified", toggle_name_modified)

nmf.menu("tools", title = "Tools")
nmf.menu_item("tools", "Refresh", cmd = "directory.refresh", key = "R")
nmf.menu_separator("tools")

def edit_from_menu(ctx):
    if ctx.current_file:
        nmf.exec("vim", args = [ctx.current_file])

nmf.menu_item("tools", "Edit in Vim", fn = edit_from_menu, key = "E")

def show_tools(ctx):
    nmf.show_menu("tools")

nmf.command("user.show_tools", show_tools)
nmf.key("T", "user.show_tools")
```

## Configuration API

Scalar sections:

- `nmf.window(width = int, height = int, x = int, y = int)`
- `nmf.startup(directory = str)`
- `nmf.theme(dark = bool, font_size = int, font_name = str, font_path = str,
  monospace_font_name = str, monospace_font_path = str)`
- `nmf.color(name, value = color|None, dark = color|None, light = color|None)`
- `nmf.debug_logging(enabled = bool, log_directory = str, max_files = int)`
- `nmf.ui(show_hidden_files = bool, item_spacing = int)`
- `nmf.copy(preserve_timestamps = bool)`
- `nmf.viewer(max_width = int, max_height = int)`
- `nmf.archive(zip_name_encoding = str)`
- `nmf.sort(by = "name|size|modified|extension", order = "asc|desc",
  directories_first = bool, temporary = bool)`
- `nmf.cursor_style(type = "underline|border|background|icon|font",
  thickness = int)`
- `nmf.cursor_memory(max_entries = int)`
- `nmf.navigation_history(max_entries = int)`
- `nmf.file_filter(max_entries = int)`

List sections:

- `nmf.directory_jump(shortcut, directory)`
- `nmf.clear_directory_jumps()`
- `nmf.key(key, cmd = None, fn = None)`
- `nmf.unkey(key)`
  (the legacy `event` argument is still accepted but deprecated and ignored)
- `nmf.clear_keys()`
- `nmf.external_command(name, cmd, exts = [], args = [], cwd = "", edit = False, key = "")`
- `nmf.clear_external_commands()`
- `nmf.menu(name, title = "")`
- `nmf.menu_item(menu, label, cmd = None, fn = None, key = "")`
- `nmf.menu_separator(menu)`
- `nmf.clear_menu(name)`

List APIs append to values already loaded from `config.json`. Use the matching
`clear_*` function when the Starlark file should own the whole list.
Directory jump shortcuts may be empty or multiple characters; the dialog filters
them by case-insensitive prefix.
`nmf.window(x = ..., y = ...)` configures the first window position on Windows;
the position is clamped into the nearest monitor work area when applied. Set
`x` and `y` together. Other platforms currently ignore the position fields.
`nmf.startup(directory = "...")` sets the fallback startup directory used only
when no command-line path is supplied.
`nmf.debug_logging(enabled = True, log_directory = "logs", max_files = 10)`
enables per-startup debug log files for the current run. Empty `log_directory`
uses a `logs` directory next to `config.json` and `init.star`; relative paths
are resolved from that config directory. The setting is treated as a Starlark
overlay and is not written back to `config.json` by routine saves.
`nmf.copy(preserve_timestamps = True)` sets the default state for the Copy
dialog checkbox. The checkbox choice applies only to the copy being queued and
is not written back to `config.json`.
`nmf.viewer(max_width = 1200, max_height = 900)` caps the built-in file viewer
dialog size. Use `0` for either value to leave that dimension uncapped.
`nmf.archive(zip_name_encoding = "...")` sets the fallback charset for ZIP entry
names that are not marked as UTF-8. Common values include `shift_jis` (default),
`cp437`, and `utf-8`.
`nmf.unkey` appends a binding to the built-in `noop` command, which disables a
default key binding with the same key for the current run.
Menu definitions are runtime-only and are not saved to `config.json`.
Menu item and external command `key` values are optional single printable
characters. While a command menu is open, typing the key runs the first visible
item with that key case-insensitively. Later duplicate keys stay visible but
behave as if they had no key.

Utility API:

- `nmf.dark_theme()` returns `True` when the current effective theme is dark.
- `nmf.getenv(name, default = None)` returns an environment variable value.
  If the variable is not set and no default is provided, it returns `None`.
- `nmf.display()` returns primary display information captured during startup.
  Use `work_width` and `work_height` for `nmf.window()` sizing because they
  describe the usable screen area in Fyne window coordinates after display DPI
  and `FYNE_SCALE` have been applied. `pixel_width` and `pixel_height` expose
  the physical pixel resolution for users that need it.
- `nmf.debug(value, ...)` writes values to NMF's debug log with the
  `ConfigScript:` prefix. It is visible only when debug logging is enabled,
  such as with `-d`, `-debug-log`, `config.json` debug settings, or an earlier
  `nmf.debug_logging(enabled = True)` call in `init.star`.
- `nmf.os()` returns the Go runtime OS name, such as `windows`, `linux`, or
  `darwin`.
- `nmf.hostname()` returns the current host name, or an empty string if it
  cannot be read.

`nmf.display()` returns a struct with these fields:

- `available`: `True` when display information was available.
- `name`: the primary display name, or an empty string.
- `width`, `height`: the primary display size in window coordinates.
- `work_width`, `work_height`: usable display size in window coordinates,
  excluding task bars or system-reserved areas when reported by the OS.
- `pixel_width`, `pixel_height`: physical display resolution in pixels.
- `scale`: effective Fyne scale used to convert between window coordinates and
  physical pixels. It includes display DPI and `FYNE_SCALE`.
- `display_scale`: display DPI scale before `FYNE_SCALE` is applied.
- `user_scale`: user scale from `FYNE_SCALE`, or `1.0` when unset, `auto`, or
  invalid.

On unsupported platforms or when display probing fails, `available` is `False`
and numeric fields are zero. Startup continues in that case.

Color API:

- `nmf.color()` customizes NMF-specific colors such as `fileRegular`,
  `fileDirectory`, `fileSymlink`, `fileHidden`, `statusAdded`,
  `statusDeleted`, `statusModified`, `selectionBackground`, `cursor`,
  `lineEditCursor`, `lineEditSelection`, `dialogListCursor`,
  `menuCursor`, `copyMoveOpenDestination`,
  `searchOverlayBackground`, `searchOverlayForeground`, and
  `busyOverlayBackground`.
- `lineEditCursor` and `lineEditSelection` apply only to one-line edit dialogs.
  `dialogListCursor` applies to Navigation History, Directory Jump, Filter,
  Copy/Move, and Jobs list cursor rows.
  `menuCursor` applies to command menu cursor rows.
  `copyMoveOpenDestination` applies to Copy/Move destination rows and
  Navigation History rows that are open in another File Manager window.
- A color can be an RGBA list or tuple like `[255, 255, 255, 255]`, a Fyne
  theme color name like `"foreground"` or `"selection"`, or a Fyne primary
  color name like `"blue"` or `"green"`.
- `value` applies to both dark and light themes. `dark` and `light` override
  only that variant.
- Passing `None` for `dark` or `light` keeps that variant on the built-in
  default, even when a common `value` is set.

## Custom Commands

Starlark can register functions as main-screen commands:

```python
def show_jobs(ctx):
    nmf.run("jobs.show")

nmf.command("user.show_jobs", show_jobs)
nmf.key("S-J", "user.show_jobs")
```

Custom command IDs must start with `user.` so they cannot override built-in
commands accidentally.
Use `nmf.command` when a function needs a stable reusable command ID for
`nmf.run`, JSON key bindings, or menu items. For Starlark-only key bindings,
`nmf.key(key, fn = callable)` can bind a function directly.

The command function receives one `ctx` struct:

- `ctx.key`: key name such as `X`, `F2`, or `Return`
- `ctx.event`: always `typed` (kept for compatibility; bindings fire on key activation)
- `ctx.shift`, `ctx.ctrl`, `ctx.alt`: modifier booleans
- `ctx.current_path`: current directory path in external-command argument form
- `ctx.current_file`: first target path in external-command argument form, or empty
- `ctx.current_name`: base name of the first target, or empty
- `ctx.selected_files`: target paths in external-command argument form. Selected
  files are used when present; otherwise the cursor item is used.
- `ctx.all_selected_files`: marked files from all open file manager windows in
  external-command argument form, without cursor fallback.

Command-only helpers:

- `nmf.run(command_id)` runs a built-in or `user.*` command and returns a bool.
- `nmf.message(message, title = "Message")` displays an OK dialog and returns
  a bool. It can only be used while a custom command is running. When launched
  from a key binding, the dialog is opened after the triggering keys are
  released.
- `nmf.clipboard(text)` writes a string to the system clipboard and returns a
  bool. Use `ctx.current_path`, `ctx.current_file`, `ctx.current_name`, and
  `ctx.selected_files` to build values for key bindings or menu items.
- `nmf.save_clipboard(name = "", edit = False)` creates a text file in the
  current directory from the system clipboard text and returns a bool. With
  `edit = False`, `name` is required and existing files are rejected. With
  `edit = True`, nmf opens the same one-line file name dialog as the built-in
  key binding and returns `False`.
- `nmf.exec(command, args = [], edit = False, cwd = "")` starts an external command and returns a bool.
  `args` is passed as raw strings; use `ctx.current_file`,
  `ctx.selected_files`, `ctx.current_path`, and `ctx.current_name` instead of
  `{file}` placeholders.
  `cwd` is an optional working directory. If it is empty, nmf preserves the
  existing process working directory behavior. If it points to a virtual path
  such as an archive or direct SMB provider, it is ignored.
  When `edit` is true, nmf opens the command line in a one-line edit dialog.
  When launched from a key binding, that dialog is opened on the next
  main-loop iteration so the triggering key's events do not leak into the
  editor.
  In that mode, `command` may be empty so the dialog can be used as a command
  prompt.
  The call returns false immediately because the edited command runs later when
  the dialog is accepted.
- `nmf.mkdir(name = "", edit = False)` creates a directory in the active
  directory and returns a bool. The name must be a single path segment. When
  `edit` is true, nmf opens a one-line edit dialog and returns false
  immediately because creation happens later when the dialog is accepted. When
  launched from a key binding, the dialog is opened after the triggering keys
  are released.
- `nmf.load_directory(path)` loads a directory path.
- `nmf.current_path()` returns the active directory path.
- `nmf.current_sort()` returns the active file-list sort as a struct with
  `by`, `order`, and `directories_first` fields.
- `nmf.sort(..., temporary = True)` re-sorts the active file list without
  changing configuration or saving to `config.json`. It can only be used while a
  custom command is running.
- `nmf.show_menu(name)` displays a Starlark-defined menu. It can only be used
  while a custom command is running. When launched from a key binding, the menu
  is shown after the triggering keys are released.

Available built-in command IDs are listed in `docs/configuration.md`.

## Safety Model

NMF embeds the official Go Starlark interpreter:

- `init.star` is executed with `go.starlark.net/starlark`.
- `ExecFileOptions` is used instead of legacy global options.
- Top-level `if` and `for` are allowed for configuration convenience.
- `while`, recursion, and global reassignment are not enabled.
- Each initialization or command call has a Starlark step limit.
- `load()` is restricted to files under the config directory. A module name
  without an extension gets `.star` appended.

Errors during `init.star` loading stop startup and include a Starlark backtrace.
Errors during a custom command are logged through debug logging and do not crash
the process.

## Persistence Model

`config.json` remains the persistence target for runtime state and existing JSON
settings. When `init.star` overlays a setting, NMF records that the field was
owned by Starlark for this run. Before `Manager.Save` or `Manager.SaveAsync`
writes a snapshot, the save transform restores Starlark-owned fields to the
pre-overlay JSON value.

This prevents normal runtime saves from converting a Starlark preference into
JSON. Runtime subfields that are not owned by Starlark, such as cursor memory
entries, continue to be persisted.

If a GUI setting changes a field that `init.star` also owns during the same run,
the Starlark overlay currently wins for persistence. Edit `init.star` for durable
changes to Starlark-owned fields.
