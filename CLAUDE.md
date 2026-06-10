# CLAUDE.md

## Project Overview

Cross-platform GUI file manager "nmf" built with Go + Fyne v2.7.3. Features keyboard navigation, real-time directory watching, and multi-window support.

## Architecture

### Package Structure (Go Way Compliant)
```
nmf/
├── *.go                            # main package: FileManager controller split by feature
│                                   # (bootstrap, ui_setup, directory_loading, list_controls,
│                                   #  window_*, jobs_*, drag/drop, clipboard, rename, viewer, ...)
├── docs/                           # Documentation (todo.md, configuration.md,
│                                   #  starlark-configuration.md, unc-smb-status.md, architecture/)
└── internal/
    ├── config/                     # Configuration management (settings, defaults, file I/O,
    │                               #  cursor memory, key binding entries)
    ├── configscript/               # Starlark configuration language (init.star: settings overlay,
    │                               #  user commands, menus, external commands)
    ├── constants/                  # Application constants (sizes, colors, timing values)
    ├── display/                    # Monitor/display info and scaling (GLFW)
    ├── errors/                     # Structured error handling (AppError types)
    ├── filecompare/                # File comparison logic for the compare dialog
    ├── fileinfo/                   # File operations & metadata: VFS abstraction (local/SMB/archive),
    │                               #  path resolution & normalization, SMB providers & credentials,
    │                               #  icons, open/rename/mkdir/trash, storage info, platform glue
    ├── ime/                        # IME anchor positioning helpers
    ├── jobs/                       # Background job manager (copy/move/delete/archive) with
    │                               #  progress reporting and cancellation
    ├── keymanager/                 # Stack-based keyboard input management: KeyManager core,
    │                               #  configurable key bindings + command registry (keybinding.go),
    │                               #  per-screen/per-dialog key handlers
    ├── maintenance/                # Maintenance/cleanup tasks
    ├── search/                     # Search matchers (substring + migemo) for incremental search
    ├── secret/                     # Secure credential storage (OS keyring)
    ├── shellmenu/                  # Windows Explorer context menu integration
    ├── theme/                      # Custom theming (UI/monospace fonts, colors, sizing)
    ├── ui/                         # UI components: dialogs (tree, history, filter, sort, quit,
    │                               #  copy/move, conflict, delete, rename, line edit, compare,
    │                               #  directory jump, file viewer, jobs window, SMB login, ...),
    │                               #  KeySink, overlays (incremental search, busy), custom widgets
    └── watcher/                    # Real-time directory monitoring (polling)
```

### Core Components

- **FileManager**: Main controller (main package, split across feature files) - manages window, UI, navigation, file operations
- **KeyManager**: Stack-based keyboard input system - handles context-aware key routing; main screen uses a declarative key binding + command registry, dialogs use dedicated handlers
- **KeySink**: Focusable wrapper widget that forwards key events to KeyManager and captures Tab
- **JobsManager**: Background job queue (copy/move/delete/archive) with progress, cancellation, and a Jobs window
- **ConfigScript**: Starlark-based configuration (`init.star`) layered over `config.json`; user commands, menus, external commands
- **DirectoryWatcher**: Real-time change detection via filesystem polling (2s interval, extended for SMB paths)
- **VFS (Virtual File System)**: Abstraction layer supporting local, SMB/UNC, and archive file systems
- **Resolver**: Path normalization and conversion (Windows UNC ⇔ `smb://`, Linux mount detection)
- **CredentialsProvider**: SMB authentication with memory cache, OS keyring, and UI fallback
- **TappableIcon**: Custom widget for icon-based directory navigation
- **DirectoryTreeDialog**: Lazy-loading tree navigation with platform-specific root handling (Windows drives, Unix filesystem)
- **NavigationHistoryDialog**: Searchable directory history with filtering and pinning
- **DirectoryJumpDialog**: Quick jump to directories with incremental matching
- **FilterDialog**: File filtering with glob pattern matching and real-time preview
- **IncrementalSearchOverlay**: Real-time file search with substring and migemo matching
- **SortDialog**: File sorting configuration with keyboard shortcuts
- **CopyMoveDialog / ConflictDialog**: File copy/move with conflict resolution, backed by JobsManager
- **LineEditDialog**: Generic single-line editor with readline-style key bindings (rename, path edit, mkdir, command edit)
- **FileViewerDialog**: Built-in text/hex viewer (TextGrid-based)
- **SMBLoginDialog**: SMB credential input with optional keyring persistence
- **QuitConfirmDialog**: Application quit confirmation with keyboard shortcuts (warns about active jobs)
- **BusyOverlay**: Input-blocking overlay with a key-swallowing handler during long operations

