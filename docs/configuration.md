# Configuration

NMF loads `config.json` from the OS-specific app config directory:

- Linux/Unix: `$XDG_CONFIG_HOME/nekomimist/nmf/config.json`, or
  `~/.config/nekomimist/nmf/config.json`
- macOS: `~/Library/Application Support/nekomimist/nmf/config.json`
- Windows: `%APPDATA%\nekomimist\nmf\config.json`

The schema source of truth is `internal/config/config.go`. Missing fields use
defaults, and some runtime state is saved back into this file.

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
    "fontPath": ""
  },
  "ui": {
    "showHiddenFiles": false,
    "sort": {
      "sortBy": "name",
      "sortOrder": "asc",
      "directoriesFirst": true
    },
    "itemSpacing": 4,
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
        "args": ["{file}"]
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

`ui`

- `showHiddenFiles`: show dotfiles and hidden files when supported.
- `sort.sortBy`: one of `name`, `size`, `modified`, or `extension`.
- `sort.sortOrder`: `asc` or `desc`.
- `sort.directoriesFirst`: keep directories before regular files.
- `itemSpacing`: list item spacing. `0` keeps the default.
- `cursorStyle.type`: one of `underline`, `border`, `background`, `icon`, or
  `font`.
- `cursorStyle.thickness`: underline or border thickness.

## Runtime State

These fields are managed by the app and normally do not need manual editing:

- `ui.cursorMemory`: remembered cursor positions per directory.
- `ui.navigationHistory`: recent navigation paths.
- `ui.fileFilter`: filter history and current filter state.

If manually editing them, preserve their JSON shape:

- history timestamps use Go's JSON `time.Time` format.
- `fileFilter.entries` and `fileFilter.current` use `pattern`, `lastUsed`, and
  `useCount`.

## Directory Jumps

`ui.directoryJumps.entries` is an ordered list of jump targets.

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

- `shortcut`: empty or one character. Matching is case-insensitive.
- `directory`: path as written in config; path resolution happens when used.

## Key Bindings

`ui.keyBindings` adds main-screen key bindings. User bindings are evaluated
before built-in defaults, so a binding for the same key and event overrides the
default behavior.

```json
{
  "ui": {
    "keyBindings": [
      { "key": "S-A-C-F2", "command": "rename.show", "event": "typed" },
      { "key": "C-N", "command": "window.new", "event": "down" }
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

Available main-screen commands:

- `cursor.up`, `cursor.down`, `cursor.pageUp`, `cursor.pageDown`
- `cursor.first`, `cursor.last`
- `open`, `selection.toggle`
- `directory.parent`, `directory.refresh`, `directory.home`
- `window.new`
- `tree.show`, `history.show`, `directoryJump.show`
- `filter.show`, `filter.clear`, `filter.toggle`
- `search.show`, `sort.show`, `jobs.show`
- `path.focus`, `app.quit`
- `copy.show`, `move.show`, `rename.show`
- `delete.trash`, `delete.permanent`
- `explorerContext.show`
- `externalCommand.menu`

## External Commands

`ui.externalCommands` defines commands shown from the main-screen external
command menu.

```json
{
  "ui": {
    "externalCommands": [
      {
        "name": "Open in editor",
        "extensions": ["go", "md"],
        "command": "vim",
        "args": ["{file}"]
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
- `extensions`: optional case-insensitive extension list. Dots are optional;
  `*` matches all files. Empty list also matches all files.
- `command`: executable name or path.
- `args`: optional argument templates. If omitted, selected target paths are
  passed as arguments.

Supported argument placeholders:

- `{file}`: first target path.
- `{files}`: all target paths. If used as the whole argument, each file becomes
  its own argument.
- `{dir}`: current directory.
- `{name}`: base name of the first target.
