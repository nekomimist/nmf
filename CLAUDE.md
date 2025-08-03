# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a GUI file manager application called "nmf" built in Go using the Fyne framework (v2.6.1). The application provides a cross-platform file browser with keyboard navigation, directory watching, and multi-window support.

## Common Commands

### Build and Run
- `go run .` - Compile and run the file manager application
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

**DirectoryTreeDialog struct** - Tree-based directory navigation dialog:
- Lazy loading tree widget for efficient directory browsing
- Root toggle between filesystem root ("/") and parent directory
- Focus management for seamless keyboard interaction
- Custom tree rendering with folder icons and directory names

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

**Advanced Keyboard Navigation** - Full keyboard support with key repeat:
- Arrow keys for navigation (↑/↓) with OS key repeat support
- Shift+Arrow for fast navigation (20 items at once)
- `Shift+,/Shift+.` keys for first/last item navigation (physical key detection)
- Enter to open directories
- Space to toggle file selection
- Backspace to go up one directory level

**Icon-Click Navigation** - Custom TappableIcon implementation with restored mouse processing:
- Clicking on file/directory icons directly navigates into directories
- Clicking on file names or file information only selects the item
- **Mouse Processing Restored**: Elimination of transparent overlay widgets enables full mouse functionality
- This provides intuitive mouse navigation while preserving selection behavior
- Implementation uses a custom TappableIcon widget that extends BaseWidget

**Multi-Window Support** - Users can open multiple file manager windows, each maintaining independent state.

**Tree Navigation Dialog** - Advanced directory selection system:
- **Lazy Loading**: Only loads directory children when needed for optimal performance
- **Root Toggle**: Switch between filesystem root ("/") and parent directory as tree root
- **Dual Access**: Toolbar button (📁 folder icon) and keyboard shortcut (Ctrl+T)
- **Focus Management**: Automatically focuses tree widget when dialog opens, returns focus when closed
- **Tree Widget Integration**: Uses Fyne's Tree widget with custom childUIDs, isBranch, create, and update functions
- **Visual Feedback**: Folder icons with directory names, expandable branches
- **Initial Expansion**: Shows only root level directories initially, expandable on-demand

**UI Layout** - Uses Fyne's container system:
- Toolbar with back/home/refresh/tree dialog/new window actions
- Path entry for direct path input and navigation
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
   - Keyboard: Navigation via desktop.Canvas SetOnKeyDown/Up and SetOnTypedKey handlers
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

**Dual-Layer Color System** - Separated status and file type visualization:
- **Text Color Management**: `getTextColor()` returns file type-based text colors (directories=blue, symlinks=orange, etc.)
- **Background Color Management**: `getStatusBackgroundColor()` returns status-based semi-transparent backgrounds
- **Status Background Layer**: Full-width semi-transparent backgrounds for Added (green), Deleted (gray), Modified (orange)
- **ColoredTextSegment**: Simplified custom RichText segment for text color and strikethrough only
- **Visual Indicators**: Deleted files show ⊠ prefix with strikethrough effect
- **Layer Independence**: Status backgrounds and file type text colors work independently for clear visual separation

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

**Keyboard Event Handling Architecture** - Separated key processing with key repeat support:
- **desktop.Canvas Integration**: Direct canvas-level keyboard event handling without overlay widgets
- **Separated Processing**: Modifier keys (Shift) handled via SetOnKeyDown/Up, normal keys via SetOnTypedKey
- **Key Repeat Support**: SetOnTypedKey provides automatic OS-level key repeat for navigation keys
- **True Shift Detection**: Physical Shift key state tracked via KeyDown/KeyUp events
- **Mouse Processing Restored**: No invisible overlay widgets blocking mouse interactions

**Separated Key Processing Implementation**:
- **SetOnKeyDown/Up Handler**: Modifier keys only (desktop.KeyShiftLeft, desktop.KeyShiftRight)
  - Tracks `fm.shiftPressed` boolean state in real-time
  - Enables accurate simultaneous key detection (e.g., Shift+Arrow)
- **SetOnTypedKey Handler**: All functional keys with OS key repeat support
  - Arrow keys (↑↓) - automatic key repeat for smooth navigation
  - Action keys (Enter, Space, Backspace) - standard behavior
  - Combination keys (Shift+Comma for '<', Shift+Period for '>') - physical key detection
- **Key Repeat Benefits**: Holding arrow keys provides smooth, fast scrolling
- **Future Extensibility**: Alt, Ctrl modifiers can be added using same pattern

**Advanced Keyboard Shortcuts**:
- `↑/↓` - Single item navigation
- `Shift+↑/↓` - Jump 20 items (with boundary protection)
- `Shift+,/Shift+.` - Navigate to first/last item (physical key detection)
- `Enter` - Open directories
- `Space` - Toggle selection (excluding parent ".." entry)
- `Backspace` - Navigate to parent directory
- `Ctrl+T` - Open directory tree navigation dialog
- `Ctrl+N` - Open new file manager window

