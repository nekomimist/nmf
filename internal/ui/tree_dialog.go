package ui

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

// DirectoryTreeDialog represents a directory tree navigation dialog
type DirectoryTreeDialog struct {
	tree         *widget.Tree
	currentRoot  string                                   // Current root path: "/" or parent directory
	selectedPath string                                   // Currently selected directory path
	parentPath   string                                   // Parent directory of current FileManager path
	rootMode     bool                                     // true = filesystem root "/", false = parent directory
	debugPrint   func(format string, args ...interface{}) // Debug function
	keyManager   *keymanager.KeyManager                   // Keyboard input manager
	dialog       dialog.Dialog                            // Reference to the actual dialog
	callback     func(string)                             // Callback function for selection
}

// NewDirectoryTreeDialog creates a new directory tree dialog
func NewDirectoryTreeDialog(currentPath string, keyManager *keymanager.KeyManager, debugPrint func(format string, args ...interface{})) *DirectoryTreeDialog {
	parentPath := filepath.Dir(currentPath)

	dialog := &DirectoryTreeDialog{
		selectedPath: currentPath,
		parentPath:   parentPath,
		rootMode:     true, // Start with filesystem root
		currentRoot:  "/",
		debugPrint:   debugPrint,
		keyManager:   keyManager,
	}

	dialog.createTree()
	return dialog
}

// createTree creates the tree widget with lazy loading
func (dtd *DirectoryTreeDialog) createTree() {
	dtd.tree = widget.NewTree(
		// childUIDs function - returns child directories
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			children := dtd.getDirectoryChildren(string(uid))
			result := make([]widget.TreeNodeID, len(children))
			for i, child := range children {
				result[i] = widget.TreeNodeID(child)
			}
			return result
		},
		// isBranch function - all paths are branches (directories)
		func(uid widget.TreeNodeID) bool {
			return dtd.isDirectory(string(uid))
		},
		// create function - creates node UI
		func(branch bool) fyne.CanvasObject {
			icon := widget.NewIcon(theme.FolderIcon())
			icon.Resize(fyne.NewSize(16, 16))
			label := widget.NewLabel("Directory")
			return container.NewHBox(icon, label)
		},
		// update function - updates node UI with directory name
		func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			hbox := obj.(*fyne.Container)
			if len(hbox.Objects) >= 2 {
				if label, ok := hbox.Objects[1].(*widget.Label); ok {
					label.SetText(dtd.getDisplayName(string(uid)))
				}
			}
		},
	)

	// Set selection handler
	dtd.tree.OnSelected = func(uid widget.TreeNodeID) {
		dtd.selectedPath = string(uid)
		dtd.debugPrint("Directory selected: %s", uid)
	}

	// Set branch open handler for lazy loading
	dtd.tree.OnBranchOpened = func(uid widget.TreeNodeID) {
		dtd.debugPrint("Branch opened: %s", uid)
	}

	// Set initial root node
	dtd.tree.Root = widget.TreeNodeID(dtd.currentRoot)
}

// getDirectoryChildren returns child directories for lazy loading
func (dtd *DirectoryTreeDialog) getDirectoryChildren(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		dtd.debugPrint("Error reading directory %s: %v", path, err)
		return []string{}
	}

	var children []string
	for _, entry := range entries {
		if entry.IsDir() {
			childPath := filepath.Join(path, entry.Name())
			// Skip hidden directories unless they're important system ones
			if !strings.HasPrefix(entry.Name(), ".") || entry.Name() == ".." {
				children = append(children, childPath)
			}
		}
	}

	return children
}

// isDirectory checks if a path is a directory
func (dtd *DirectoryTreeDialog) isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// getDisplayName returns the display name for a directory path
func (dtd *DirectoryTreeDialog) getDisplayName(path string) string {
	if path == "/" {
		return "/"
	}
	base := filepath.Base(path)
	if base == "." {
		return filepath.Dir(path)
	}
	return base
}

// expandInitialLevel expands only the root level
func (dtd *DirectoryTreeDialog) expandInitialLevel() {
	// Set the root node first
	dtd.tree.Root = widget.TreeNodeID(dtd.currentRoot)

	// Only expand the root level to show first-level directories
	dtd.tree.OpenBranch(widget.TreeNodeID(dtd.currentRoot))
}

