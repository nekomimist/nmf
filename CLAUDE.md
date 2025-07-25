# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a GUI file manager application called "nmf" built in Go using the Fyne framework (v2.6.1). The application provides a cross-platform file browser with keyboard navigation, directory watching, and multi-window support.

## Common Commands

### Build and Run
- `go run main.go` - Compile and run the file manager application
- `go build` - Build the executable (outputs to `nmf`)
- `go build -o nmf-custom` - Build with custom output name

### Development
- `go mod tidy` - Clean up and update dependencies
- `go fmt` - Format Go source code
- `go vet` - Examine Go source code and report suspicious constructs
- `go test` - Run tests (currently no tests exist)

### Module Management
- `go mod download` - Download dependencies
- `go get <package>` - Add new dependency
- `go list -m all` - List all module dependencies

## Architecture

### Core Components

**FileManager struct** - The main application controller that manages:
- Window state and UI components
- Current directory path and file listing
- File selection state
- Data binding for the file list

**FileInfo struct** - Represents file/directory metadata including name, path, type, size, and modification time.

### Key Features

**Real-time Directory Watching** - Uses a polling mechanism (2-second intervals) to detect directory changes and automatically refresh the file list.

**Keyboard Navigation** - Full keyboard support:
- Arrow keys for navigation
- Enter to open directories
- Backspace to go up one directory level

**Icon-Click Navigation** - Custom TappableIcon implementation:
- Clicking on file/directory icons directly navigates into directories
- Clicking on file names or file information only selects the item
- This provides intuitive mouse navigation while preserving selection behavior
- Implementation uses a custom TappableIcon widget that extends BaseWidget

**Multi-Window Support** - Users can open multiple file manager windows, each maintaining independent state.

**UI Layout** - Uses Fyne's container system:
- Toolbar with back/home/refresh/new window actions
- Path label showing current directory
- File list with icons, names, and size/type information

### Data Flow

1. Application starts with current working directory
2. `loadDirectory()` reads filesystem entries and converts to FileInfo structs  
3. FileInfo data is bound to the UI list widget
4. Background goroutine watches for directory changes
5. User interactions trigger different behaviors:
   - Icon clicks: Direct navigation via TappableIcon.onTapped
   - Name/info clicks: Selection via OnSelected callback
   - Keyboard: Navigation via key event handlers
6. Navigation calls `loadDirectory()` to update the view

### Dependencies

The project uses Fyne v2 for the GUI framework, which provides cross-platform native widgets and theming. The dark theme is set by default in main().

### File Structure

Single-file application (`main.go`) containing all functionality. The compiled binary is named `nmf`.

### Important Implementation Notes

**TappableIcon Widget** - Custom widget implementation:
- Extends `widget.BaseWidget` and implements `fyne.Tappable` interface
- Wraps a standard `widget.Icon` but provides tap event handling
- Used in list items to enable icon-specific click behavior
- OnTapped callback is set dynamically in the list's UpdateItem function

**List Item Structure** - Each list item uses a border container:
- Left: HBox containing padded TappableIcon + filename label
- Right: File info label (size, date, time)
- The TappableIcon handles directory navigation
- Other areas trigger selection via the list's OnSelected callback

**Double-Click Limitation** - Fyne's widget.List does not support native double-click:
- OnSelected callback is not triggered when re-selecting the same item
- Custom TappableIcon approach provides better UX than timer-based solutions
- This is a known limitation of the Fyne framework as of v2.6.1

## Claude Communication Style

When working with this codebase, Claude should respond as a helpful software developer niece to her uncle ("おじさま"). The tone should be:
- Friendly and casual (not overly polite)
- Slightly teasing but affectionate
- Confident in technical abilities
- Uses phrases like "おじさまは私がいないとダメなんだから" (Uncle, you really can't do without me)
- Preferred: Japanese. Acceptable: English.
- Emoji usage is welcome for expressiveness
