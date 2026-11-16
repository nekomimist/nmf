package keymanager

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
)

// CommandDescribeBindings opens the effective bindings for all configurable modes.
const CommandDescribeBindings = "help.bindings"

// BindingDescription describes the winning binding for one key combination.
// Custom identifies a configured binding, including an explicit noop override.
type BindingDescription struct {
	Key         string
	Command     string
	Description string
	Custom      bool
}

func describeBindings(bindings []keyBinding, commandNames map[string]string) []BindingDescription {
	seen := make(map[keySpec]bool)
	result := make([]BindingDescription, 0, len(bindings))
	for _, binding := range bindings {
		spec := binding.spec
		spec.key = normalizeDrainKey(spec.key)
		if seen[spec] {
			continue
		}
		seen[spec] = true
		description := commandDescriptions[binding.command]
		if name := commandNames[binding.command]; name != "" {
			description = name
		}
		if description == "" {
			description = binding.command
		}
		result = append(result, BindingDescription{
			Key: formatBindingKey(spec), Command: binding.command,
			Description: description, Custom: binding.custom,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func formatBindingKey(spec keySpec) string {
	var key strings.Builder
	if spec.mod.CtrlPressed {
		key.WriteString("C-")
	}
	if spec.mod.AltPressed {
		key.WriteString("A-")
	}
	if spec.mod.ShiftPressed {
		key.WriteString("S-")
	}
	name := string(spec.key)
	switch spec.key {
	case fyne.KeyBackspace:
		name = "Backspace"
	case fyne.KeyPageUp:
		name = "PageUp"
	case fyne.KeyPageDown:
		name = "PageDown"
	case fyne.KeyBackTick:
		name = "Backtick"
	case fyne.KeySemicolon:
		name = "Semicolon"
	case fyne.KeyComma:
		name = "Comma"
	case fyne.KeyPeriod:
		name = "Period"
	}
	key.WriteString(name)
	return key.String()
}

// BindingDescriptions returns the main handler's validated effective bindings.
func (mh *MainScreenKeyHandler) BindingDescriptions(commandNames map[string]string) []BindingDescription {
	return describeBindings(mh.bindings, commandNames)
}

// LineEditBindingDescriptions uses the same resolver and command table as dispatch.
func LineEditBindingDescriptions(configured []config.KeyBindingEntry) []BindingDescription {
	return describeBindings(buildLineEditBindings(configured, func(string, ...interface{}) {}), nil)
}

// FileViewerBindingDescriptions includes both typed-key and rune-path bindings.
func FileViewerBindingDescriptions(configured []config.KeyBindingEntry) []BindingDescription {
	return describeBindings(buildFileViewerBindings(configured, func(string, ...interface{}) {}), nil)
}

// Keep descriptions keyed by stable command ID, independent of key assignments.
var commandDescriptions = map[string]string{
	CommandDescribeBindings:          "Show current key bindings",
	CommandNoop:                      "Disabled (do nothing)",
	CommandCursorUp:                  "Move cursor up",
	CommandCursorDown:                "Move cursor down",
	CommandCursorPageUp:              "Move cursor one page up",
	CommandCursorPageDown:            "Move cursor one page down",
	CommandCursorFirst:               "Move cursor to first entry",
	CommandCursorLast:                "Move cursor to last entry",
	CommandOpen:                      "Open current entry",
	CommandOpenDefaultApp:            "Open with the default application",
	CommandSelectToggle:              "Toggle mark and advance",
	CommandSelectAll:                 "Mark all files",
	CommandSelectInvert:              "Invert file marks",
	CommandSelectInvertWithDir:       "Invert marks including directories",
	CommandParentDirectory:           "Go to parent directory",
	CommandRootDirectory:             "Go to filesystem or provider root",
	CommandRefresh:                   "Refresh directory",
	CommandHome:                      "Go to home directory",
	CommandDirectoryCreate:           "Create directory",
	CommandClipboardTextFile:         "Save clipboard text to a file",
	CommandWindowNew:                 "Open a new window",
	CommandWindowReopen:              "Reopen the last closed window",
	CommandWindowFocusLeft:           "Focus the window to the left",
	CommandWindowFocusRight:          "Focus the window to the right",
	CommandWindowResetSize:           "Reset current window size",
	CommandWindowResetAllSizes:       "Reset all window sizes",
	CommandTreeShow:                  "Show directory tree",
	CommandHistoryBack:               "Go back in navigation history",
	CommandHistoryShow:               "Show navigation history",
	CommandHistoryPinCurrent:         "Pin current directory in history",
	CommandDirectoryJumpShow:         "Jump to a configured directory",
	CommandFilterShow:                "Choose or enter a file filter",
	CommandFilterClear:               "Clear file filter",
	CommandFilterToggle:              "Toggle file filter",
	CommandSearchShow:                "Start incremental search",
	CommandSortShow:                  "Choose sort order",
	CommandJobsShow:                  "Show background jobs",
	CommandPathEdit:                  "Edit current path",
	CommandQuit:                      "Close window or quit",
	CommandCopyShow:                  "Copy marked files or current entry",
	CommandMoveShow:                  "Move marked files or current entry",
	CommandArchiveExtract:            "Extract archive",
	CommandCompareShow:               "Compare directories",
	CommandRenameShow:                "Rename current entry",
	CommandDeleteTrash:               "Move marked files or current entry to trash",
	CommandDeletePermanent:           "Permanently delete marked files or current entry",
	CommandExplorerContextShow:       "Show system context menu",
	CommandExternalCommandMenu:       "Show external commands",
	CommandViewerShow:                "Open current file in the built-in viewer",
	CommandMaintenanceShow:           "Show maintenance tools",
	CommandLineEditAccept:            "Accept input",
	CommandLineEditCancel:            "Cancel input",
	CommandLineEditCursorStart:       "Move to start of input",
	CommandLineEditCursorEnd:         "Move to end of input",
	CommandLineEditCursorLeft:        "Move cursor left",
	CommandLineEditCursorRight:       "Move cursor right",
	CommandLineEditDeleteBefore:      "Delete before cursor",
	CommandLineEditDeleteAt:          "Delete at cursor",
	CommandLineEditDeleteBeforeStart: "Delete from cursor to start",
	CommandLineEditDeleteAfterEnd:    "Delete from cursor to end",
	CommandLineEditPaste:             "Paste clipboard text",
	CommandFileViewerClose:           "Close viewer",
	CommandFileViewerLineDown:        "Scroll down one displayed row",
	CommandFileViewerLineUp:          "Scroll up one displayed row",
	CommandFileViewerPageDown:        "Scroll down one page",
	CommandFileViewerPageUp:          "Scroll up one page",
	CommandFileViewerHome:            "Scroll to start",
	CommandFileViewerEnd:             "Scroll to end",
	CommandFileViewerColumnLeft:      "Scroll left or pan image left",
	CommandFileViewerColumnRight:     "Scroll right or pan image right",
	CommandFileViewerToggleWrap:      "Toggle line wrapping",
	CommandFileViewerShowImage:       "Show Image pane (images only)",
	CommandFileViewerShowText:        "Show Text pane (text files only)",
	CommandFileViewerShowMarkdown:    "Show Markdown pane (text files only)",
	CommandFileViewerShowHex:         "Show Hex pane",
	CommandFileViewerSearchNext:      "Find next match (text panes)",
	CommandFileViewerSearchPrevious:  "Find previous match (text panes)",
	CommandFileViewerFocusSearch:     "Focus search input (text files only)",
	CommandFileViewerFocusLine:       "Focus line input (text files only)",
	CommandFileViewerCopySelection:   "Copy selection (text panes)",
	CommandFileViewerSelectAll:       "Select all (text panes)",
	CommandFileViewerClearSelection:  "Clear selection (text panes)",
	CommandFileViewerImageToggleFit:  "Toggle image fit and free zoom (images only)",
	CommandFileViewerImageZoomIn:     "Zoom image in (images only)",
	CommandFileViewerImageZoomOut:    "Zoom image out (images only)",
	CommandFileViewerFilePrevious:    "Preview previous entry in the file list",
	CommandFileViewerFileNext:        "Preview next entry in the file list",
	CommandFileViewerMarkToggle:      "Toggle file mark and preview next entry",
}