// ShowDialog shows the directory tree dialog
func (dtd *DirectoryTreeDialog) ShowDialog(parent fyne.Window, callback func(string)) {
	// Create radio group for root selection
	rootOptions := []string{"Root /", "Parent Dir"}
	var selectedOption string
	if dtd.rootMode {
		selectedOption = "Root /"
	} else {
		selectedOption = "Parent Dir"
	}

	radioGroup := widget.NewRadioGroup(rootOptions, func(selected string) {
		var newRootMode bool
		var newCurrentRoot string

		switch selected {
		case "Root /":
			newRootMode = true
			newCurrentRoot = "/"
		case "Parent Dir":
			newRootMode = false
			newCurrentRoot = dtd.parentPath
		}

		// Only update if the mode is actually changing
		if dtd.rootMode != newRootMode {
			dtd.rootMode = newRootMode
			dtd.currentRoot = newCurrentRoot

			// Refresh tree with new root
			dtd.tree.Refresh()

			// Expand the root level initially
			dtd.expandInitialLevel()
		}
	})
	radioGroup.SetSelected(selectedOption)
	radioGroup.Horizontal = true

	// Create top control panel
	buttonPanel := container.NewHBox(radioGroup)

	// Set tree size and minimum size
	dtd.tree.Resize(fyne.NewSize(500, 400))

	// Create scrollable container for the tree
	treeScroll := container.NewScroll(dtd.tree)
	treeScroll.SetMinSize(fyne.NewSize(500, 400))

	// Create main content
	content := container.NewBorder(
		buttonPanel, // top
		nil,         // bottom
		nil,         // left
		nil,         // right
		treeScroll,  // center
	)

	// Set minimum size for the entire content
	content.Resize(fyne.NewSize(550, 500))

	// Expand initial level
	dtd.expandInitialLevel()

	// Store callback for use by key handler
	dtd.callback = callback

	// Create and push tree dialog key handler
	treeHandler := keymanager.NewTreeDialogKeyHandler(dtd, dtd.debugPrint)
	dtd.keyManager.PushHandler(treeHandler)

	// Create dialog with custom content
	dtd.dialog = dialog.NewCustomConfirm(
		"Select Directory",
		"OK",
		"Cancel",
		content,
		func(response bool) {
			// Pop the tree handler when dialog closes
			dtd.keyManager.PopHandler()

			// ダイアログが閉じられたときにメインウィンドウのフォーカスを解除
			parent.Canvas().Unfocus()

			if response && dtd.selectedPath != "" {
				callback(dtd.selectedPath)
			}
		},
		parent,
	)

	// Show the dialog
	dtd.dialog.Show()

	// ダイアログ表示後、ツリーにフォーカスを当てる
	parent.Canvas().Focus(dtd.tree)
}

// TreeDialogInterface implementation methods

// MoveUp moves the selection up in the tree
func (dtd *DirectoryTreeDialog) MoveUp() {
	// Implementation would depend on the tree widget's current selection
	// For now, this is a placeholder
	dtd.debugPrint("TreeDialog: Move up")
}

// MoveDown moves the selection down in the tree
func (dtd *DirectoryTreeDialog) MoveDown() {
	dtd.debugPrint("TreeDialog: Move down")
}

// ExpandNode expands the currently selected node
func (dtd *DirectoryTreeDialog) ExpandNode() {
	dtd.debugPrint("TreeDialog: Expand node")
}

// CollapseNode collapses the currently selected node
func (dtd *DirectoryTreeDialog) CollapseNode() {
	dtd.debugPrint("TreeDialog: Collapse node")
}

// SelectCurrentNode selects the current node
func (dtd *DirectoryTreeDialog) SelectCurrentNode() {
	dtd.debugPrint("TreeDialog: Select current node")
}

// AcceptSelection accepts the current selection and closes the dialog
func (dtd *DirectoryTreeDialog) AcceptSelection() {
	// Pop the handler first
	dtd.keyManager.PopHandler()

	if dtd.callback != nil && dtd.selectedPath != "" {
		dtd.callback(dtd.selectedPath)
	}
	if dtd.dialog != nil {
		dtd.dialog.Hide()
	}
}

// CancelDialog cancels the dialog without selection
func (dtd *DirectoryTreeDialog) CancelDialog() {
	// Pop the handler first
	dtd.keyManager.PopHandler()

	if dtd.dialog != nil {
		dtd.dialog.Hide()
	}
}

// ToggleRootMode toggles between root filesystem and parent directory mode
func (dtd *DirectoryTreeDialog) ToggleRootMode() {
	dtd.rootMode = !dtd.rootMode
	if dtd.rootMode {
		dtd.currentRoot = "/"
	} else {
		dtd.currentRoot = dtd.parentPath
	}

	// Refresh tree with new root
	dtd.tree.Refresh()
	dtd.expandInitialLevel()

	dtd.debugPrint("TreeDialog: Toggled root mode to %t (root: %s)", dtd.rootMode, dtd.currentRoot)
}
