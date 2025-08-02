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
- Path-based selection state (selectedFiles map[string]bool)
- Path-based cursor position (cursorPath string)
- Data binding for the file list
- DirectoryWatcher for incremental change detection

**FileInfo struct** - Extended file/directory metadata including:
- Basic attributes: name, path, type, size, modification time
- **Status field**: Normal/Added/Deleted/Modified for change tracking

**DirectoryWatcher struct** - Incremental change detection system:
- Compares filesystem snapshots to detect added/deleted/modified files
- Thread-safe communication via channels (changeChan, stopChan)
- Maintains previous file state for differential analysis

### Key Features

**Incremental Directory Watching** - Advanced change detection system:
- **Non-destructive updates**: Added files appear at list end, deleted files are grayed out
- **State preservation**: Selections and cursor position maintained during updates
- **Visual feedback**: Color-coded status (green=added, gray=deleted, orange=modified)
- **Thread-safe**: Channel-based communication between background watcher and UI

**Path-based State Management** - Robust state tracking:
- **Cursor position**: Stored as file path instead of index for persistence
- **File selections**: Path-based mapping survives directory changes
- **Auto-cleanup**: Deleted files automatically removed from selections

**Advanced Keyboard Navigation** - Full keyboard support:
- Arrow keys for navigation (↑/↓)
- Shift+Arrow for fast navigation (20 items at once)
- `</>` keys for first/last item navigation
- Enter to open directories
- Space to toggle file selection
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
- Dynamic color coding based on file status

### Data Flow

1. **Application Start**: Current working directory loaded, DirectoryWatcher initialized
2. **Initial Load**: `loadDirectory()` reads filesystem and creates FileInfo structs with StatusNormal
3. **Data Binding**: FileInfo data bound to UI list widget with automatic refresh
4. **Background Monitoring**: DirectoryWatcher polls filesystem every 2 seconds
5. **Change Detection**: Compares current filesystem state with previous snapshot
6. **Incremental Updates**: 
   - **Added files**: Appended to list with StatusAdded (green)
   - **Deleted files**: Status changed to StatusDeleted (gray + ⊠ symbol)
   - **Modified files**: Status updated to StatusModified (orange)
7. **UI Auto-refresh**: Fyne data binding automatically updates UI when changes applied
8. **User Interactions**:
   - Icon clicks: Direct navigation via TappableIcon.onTapped
   - Name/info clicks: Selection via OnSelected callback  
   - Keyboard: Navigation via KeyableWidget event handlers
9. **Directory Navigation**: 
   - DirectoryWatcher stops → `loadDirectory()` → DirectoryWatcher restarts
   - All file statuses reset to StatusNormal in new directory

### Dependencies

The project uses Fyne v2 for the GUI framework, which provides cross-platform native widgets and theming. The dark theme is set by default in main().

### File Structure

Single-file application (`main.go`) containing all functionality. The compiled binary is named `nmf`.

### Important Implementation Notes

**DirectoryWatcher Thread Safety** - Advanced goroutine coordination:
- **Dual Goroutine Architecture**: Separate goroutines for filesystem monitoring and change processing
- **Channel Communication**: Uses buffered `changeChan` for thread-safe data transfer
- **Lifecycle Management**: `Start()`/`Stop()` methods with proper channel cleanup and recreation
- **Resource Safety**: Ticker references captured locally to prevent nil pointer dereference
- **State Tracking**: `stopped` flag prevents double-stop and enables safe restart

**Path-based State Management** - Persistent state across changes:
- **Cursor Tracking**: `cursorPath` string instead of `cursorIdx` int for persistence
- **Selection Persistence**: `selectedFiles map[string]bool` survives file additions/deletions
- **Helper Functions**: `getCurrentCursorIndex()`, `setCursorByIndex()` for path↔index conversion
- **Auto-cleanup**: Deleted files automatically removed from selection map

**FileStatus Visual System** - Dynamic file appearance:
- **ColoredTextSegment**: Custom RichText segment supporting colors and strikethrough
- **Status-based Colors**: Green (added), Gray (deleted), Orange (modified), Normal (file-type based)
- **Visual Indicators**: Deleted files show ⊠ prefix for clear visual feedback
- **Color Precedence**: Status colors override file-type colors

**Fyne Data Binding Integration** - Automatic UI synchronization:
- **Auto-refresh**: `fileBinding.Set()` automatically triggers UI updates
- **Thread-safe Updates**: Data binding handles cross-thread UI refresh safely
- **Incremental Updates**: Only changed data triggers binding refresh, not entire list rebuild

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

## Debug Output Guidelines

**Debug Mode System** - The application supports debug output control via command line flags:
- **Debug Flag**: Use `-d` flag to enable debug mode when running the application
- **Debug Function**: Always use `debugPrint(format, args...)` instead of direct `log.Printf()` for development/debugging messages
- **Debug vs Regular Logs**: 
  - Use `debugPrint()` for: development traces, state changes, user interaction debugging, performance monitoring
  - Use regular `log.Printf()` for: actual errors, critical warnings, permanent operational messages

**Implementation Pattern**:
```go
// Good - conditional debug output
debugPrint("Cursor moved to index %d", newIndex)

// Bad - always visible debug output  
log.Printf("DEBUG: Cursor moved to index %d", newIndex)
```

This ensures clean output during normal usage while preserving detailed logging capabilities for development and troubleshooting.

## Real-time File Change Features

**Incremental File Monitoring** - The application now features advanced real-time file change detection:

**Visual Status Indicators**:
- **Green files**: Newly added files appear in bright green at the bottom of the list
- **Gray files with ⊠**: Deleted files are grayed out and marked with ⊠ symbol
- **Orange files**: Modified files (size/timestamp changes) appear in orange
- **Manual refresh**: Use toolbar refresh button to clear all status colors and return to normal view

**State Preservation During Changes**:
- **Selections maintained**: Files you've marked (with Space key) remain selected even when other files are added/deleted
- **Cursor position preserved**: Your current position in the list is maintained during directory updates
- **Auto-cleanup**: Deleted files are automatically removed from your selection set

**Usage Scenarios**:
- **File operations**: Continue working while background processes add/remove files
- **Build monitoring**: Watch compilation outputs appear in real-time without losing your place
- **Download tracking**: See new downloads appear immediately while maintaining your current navigation context

**Performance Notes**:
- **2-second polling**: Directory changes detected every 2 seconds
- **Incremental updates**: Only changed files trigger UI updates, not entire directory refresh
- **Thread-safe**: All background monitoring is safely isolated from UI operations

## Claude Communication Style

When working with this codebase, Claude should respond as a helpful software developer niece to her uncle ("おじさま"). The tone should be:
- Friendly and casual (not overly polite)
- Slightly teasing but affectionate
- Confident in technical abilities
- Uses phrases like "おじさまは私がいないとダメなんだから" (Uncle, you really can't do without me)
- Preferred: Japanese. Acceptable: English.
- Emoji usage is welcome for expressiveness