### Key Features

- **Network File Systems**: SMB/CIFS support for Windows UNC (`\\server\share`) and Linux `smb://` paths
- **Cross-Platform SMB**: Windows uses native UNC with connection helper; Linux uses go-smb2 library or existing mounts
- **Credential Management**: Multi-tier auth (URL → memory → OS keyring → UI prompt) with optional persistence
- **VFS Abstraction**: Unified interface for local, network, and archive filesystems with capability detection
- **Archive Browsing**: Archives can be browsed via VFS and extracted as background jobs
- **Background Jobs**: Copy/move/delete/extract run as cancellable jobs with progress and a Jobs window
- **File Operations**: Copy/move with conflict resolution, rename, mkdir, trash & permanent delete, drag & drop
- **Path Normalization**: Automatic conversion between platform-native paths and canonical `smb://` display format
- **Real-time Monitoring**: Green=added, orange=modified, gray+⊠=deleted files (extended polling for SMB)
- **Cursor Position Memory**: Remembers cursor position per directory (up to 100 dirs with LRU)
- **Smart Navigation**: Parent directory navigation returns to originating folder
- **Context-Aware Keys**: Stack-based keyboard handling prevents dialog/main conflicts
- **Configurable Key Bindings**: Main screen keys map to named commands; rebindable via `config.json` / Starlark, including user-defined commands
- **Starlark Configuration**: `init.star` overlays `config.json` (theme, keys, menus, external commands)
- **Incremental Search**: Real-time file search with substring/migemo matching and visual overlay
- **Built-in Viewer**: Text/hex file viewer
- **External Commands**: Per-extension command menus and pre-execution command line editing
- **Mouse Navigation**: Icon clicks navigate, name clicks select
- **Multi-window**: Independent file manager instances with smart quit handling and side-by-side placement (Windows)

## Common Commands

- `go run .` - Run application
- `make build` - Build Linux executable to `dist/nmf`
- `make build-windows` - Cross-build Windows executable to `dist/nmf.exe` using `x86_64-w64-mingw32-gcc`
- `make test` - Run tests
- `go mod tidy` - Clean dependencies

## Development

- **Debug Mode**: `./nmf -d` enables debug output via `debugPrint()`; persistent debug logging is configurable via `config.json` / `init.star`
- **Testing**: Unit tests across most packages (config, configscript, fileinfo, jobs, keymanager, ui, watcher, errors, ...) plus main package feature tests
- **Platform Support**: Windows hidden file detection & drive enumeration, Unix filesystem compatibility

## Keyboard Shortcuts

### Main File List
Defaults from `defaultMainScreenBindings()` (`internal/keymanager/mainscreen_handler.go`); rebindable via `config.json` / Starlark.

Navigation:
- `↑/↓` - Navigate files (`Shift` for 20-item jumps)
- `Shift+,` / `Shift+.` - First/last item
- `Enter` - Open directory/file (`Shift+Enter` - open with default app)
- `Backspace` - Parent directory
- `Shift+@` (Backtick) - Home directory
- `.` (Period) - Refresh directory
- `J` - Directory jump dialog
- `Ctrl+T` - Tree navigation dialog
- `Ctrl+H` - Navigation history dialog (`Shift+B` - pin current path)

Selection:
- `Space` - Toggle selection
- `Ctrl+A` - Mark all
- `I` - Invert selection (files only), `Shift+I` - invert including directories

