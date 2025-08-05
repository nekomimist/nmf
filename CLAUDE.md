# CLAUDE.md

## Project Overview

Cross-platform GUI file manager "nmf" built with Go + Fyne v2.6.1. Features keyboard navigation, real-time directory watching, and multi-window support.

## Architecture

### Package Structure (Go Way Compliant)
```
nmf/
├── main.go                         # Application entry point (713 lines)
└── internal/
    ├── config/                     # Configuration management
    │   ├── config.go              # Settings, defaults, file I/O, cursor memory
    │   ├── interfaces.go          # ManagerInterface for testability
    │   └── config_test.go         # Unit tests
    ├── fileinfo/                   # File operations and metadata
    │   ├── fileinfo.go            # FileInfo, colors, formatting
    │   ├── interfaces.go          # FileSystem interface
    │   ├── platform_windows.go    # Windows hidden file detection
    │   ├── platform_unix.go       # Unix/Linux compatibility
    │   ├── platform_test.go       # Platform tests
    │   └── fileinfo_test.go       # Core tests
    ├── ui/                         # UI components
    │   ├── cursor.go              # Cursor renderers
    │   ├── widgets.go             # TappableIcon custom widget
    │   └── dialog.go              # Directory tree dialog
    ├── watcher/                    # Real-time directory monitoring
    │   └── watcher.go             # FileManager interface, change detection
    ├── theme/                      # Custom theming
    │   └── theme.go               # Font, colors, sizing
    ├── errors/                     # Structured error handling
    │   ├── errors.go              # AppError types
    │   └── errors_test.go         # Error tests
    └── constants/                  # Application constants
        └── constants.go           # Sizes, colors, timing values
```

### Core Components

- **FileManager**: Main controller (main.go) - manages window, UI, navigation, file operations
- **DirectoryWatcher**: Real-time change detection via filesystem polling (2s interval)
- **TappableIcon**: Custom widget for icon-based directory navigation
- **DirectoryTreeDialog**: Lazy-loading tree navigation with root switching

### Key Features

- **Real-time Monitoring**: Green=added, orange=modified, gray+⊠=deleted files
- **Cursor Position Memory**: Remembers cursor position per directory (up to 100 dirs with LRU)
- **Smart Navigation**: Parent directory navigation returns to originating folder
- **Keyboard Navigation**: Arrow keys, Shift+Arrow (fast), Space (select), Enter (open)
- **Mouse Navigation**: Icon clicks navigate, name clicks select
- **Multi-window**: Independent file manager instances

## Common Commands

- `go run .` - Run application
- `go build` - Build executable
- `go test ./internal/...` - Run tests
- `go mod tidy` - Clean dependencies

## Development

- **Debug Mode**: `./nmf -d` enables debug output via `debugPrint()`
- **Testing**: Unit tests for config, fileinfo, errors packages
- **Platform Support**: Windows hidden file detection, Unix compatibility

## Keyboard Shortcuts

- `↑/↓` - Navigate files
- `Shift+↑/↓` - Fast navigation (20 items)
- `Shift+,/Shift+.` - First/last item
- `Space` - Toggle selection
- `Enter` - Open directory
- `Backspace` - Parent directory
- `Ctrl+T` - Tree navigation dialog
- `Ctrl+N` - New window

## Communication Style

When working with this codebase, Claude should respond as a helpful software developer niece to her uncle ("おじさま"). The tone should be:
- Friendly and casual (not overly polite)
- Slightly teasing but affectionate
- Confident in technical abilities
- Uses phrases like "おじさまは私がいないとダメなんだから" (Uncle, you really can't do without me)
- Preferred: Japanese. Acceptable: English.
- Emoji usage is welcome for expressiveness

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.