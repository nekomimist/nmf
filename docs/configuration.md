# Configuration

NMF loads `config.json` from the OS-specific app config directory:

- Linux/Unix: `$XDG_CONFIG_HOME/nekomimist/nmf/config.json`, or
  `~/.config/nekomimist/nmf/config.json`
- macOS: `~/Library/Application Support/nekomimist/nmf/config.json`
- Windows: `%APPDATA%\nekomimist\nmf\config.json`

The schema source of truth is `internal/config/config.go`. Missing fields use
defaults. `config.json` is read-only from the app's point of view: NMF never
writes to it. Frequently-changing runtime state (remembered cursor positions,
navigation history, file filter history, and the last-applied sort) instead
lives in a separate `state.json`; see [Runtime State](#runtime-state) below.

Unknown object fields and invalid bounded/enum values are startup errors rather
than silently ignored settings. This includes non-positive window sizes and
entry limits, negative spacing/scroll margins/viewer sizes/cursor thickness,
and unsupported sort, viewer-pane, or cursor-style values. Known legacy
runtime-state fields remain accepted until their one-time migration to
`state.json` completes.
Only a missing `config.json` selects defaults; permission and other read errors
are reported.

After `config.json` is loaded, NMF also loads an optional `init.star` from the
same directory. Starlark settings overlay JSON for the current run and can define
custom commands for key bindings. See `docs/starlark-configuration.md`.

## Example

```json
{
  "window": {
    "width": 1000,
    "height": 720,
    "x": 100,
    "y": 80
  },
  "startup": {
    "directory": "~/projects"
  },
  "theme": {
    "dark": true,
    "fontSize": 14,
    "fontName": "Noto Sans CJK JP",
    "fontPath": "",
    "monospaceFontName": "UDEV Gothic",
    "monospaceFontPath": "",
    "colors": {
      "cursor": {
        "dark": "foreground",
        "light": [0, 0, 0, 255]
      },
      "selectionBackground": "selection"
    }
  },
  "debug": {
    "enabled": false,
    "logDirectory": "",
    "maxLogFiles": 10
  },
  "ui": {
    "showHiddenFiles": false,
    "sort": {
      "sortBy": "name",
      "sortOrder": "asc",
      "directoriesFirst": true
    },
    "itemSpacing": 4,
    "scrollMargin": 3,
    "copy": {
      "preserveTimestamps": false
    },
    "viewer": {
      "maxWidth": 0,
      "maxHeight": 0,
      "defaultPane": "auto",
      "defaultWrap": false
    },
    "archive": {
      "zipNameEncoding": "shift_jis"
    },
    "ime": {
      "enabled": true
    },
    "cursorStyle": {
      "type": "underline",
      "thickness": 2
    },
    "directoryJumps": {
      "entries": [
        { "shortcut": "p", "directory": "~/projects" },
        { "shortcut": "d", "directory": "~/Downloads" }
      ]
    },
    "keyBindings": [
      { "key": "C-N", "command": "window.new" },
      { "key": "A-X", "command": "externalCommand.menu" }
    ],
    "externalCommands": [
      {
        "name": "Open in Vim",
        "extensions": ["go", "md"],
        "command": "vim",
        "args": ["{file}"],
        "cwd": "{dir}"
      }
    ]
  }
}
```

## Sections

`window`

- `width`, `height`: initial window size in pixels.
- `x`, `y`: optional initial window position. On Windows, NMF moves the first
  window to this position after startup and clamps it into the nearest monitor's
  work area if monitor layout changes would otherwise put it off-screen. Other
  platforms currently ignore these fields.

`startup`

- `directory`: starting directory used when no `-path` flag or positional path
  argument is supplied. Command-line paths always take precedence.

`theme`

- `dark`: `true` for dark theme, `false` for light theme.
- `fontSize`: base text size. `0` keeps the default.
- `fontName`: preferred system font name. Empty uses the built-in fallback list.
- `fontPath`: explicit font file path. Empty disables explicit file loading.
- `monospaceFontName`: preferred system font name for file/path style text.
  Empty inherits the regular font.
- `monospaceFontPath`: explicit font file path for file/path style text. Empty
  inherits the regular font.
- `colors`: optional app-specific color overrides. Values can be RGBA arrays,
  Fyne theme color names, or Fyne primary color names.

`debug`

- `enabled`: enable persistent debug logging for normal startup.
- `logDirectory`: directory for per-startup log files. Empty creates a `logs`
  directory next to `config.json` and `init.star`. Relative paths are resolved
  from that same config directory.
- `maxLogFiles`: number of `nmf-*.log` session files to keep. Older matching
  files are deleted after a new log is opened.

`ui`

- `showHiddenFiles`: show dotfiles and hidden files when supported.
- `sort.sortBy`: one of `name`, `size`, `modified`, or `extension`.
- `sort.sortOrder`: `asc` or `desc`.
- `sort.directoriesFirst`: keep directories before regular files.
- `itemSpacing`: list item spacing. `0` keeps the default.
- `scrollMargin`: number of rows kept between the cursor and the approaching
  top or bottom edge before scrolling begins. Defaults to `3`; `0` restores
  scrolling only when the cursor reaches the edge. The effective value is
  reduced when the viewport is too short to keep the cursor visible.
- `copy.preserveTimestamps`: default state for the Copy dialog's
  "Preserve timestamps" checkbox. When enabled for a copy, NMF preserves file
  and directory modification times; directory times are restored after children
  are copied.
- `viewer.maxWidth`, `viewer.maxHeight`: optional maximum size for the built-in
  file viewer dialog. `0` means uncapped.
- `viewer.defaultPane`: initial built-in viewer pane. `auto` opens supported
  images on Image, other binary files on Hex, Markdown files on Markdown, and
  other files on Text. Use `text` to prefer raw text for Markdown files. Other
  values are `markdown` and `hex`; explicitly selecting `hex` also starts image
  files on Hex.
- `viewer.defaultWrap`: initial wrapping state for each Text, Markdown, and Hex
  pane. Defaults to `false`; the Wrap button or `w` changes the active pane
  independently for the current viewer dialog.
- `archive.zipNameEncoding`: fallback charset for ZIP entry names that are not
  marked as UTF-8. Default is `shift_jis`; common alternatives include `cp437`
  and `utf-8`.
- `ime.enabled`: enable native IME candidate/composition position hints on
  platforms that support them. Set to `false` to disable this integration.
- `cursorStyle.type`: one of `underline`, `border`, `background`, `icon`, or
  `font`.
- `cursorStyle.thickness`: underline or border thickness.

## Debug Logging

For one-off debugging, `-d` still enables debug output to stderr and
`-debug-log /path/to/debug.log` writes to the specified file.

For long-running reproduction work, set `debug.enabled` in `config.json`.
NMF creates a new session log on each startup using names like
`nmf-20260608-213000-12345.log` and prunes old `nmf-*.log` files in that
directory according to `maxLogFiles`.

When debug logging is enabled, the main toolbar shows a debug action that dumps
the current KeyManager stack, modifiers, pressed keys, and pending input
transitions into the log. It is intended for cases where keyboard input stops
responding but mouse clicks still work.

## Colors

`theme.colors` customizes NMF-specific colors without replacing the whole Fyne
theme. Missing values keep the built-in defaults.

```json
{
  "theme": {
    "colors": {
      "fileRegular": "foreground",
      "fileDirectory": [135, 206, 250, 255],
      "cursor": {
        "dark": "foreground",
        "light": [0, 0, 0, 255]
      },
      "lineEditCursor": "foreground",
      "lineEditSelection": "selection",
      "dialogListCursor": [80, 120, 180, 140]
    }
  }
}
```

Configurable color names:

- `fileRegular`, `fileDirectory`, `fileSymlink`, `fileHidden`
- `statusAdded`, `statusDeleted`, `statusModified`
- `selectionBackground`, `cursor`
- `lineEditCursor`, `lineEditSelection`, `dialogListCursor`, `menuCursor`
- `copyMoveOpenDestination`
- `searchOverlayBackground`, `searchOverlayForeground`
- `busyOverlayBackground`

`lineEditCursor` and `lineEditSelection` apply to one-line edit dialogs and
the built-in File Viewer search/line inputs. `lineEditSelection` also applies
to mouse text selection in the File Viewer content panes.
`dialogListCursor` applies to keyboard cursor rows in Navigation History,
Directory Jump, Filter, Copy/Move, and Jobs lists.
`menuCursor` applies to command menu cursor rows.
`copyMoveOpenDestination` applies to Copy/Move destination rows and Navigation
History rows that are open in another File Manager window.

Color values:

- RGBA array: `[r, g, b, a]`, each value from `0` to `255`.
- Fyne theme color name: `background`, `foreground`, `primary`, `selection`,
  `focus`, `overlayBackground`, and other `theme.ColorName*` string values.
- Fyne primary color name: `red`, `orange`, `yellow`, `green`, `blue`,
  `purple`, `brown`, or `gray`.

Use an object when dark and light themes should differ:

```json
{
  "theme": {
    "colors": {
      "cursor": {
        "dark": "foreground",
        "light": [0, 0, 0, 255]
      }
    }
  }
}
```

Set `dark` or `light` to `null` to force that variant to keep the built-in
default even when `value` is set.

## Runtime State

NMF persists frequently-changing runtime state — remembered cursor positions,
navigation history (including saved History Jump paths), file filter history
plus the currently applied filter, and the last-applied sort — to a separate
`state.json` file, not to `config.json`. `config.json` is never written to by
the app.

`state.json` location:

- Linux/Unix: `$XDG_STATE_HOME/nekomimist/nmf/state.json`, or
  `~/.local/state/nekomimist/nmf/state.json`
- macOS: `~/Library/Application Support/nekomimist/nmf/state.json`
- Windows: `%LOCALAPPDATA%\nekomimist\nmf\state.json`

Saves are debounced (~500ms after the last change) and written atomically
(temp file, then rename), and pending writes are flushed on shutdown.

Shape of `state.json`:

```json
{
  "cursorMemory": {
    "entries": {},
    "lastUsed": {}
  },
  "navigationHistory": {
    "entries": [],
    "lastUsed": {},
    "useCount": {},
    "pinned": []
  },
  "fileFilter": {
    "entries": [],
    "current": null,
    "enabled": false
  },
  "sort": {
    "sortBy": "name",
    "sortOrder": "asc",
    "directoriesFirst": true
  }
}
```

- `cursorMemory.entries`/`lastUsed`: remembered cursor file name per
  directory, and its LRU timestamp. The entry limit is
  `ui.cursorMemory.maxEntries` (in `config.json`).
- `navigationHistory.entries`/`lastUsed`/`useCount`: visited-path history,
  shown sorted by zoxide-style frecency. `useCount` stores usage counters;
  missing values are migrated to `1`. The entry limit is
  `ui.navigationHistory.maxEntries` (in `config.json`).
- `navigationHistory.pinned`: saved History Jump paths. They are shown before
  regular history and are never pruned by `maxEntries`.
- `fileFilter.entries`/`current`/`enabled`: filter history, the currently
  applied filter, and whether it is enabled. Entries use `pattern`,
  `lastUsed`, and `useCount`; text after `;;` in a pattern is a searchable
  comment, and only the text before `;;` is used for matching. The entry
  limit is `ui.fileFilter.maxEntries` (in `config.json`).
- `sort`: the sort last applied through the Sort dialog, omitted until a sort
  has been applied. While present, it overrides `ui.sort` from `config.json`;
  `config.json`'s `ui.sort` is only used as the initial default before any
  sort has been applied, or after the `sort` key is removed from
  `state.json`.
- history timestamps use Go's JSON `time.Time` format.
- navigation history paths are normalized when recorded or shown; SMB/UNC forms
  are stored as canonical `smb://host/share/...` paths.

`config.json` keeps only the entry-count knobs for these features
(`ui.cursorMemory.maxEntries`, `ui.navigationHistory.maxEntries`,
`ui.fileFilter.maxEntries`) plus `ui.sort`, used as described above.

### Migration from older config.json

Older NMF versions stored this runtime state directly in `config.json` under
`ui.cursorMemory`, `ui.navigationHistory`, and `ui.fileFilter`. The first time
NMF runs with no `state.json` present, it performs a one-time migration: it
reads those legacy keys out of `config.json` (without ever writing to
`config.json`) and uses them to seed a new `state.json`.

This migration is triggered by `state.json` being absent, not by any marker
in `config.json`. Deleting `state.json` while the old runtime keys are still
present in `config.json` re-triggers migration and resurrects that old
cursor/history/filter data on the next startup. To fully reset runtime state,
also remove the old `entries`/`current`/`enabled`/etc. sub-fields from
`ui.cursorMemory`, `ui.navigationHistory`, and `ui.fileFilter` in
`config.json` by hand, leaving only `maxEntries` in each.

## Search Matching

Navigation History, Apply Filter, Incremental Search, and Copy/Move destination
search use case-insensitive substring matching plus migemo expansion for
Japanese names.
Whitespace-separated query tokens are combined as unordered AND conditions.
Migemo uses the embedded dictionary and is enabled automatically when it loads;
if it is unavailable, the app falls back to substring matching.
History Jump also includes `navigationHistory.pinned` paths; saved rows are
marked with `*`.

Directory Jump is intentionally separate: it filters only by configured shortcut
prefix and does not use migemo.

## Directory Jumps

`ui.directoryJumps.entries` is a list of jump targets.

```json
{
  "ui": {
    "directoryJumps": {
      "entries": [
        { "shortcut": "p", "directory": "~/projects" },
        { "shortcut": "", "directory": "/tmp" }
      ]
    }
  }
}
```

- `shortcut`: empty or a shortcut prefix. Matching is case-insensitive.
- `directory`: path as written in config; path resolution happens when used.

The Directory Jump dialog filters by shortcut prefix. When a non-empty filter
leaves exactly one candidate, nmf jumps to that directory automatically. The
unfiltered list shows shortcut entries first, sorted by shortcut length and then
alphabetically; entries without shortcuts appear last.

## Key Bindings

`ui.keyBindings` adds key bindings. User bindings are evaluated before built-in
defaults for the same target, so a binding for the same key overrides the
default behavior. `target` is optional and defaults to `main`; supported values
are `main`, `lineEdit`, and `fileViewer`.

```json
{
  "ui": {
    "keyBindings": [
      { "key": "S-A-C-F2", "command": "rename.show" },
      { "key": "C-N", "command": "window.new" },
      { "key": "F12", "command": "maintenance.show" },
      { "key": "S-S", "command": "noop" },
      { "target": "lineEdit", "key": "C-A", "command": "lineEdit.cursor.start" },
      { "target": "fileViewer", "key": "J", "command": "fileViewer.page.down" }
    ]
  }
}
```

Key syntax:

- plain key: `F2`, `Return`, `Delete`, `A`
- Shift: `S-Key`
- Alt/Meta: `A-Key`
- Ctrl: `C-Key`
- modifiers can be combined, for example `S-A-C-F2` or `S-C-Up`

`Key` must be a valid `fyne.KeyName` value. Common values include `Up`,
`Down`, `Return`, `BackSpace`, `Delete`, `F1` through `F12`, letters, digits,
and punctuation key values such as `-`, `.`, `/`, `[`, and `]`.

Supported aliases:

- `Backspace` -> `BackSpace`
- `Enter` -> `Return`
- `Esc` -> `Escape`
- `PageUp` -> `Prior`
- `PageDown` -> `Next`
- `Comma` -> `,`
- `Period` or `Dot` -> `.`
- `Backtick` or `Backquote` -> `` ` ``
- `Semicolon` -> `;`
- `Del` -> `Delete`

Invalid key names, invalid modifiers, and unknown commands are logged as
warnings and only that binding entry is ignored. Unknown `target` values are
also warned at startup, and the entry is ignored.

Bindings fire when the key combination activates (Fyne typed key or shortcut,
chosen automatically from the key spec) and repeat while the key is held. The
legacy `event` field (`typed`/`down`/`up`) is deprecated: it is accepted for
backward compatibility but ignored with a warning.

Built-in window-size reset bindings are `S-Q` for the current File Manager
window and `C-S-Q` for all File Manager windows.
The built-in History Jump save binding is `S-B`, which pins the current
directory in `navigationHistory.pinned`.

Available main-screen commands:

- `cursor.up`, `cursor.down`, `cursor.pageUp`, `cursor.pageDown`
- `cursor.first`, `cursor.last`
- `open`, `open.defaultApp`, `selection.toggle`, `selection.markAll`
- `selection.invert`, `selection.invertWithDirectories`
- `directory.parent`, `directory.refresh`, `directory.home`, `directory.create`
- `clipboard.createTextFile`
- `window.new`, `window.reopen`, `window.focusLeft`, `window.focusRight`
- `window.resetSize`, `window.resetAllSizes`
- `tree.show`, `history.show`, `history.pinCurrent`, `directoryJump.show`
- `filter.show`, `filter.clear`, `filter.toggle`
- `search.show`, `sort.show`, `jobs.show`
- `path.edit`, `app.quit`
- `copy.show`, `move.show`, `archive.extract`, `compare.show`, `rename.show`
- `delete.trash`, `delete.permanent`
- `explorerContext.show`
- `externalCommand.menu`
- `viewer.show`
- `maintenance.show`
- `noop`

Starlark `init.star` can register additional command IDs with the `user.`
prefix and bind them through the same key binding mechanism.

Available line-edit commands:

- `lineEdit.accept`, `lineEdit.cancel`
- `lineEdit.cursor.start`, `lineEdit.cursor.end`
- `lineEdit.cursor.left`, `lineEdit.cursor.right`
- `lineEdit.delete.before`, `lineEdit.delete.at`
- `lineEdit.delete.beforeStart`, `lineEdit.delete.afterEnd`
- `lineEdit.paste`
- `noop`

Available file-viewer commands:

- `fileViewer.close`
- `fileViewer.line.down`, `fileViewer.line.up`
- `fileViewer.page.down`, `fileViewer.page.up`
- `fileViewer.home`, `fileViewer.end`
- `fileViewer.column.left`, `fileViewer.column.right`
- `fileViewer.wrap.toggle`
- `fileViewer.pane.image`, `fileViewer.pane.text`, `fileViewer.pane.markdown`,
  `fileViewer.pane.hex`
- `fileViewer.image.zoom.toggle`, `fileViewer.image.zoom.in`,
  `fileViewer.image.zoom.out`
- `fileViewer.search.next`, `fileViewer.search.previous`, `fileViewer.search.focus`
- `fileViewer.line.focus`
- `fileViewer.selection.selectAll`, `fileViewer.selection.copy`
- `noop`

## External Commands

`ui.externalCommands` defines commands shown from the main-screen external
command menu.

```json
{
  "ui": {
    "externalCommands": [
      {
        "name": "Open in editor",
        "key": "E",
        "extensions": ["go", "md"],
        "command": "vim",
        "args": ["{file}"],
        "edit": true
      },
      {
        "name": "Compare selected",
        "extensions": ["*"],
        "command": "meld",
        "args": ["{files}"]
      }
    ]
  }
}
```

- `name`: menu label.
- `key`: optional single printable character. In the command menu, the first
  visible item using a key wins case-insensitively; later duplicates behave as
  if they had no key.
- `extensions`: optional case-insensitive extension list. Dots are optional;
  `*` matches all files. Empty list also matches all files.
- `command`: executable name or path. It may be empty when `edit` is true.
- `args`: optional argument templates. If omitted, selected target paths are
  passed as arguments.
- `cwd`: optional working directory. It supports `{file}`, `{dir}`, and
  `{name}` placeholders. If omitted, nmf preserves the existing process working
  directory behavior. Virtual paths such as archives and direct SMB providers
  are ignored.
- `edit`: optional boolean. When true, nmf shows the final command line in a
  one-line edit dialog before running it.

Supported argument placeholders:

- `{file}`: first target path.
- `{files}`: all target paths. If used as the whole argument, each file becomes
  its own argument.
- `{all_files}`: marked files from all open file manager windows, without cursor
  fallback. If used as the whole argument, each file becomes its own argument.
- `{dir}`: current directory.
- `{name}`: base name of the first target.

Edited command lines are split with shell-like quote and backslash handling, but
are still executed directly without a shell.