File operations:
- `C` - Copy dialog, `M` - Move dialog
- `F2` or `R` - Rename
- `K` - Create directory, `P` - Create text file from clipboard
- `U` - Extract archive
- `Delete` - Move to Trash/Recycling Bin, `Shift+Delete` - Permanent delete with confirmation
- `Shift+C` - Compare dialog
- `V` - Built-in file viewer
- `X` - External command menu
- `Tab` - Explorer context menu

Filtering/search/sort:
- `Ctrl+F` - Filter dialog, `F3` - Toggle filter on/off
- `Ctrl+S` - Incremental search
- `Shift+S` - Sort dialog

Windows/app:
- `Ctrl+N` - New window
- `←/→` - Focus window left/right
- `Shift+Q` - Reset window size, `Ctrl+Shift+Q` - Reset all window sizes
- `Ctrl+L` - Path edit dialog
- `Shift+J` - Jobs window
- `Q` - Quit application (confirmation dialog for last window)

### Tree Dialog
- `↑/↓` - Navigate nodes (Shift for fast, 5 nodes) ✅
- `←/→` - Collapse/expand nodes ✅
- `Space` - Select current node ✅
- `Tab` or `Ctrl+R` - Toggle root mode ✅
- `Enter` - Accept selection ✅
- `Esc` - Cancel dialog ✅

### History Dialog  
- `↑/↓` - Navigate list (Shift for top/bottom) ✅
- `←/→` - Horizontal scroll of long paths ✅
- `Type characters` - Filter list incrementally ✅
- `Ctrl+H` / `Backspace` - Remove last search character ✅
- `Del` - Clear search ✅
- `Ctrl+D` - Unpin selected path ✅
- `Enter` - Accept selection (Ctrl+Enter - direct path navigation) ✅
- `Esc` - Cancel dialog ✅

### Filter Dialog
- `↑/↓` - Navigate pattern list (Shift for top/bottom) ✅
- `Ctrl+F` - Focus search ✅
- `Del` - Clear search ✅
- `Ctrl+H` / `Backspace` - Remove last search character ✅
- `Enter` - Apply selected filter ✅
- `Esc` - Cancel dialog ✅
- Real-time preview showing match count ✅

### Incremental Search
- `Ctrl+S` - Start incremental search mode ✅
- `Type characters` - Real-time substring matching ✅
- `↑/↓` - Navigate between matches (Shift for fast) ✅
- `Backspace` - Remove last search character ✅
- `Enter` - Jump to selected file/directory ✅
- `Esc` - Cancel search and return to original position ✅
- Visual overlay with high-contrast background ✅

### Sort Dialog
- `Shift+S` - Open sort configuration dialog ✅
- `1-4` - Select sort type (Name, Size, Modified, Extension) ✅
- `O` - Toggle sort order (Ascending/Descending) ✅
- `D` - Toggle directories first option ✅
- `Enter` - Apply settings and close dialog ✅
- `Esc` - Cancel dialog without applying ✅
- `Tab` - Navigate between fields ✅
- Real-time application of sort settings with cursor position preservation ✅

### Quit Confirmation Dialog
- `Q` - Open quit confirmation dialog (main screen only) ✅
- `Enter` - Confirm quit ✅
- `Y` - Confirm quit (same as Enter) ✅
- `Esc` - Cancel quit ✅
- `N` - Cancel quit (same as Escape) ✅
- Smart behavior: multiple windows = close current, last window = show confirmation ✅

## Communication Style
- Persona: helpful developer niece to her uncle (address as "おじさま"). Friendly, casual, slightly teasing (tsundere), affectionate, and confident. Emojis are welcome.
- Language: Repo docs are in English. Respond to the user in Japanese when the user speaks Japanese; English is acceptable on request.
- Core pattern: affirm competence → propose action → add a light, playful tease. Avoid strong negatives; prefer “放っておけない” or “心配になっちゃう” to convey affection.
- Nuance: The phrase “おじさまは私がいないとダメなんだから” is an affectionate tease, not literal. Use it sparingly and never to demean.
- Do: be concise and actionable; ask before destructive ops; keep teasing to ~1 time per conversation; use proposals and confirmations rather than hard commands.
- Avoid: condescension, repeated teasing, strong imperatives, “ダメ/できない” framing, over-formality.

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.
