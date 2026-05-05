# Starlark Configuration

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
nmf.window(width = 1000, height = 720)

nmf.theme(
    dark = True,
    font_size = 14,
    font_name = nmf.getenv("NMF_FONT", "Noto Sans CJK JP"),
)

nmf.ui(show_hidden_files = True, item_spacing = 2)
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
    extensions = ["go", "md"],
    command = "vim",
    args = ["{file}"],
)

def open_parent_and_refresh(ctx):
    nmf.run("directory.parent")
    nmf.run("directory.refresh")

nmf.command("user.open_parent_and_refresh", open_parent_and_refresh)
nmf.key("C-P", "user.open_parent_and_refresh", event = "down")
```

## Configuration API

Scalar sections:

- `nmf.window(width = int, height = int)`
- `nmf.theme(dark = bool, font_size = int, font_name = str, font_path = str)`
- `nmf.ui(show_hidden_files = bool, item_spacing = int)`
- `nmf.sort(by = "name|size|modified|extension", order = "asc|desc",
  directories_first = bool)`
- `nmf.cursor_style(type = "underline|border|background|icon|font",
  thickness = int)`
- `nmf.cursor_memory(max_entries = int)`
- `nmf.navigation_history(max_entries = int)`
- `nmf.file_filter(max_entries = int)`

List sections:

- `nmf.directory_jump(shortcut, directory)`
- `nmf.clear_directory_jumps()`
- `nmf.key(key, command, event = "")`
- `nmf.clear_keys()`
- `nmf.external_command(name, command, extensions = [], args = [])`
- `nmf.clear_external_commands()`

List APIs append to values already loaded from `config.json`. Use the matching
`clear_*` function when the Starlark file should own the whole list.

Utility API:

- `nmf.getenv(name, default = None)` returns an environment variable value.
  If the variable is not set and no default is provided, it returns `None`.
- `nmf.os()` returns the Go runtime OS name, such as `windows`, `linux`, or
  `darwin`.
- `nmf.hostname()` returns the current host name, or an empty string if it
  cannot be read.

## Custom Commands

Starlark can register functions as main-screen commands:

```python
def show_jobs(ctx):
    nmf.run("jobs.show")

nmf.command("user.show_jobs", show_jobs)
nmf.key("S-J", "user.show_jobs", event = "typed")
```

Custom command IDs must start with `user.` so they cannot override built-in
commands accidentally.

The command function receives one `ctx` struct:

- `ctx.key`: key name such as `X`, `F2`, or `Return`
- `ctx.event`: `typed`, `down`, or `up`
- `ctx.shift`, `ctx.ctrl`, `ctx.alt`: modifier booleans
- `ctx.current_path`: current directory path
- `ctx.current_file`: current cursor item path, or empty
- `ctx.current_name`: current cursor item name, or empty
- `ctx.selected_files`: sorted list of selected paths

Command-only helpers:

- `nmf.run(command_id)` runs a built-in or `user.*` command and returns a bool.
- `nmf.load_directory(path)` loads a directory path.
- `nmf.current_path()` returns the active directory path.

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
