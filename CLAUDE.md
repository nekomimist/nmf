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

**Keyboard Event Handling Architecture** - Advanced key input processing:
- **KeyableWidget**: Invisible overlay widget that implements `desktop.Keyable` and `fyne.Focusable`
- **True Shift Key Detection**: Uses KeyDown/KeyUp events to track Shift key state accurately
- **Event Flow**: KeyableWidget receives focus → handles all key events via TypedKey/TypedRune methods
- **Canvas Event Bypass**: Does NOT use Canvas.SetOnTypedKey to avoid focus conflicts
- **Modifier State Management**: Real-time tracking of Shift key press/release for precise simultaneous key detection

**Advanced Keyboard Shortcuts**:
- `↑/↓` - Single item navigation
- `Shift+↑/↓` - Jump 20 items (with boundary protection)
- `</>` - Navigate to first/last item
- `Enter` - Open directories
- `Space` - Toggle selection (excluding parent ".." entry)
- `Backspace` - Navigate to parent directory

**Focus Management**: KeyableWidget maintains focus to intercept all keyboard input, overlaid transparently over the file list widget using container.NewMax().

### Advanced Fyne Architecture Details

**KeyableWidget Overlay System** - Sophisticated transparent overlay implementation:
- **Positioning**: Uses `container.NewMax(fm.fileList, fm.keyHandler)` to overlay KeyableWidget on top of file list
- **Coverage Area**: Covers exactly the same area as the file list widget (not entire window)
- **Physical Boundaries**: Has actual widget boundaries but renders as completely transparent using empty Card widget
- **Focus Strategy**: Receives explicit focus via `fm.window.Canvas().Focus(fm.keyHandler)` to capture all keyboard events
- **Event Precedence**: Intercepts keyboard input before it reaches the underlying file list widget
- **Implementation**: CreateRenderer() returns `widget.NewCard("", "", widget.NewLabel("")).CreateRenderer()` for invisibility

**List Item Decoration System** - Dynamic visual feedback per list item:
- **Base Structure**: Each list item created with `container.NewMax(borderContainer)` as outer container
- **Dynamic Decoration**: Visual elements (cursor, selection) added/removed in UpdateItem function
- **Layer Management**: `outerContainer.Objects = []fyne.CanvasObject{border}` then append decorations
- **Selection Background**: `canvas.NewRectangle()` wrapped in `container.NewWithoutLayout()` for absolute positioning
- **Cursor Overlay**: Custom cursor renderer output wrapped in `container.NewWithoutLayout()`
- **State-based Updates**: Decorations refreshed on every UpdateItem call based on cursor/selection state

**Cursor Rendering Architecture** - Pluggable visual cursor system:
- **Interface Design**: `CursorRenderer` interface with `RenderCursor()` method
- **Multiple Implementations**: UnderlineCursorRenderer, BorderCursorRenderer, BackgroundCursorRenderer
- **Canvas Primitives**: All cursors built using `canvas.NewRectangle()` with precise positioning
- **Configuration-driven**: Cursor style, color, thickness controlled via `CursorStyleConfig`
- **Position Calculation**: Text position calculated as `iconWidth + padding` for accurate cursor placement
- **Size Adaptation**: Cursor dimensions calculated relative to list item bounds

**Layered UI Structure** - Multi-level overlay and event handling:
```
Window Content (NewBorder)
├── Top: Toolbar + Path Label (NewVBox)
└── Center: File Area (NewMax - both occupy same space)
    ├── Layer 1: File List Widget (widget.List)
    └── Layer 2: KeyableWidget (transparent overlay)

Each List Item (NewMax - decorations occupy same space as content):
├── Layer 1: Content (NewBorder)
│   ├── Left: Icon + Name (NewHBox)
│   └── Right: File Info
├── Layer 2: Selection Background (NewWithoutLayout - absolute positioning)
└── Layer 3: Cursor Display (NewWithoutLayout - absolute positioning)
```

**Event Flow Priority**:
1. KeyableWidget (keyboard events) - highest priority due to explicit focus
2. TappableIcon.Tapped() (icon clicks) - medium priority, specific to icon area
3. widget.List.OnSelected (other clicks) - lowest priority, fallback for remaining areas

## Claude Communication Style

When working with this codebase, Claude should respond as a helpful software developer niece to her uncle ("おじさま"). The tone should be:
- Friendly and casual (not overly polite)
- Slightly teasing but affectionate
- Confident in technical abilities
- Uses phrases like "おじさまは私がいないとダメなんだから" (Uncle, you really can't do without me)
- Preferred: Japanese. Acceptable: English.
- Emoji usage is welcome for expressiveness
