# Configuration

NMF loads `config.json` from the OS-specific app config directory:

- Linux/Unix: `$XDG_CONFIG_HOME/nekomimist/nmf/config.json`, or
  `~/.config/nekomimist/nmf/config.json`
- macOS: `~/Library/Application Support/nekomimist/nmf/config.json`
- Windows: `%APPDATA%\nekomimist\nmf\config.json`

The schema source of truth is `internal/config/config.go`. Missing fields use
defaults, and some runtime state is saved back into this file.

After `config.json` is loaded, NMF also loads an optional `init.star` from the
same directory. Starlark settings overlay JSON for the current run and can define
custom commands for key bindings. See `docs/starlark-configuration.md`.

## Example

```json
{
  "window": {
    "width": 1000,
    "height": 720
  },
  "theme": {
    "dark": true,
    "fontSize": 14,
    "fontName": "Noto Sans CJK JP",
    "fontPath": "",
    "colors": {
      "cursor": {
        "dark": "foreground",
        "light": [0, 0, 0, 255]
      },
      "selectionBackground": "selection"
    }
  },
  "ui": {
    "showHiddenFiles": false,
    "sort": {
      "sortBy": "name",
      "sortOrder": "asc",
      "directoriesFirst": true
    },
    "itemSpacing": 4,
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
      { "key": "C-N", "command": "window.new", "event": "down" },
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

`theme`

- `dark`: `true` for dark theme, `false` for light theme.
- `fontSize`: base text size. `0` keeps the default.
- `fontName`: preferred system font name. Empty uses the built-in fallback list.
- `fontPath`: explicit font file path. Empty disables explicit file loading.
- `colors`: optional app-specific color overrides. Values can be RGBA arrays,
  Fyne theme color names, or Fyne primary color names.

`ui`

- `showHiddenFiles`: show dotfiles and hidden files when supported.
- `sort.sortBy`: one of `name`, `size`, `modified`, or `extension`.
- `sort.sortOrder`: `asc` or `desc`.
- `sort.directoriesFirst`: keep directories before regular files.
- `itemSpacing`: list item spacing. `0` keeps the default.
- `ime.enabled`: enable native IME candidate/composition position hints on
  platforms that support them. Set to `false` to disable this integration.
- `cursorStyle.type`: one of `underline`, `border`, `background`, `icon`, or
  `font`.
- `cursorStyle.thickness`: underline or border thickness.

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

`lineEditCursor` and `lineEditSelection` apply only to one-line edit dialogs.
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

These fields are managed by the app and normally do not need manual editing:

- `ui.cursorMemory`: remembered cursor positions per directory.
- `ui.navigationHistory`: recent navigation paths.
- `ui.fileFilter`: filter history and current filter state.

If manually editing them, preserve their JSON shape:

- history timestamps use Go's JSON `time.Time` format.
- `fileFilter.entries` and `fileFilter.current` use `pattern`, `lastUsed`, and
  `useCount`.
- navigation history paths are normalized when recorded or shown; SMB/UNC forms
  are stored as canonical `smb://host/share/...` paths.

## Search Matching

Navigation History and Incremental Search use case-insensitive substring
matching plus migemo expansion for Japanese names. Migemo uses the embedded
dictionary and is enabled automatically when it loads; if it is unavailable, the
app falls back to substring matching.

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

`ui.keyBindings` adds main-screen key bindings. User bindings are evaluated
before built-in defaults, so a binding for the same key and event overrides the
default behavior.

```json
{
  "ui": {
    "keyBindings": [
      { "key": "S-A-C-F2", "command": "rename.show", "event": "typed" },
      { "key": "C-N", "command": "window.new", "event": "down" },
      { "key": "F12", "command": "maintenance.show" },
      { "key": "S-S", "command": "noop" }
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
- `Del` -> `Delete`

Invalid key names, invalid modifiers, invalid events, and unknown commands are
logged as warnings and only that binding entry is ignored.

Events:

- `typed`: Fyne typed key event; best for normal keys that should repeat.
- `down`: desktop key down event; useful for Ctrl/Alt combinations.
- `up`: desktop key up event.

When `event` is omitted, Ctrl or Alt bindings default to `down`; other bindings
default to `typed`.

Built-in window-size reset bindings are `S-Q` for the current File Manager
window and `C-S-Q` for all File Manager windows.

Available main-screen commands:

- `cursor.up`, `cursor.down`, `cursor.pageUp`, `cursor.pageDown`
- `cursor.first`, `cursor.last`
- `open`, `open.defaultApp`, `selection.toggle`, `selection.markAll`
- `directory.parent`, `directory.refresh`, `directory.home`, `directory.create`
- `clipboard.createTextFile`
- `window.new`, `window.reopen`, `window.focusLeft`, `window.focusRight`
- `window.resetSize`, `window.resetAllSizes`
- `tree.show`, `history.show`, `directoryJump.show`
- `filter.show`, `filter.clear`, `filter.toggle`
- `search.show`, `sort.show`, `jobs.show`
- `path.edit`, `app.quit`
- `copy.show`, `move.show`, `rename.show`
- `delete.trash`, `delete.permanent`
- `explorerContext.show`
- `externalCommand.menu`
- `viewer.show`
- `maintenance.show`
- `noop`

Starlark `init.star` can register additional command IDs with the `user.`
prefix and bind them through the same key binding mechanism.

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