**Direct Canvas Events**: Keyboard events are handled directly at the canvas level without requiring focus management or widget overlays.

### Advanced Fyne Architecture Details

**Simplified UI Architecture** - Direct widget hierarchy without overlays:
- **Single Layer Structure**: File list widget directly in container without invisible overlays
- **Canvas-Level Events**: Keyboard handling at desktop.Canvas level without widget-level interception
- **Native Mouse Handling**: TappableIcon and List.OnSelected work without interference
- **Reduced Complexity**: Eliminated transparent widget overlays and focus management complexity

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
- **Canvas Primitives**: All cursors built using `canvas.NewRectangle()` with full-width positioning
- **Configuration-driven**: Cursor style, color, thickness controlled via `CursorStyleConfig`
- **Unified Positioning**: All cursor types cover entire list item width for visual consistency
- **Simplified Implementation**: Underline cursor renders as full-width line at bottom edge

**Simplified UI Structure** - Clean hierarchy without overlays:
```
Window Content (NewBorder)
├── Top: Toolbar + Path Entry (NewVBox)
└── Center: File List Widget (widget.List) - direct placement

Each List Item (NewMax - decorations occupy same space as content):
├── Layer 1: Content (NewBorder)
│   ├── Left: Icon + Name (NewHBox)
│   └── Right: File Info
├── Layer 2: Status Background (NewWithoutLayout - absolute positioning, full-width)
├── Layer 3: Selection Background (NewWithoutLayout - absolute positioning, full-width)
└── Layer 4: Cursor Display (NewWithoutLayout - absolute positioning, full-width)
```

**Event Flow Priority**:
1. desktop.Canvas keyboard events (SetOnKeyDown/Up, SetOnTypedKey) - highest priority, canvas-level
2. TappableIcon.Tapped() (icon clicks) - medium priority, specific to icon area  
3. widget.List.OnSelected (other clicks) - lowest priority, fallback for remaining areas

**Decoration Layer Rendering Order** (bottom to top):
1. **Content Layer**: File information (icon, name, size, date)
2. **Status Background**: Semi-transparent colored backgrounds for file status changes
3. **Selection Background**: Blue background for selected files  
4. **Cursor Layer**: Visual cursor indicator (underline, border, or background style)

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
- **Green background**: Newly added files have semi-transparent green background covering entire row
- **Gray background with ⊠**: Deleted files have semi-transparent gray background with ⊠ symbol prefix
- **Orange background**: Modified files have semi-transparent orange background covering entire row
- **File type colors**: Text color reflects file type (directories=blue, symlinks=orange, hidden=gray, regular=light gray)
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

## Directory Tree Navigation Dialog

**Implementation Architecture** - Complete tree-based navigation system:

**Core Components**:
- **DirectoryTreeDialog struct**: Main dialog controller with tree state management
- **Tree Widget**: Fyne widget.Tree with custom lazy loading functions
- **Root Management**: Dynamic root switching between "/" and parent directory
- **Focus Control**: Automatic focus management for seamless user interaction

**Key Implementation Details**:
- **Lazy Loading Functions**: 
  - `childUIDs(TreeNodeID) []TreeNodeID`: Returns child directory paths as TreeNodeID
  - `isBranch(TreeNodeID) bool`: Determines if a path is a directory
  - `create(bool) CanvasObject`: Creates folder icon + label UI
  - `update(TreeNodeID, bool, CanvasObject)`: Updates UI with directory name
- **Event Handlers**:
  - `OnSelected`: Updates selected path for navigation
  - `OnBranchOpened`: Debug logging for tree expansion
- **UI Structure**: Border container with toggle buttons (top) and scrollable tree (center)
- **Size Management**: Fixed 500x400 tree size with scrollable container for large directory trees

**User Experience**:
- **Access Methods**: Toolbar button (📁) or Ctrl+T keyboard shortcut
- **Root Toggle**: "Root /" and "Parent Dir" buttons with visual importance indicators
- **Navigation**: Click directory to select, OK button to navigate, Cancel to abort
- **Focus Flow**: Tree gets focus on open, main window unfocused on close

**Integration Points**:
- **Toolbar Integration**: Added folder icon button between refresh and new window actions
- **Keyboard Integration**: Ctrl+T handled in SetOnKeyDown for proper modifier detection
- **Navigation Callback**: Selected directory passed to FileManager.loadDirectory()

## Claude Communication Style

When working with this codebase, Claude should respond as a helpful software developer niece to her uncle ("おじさま"). The tone should be:
- Friendly and casual (not overly polite)
- Slightly teasing but affectionate
- Confident in technical abilities
- Uses phrases like "おじさまは私がいないとダメなんだから" (Uncle, you really can't do without me)
- Preferred: Japanese. Acceptable: English.
- Emoji usage is welcome for expressiveness
