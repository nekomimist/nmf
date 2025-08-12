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
	parent       fyne.Window                              // Parent window for focus management
	closed       bool                                     // Prevent double-close/pop
	sink         *KeySink                                 // Key capturing wrapper
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
		// Keep focus on sink so KeyManager continues to receive keys
		if dtd.parent != nil && dtd.sink != nil {
			dtd.parent.Canvas().Focus(dtd.sink)
		}
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

// getVisibleNodes returns all currently visible nodes in the tree
func (dtd *DirectoryTreeDialog) getVisibleNodes() []widget.TreeNodeID {
	var visibleNodes []widget.TreeNodeID
	dtd.collectVisibleNodes(widget.TreeNodeID(dtd.currentRoot), &visibleNodes)
	return visibleNodes
}

// collectVisibleNodes recursively collects all visible nodes from the tree
func (dtd *DirectoryTreeDialog) collectVisibleNodes(nodeID widget.TreeNodeID, visibleNodes *[]widget.TreeNodeID) {
	*visibleNodes = append(*visibleNodes, nodeID)

	// If this branch is open, collect its children
	if dtd.tree.IsBranchOpen(nodeID) {
		children := dtd.getDirectoryChildren(string(nodeID))
		for _, child := range children {
			childID := widget.TreeNodeID(child)
			dtd.collectVisibleNodes(childID, visibleNodes)
		}
	}
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
	dtd.parent = parent

	// Create and push tree dialog key handler
	treeHandler := keymanager.NewTreeDialogKeyHandler(dtd, dtd.debugPrint)
	dtd.keyManager.PushHandler(treeHandler)

	// Wrap content with KeySink to capture Tab and forward keys
	dtd.sink = NewKeySink(content, dtd.keyManager, WithTabCapture(true))

	// Create dialog with custom content (wrapped by sink)
	dtd.dialog = dialog.NewCustomConfirm(
		"Select Directory",
		"OK",
		"Cancel",
		dtd.sink,
		func(response bool) {
			if response {
				dtd.AcceptSelection()
			} else {
				dtd.CancelDialog()
			}
		},
		parent,
	)

	// Show the dialog
	dtd.dialog.Show()

	// Set initial selection to show cursor (don't focus the tree widget)
	dtd.tree.Select(widget.TreeNodeID(dtd.currentRoot))
	dtd.selectedPath = dtd.currentRoot

	// Ensure focus stays on sink so keys route to KeyManager
	if dtd.parent != nil && dtd.sink != nil {
		dtd.parent.Canvas().Focus(dtd.sink)
	}
}

// TreeDialogInterface implementation methods

// MoveUp moves the selection up in the tree
func (dtd *DirectoryTreeDialog) MoveUp() {
	visibleNodes := dtd.getVisibleNodes()
	if len(visibleNodes) == 0 {
		return
	}

	currentIndex := -1
	for i, node := range visibleNodes {
		if string(node) == dtd.selectedPath {
			currentIndex = i
			break
		}
	}

	if currentIndex > 0 {
		newSelection := visibleNodes[currentIndex-1]
		dtd.tree.Select(newSelection)
		dtd.selectedPath = string(newSelection)
		dtd.debugPrint("TreeDialog: Move up to %s", newSelection)
	}
}

// MoveDown moves the selection down in the tree
func (dtd *DirectoryTreeDialog) MoveDown() {
	visibleNodes := dtd.getVisibleNodes()
	if len(visibleNodes) == 0 {
		return
	}

	currentIndex := -1
	for i, node := range visibleNodes {
		if string(node) == dtd.selectedPath {
			currentIndex = i
			break
		}
	}

	if currentIndex >= 0 && currentIndex < len(visibleNodes)-1 {
		newSelection := visibleNodes[currentIndex+1]
		dtd.tree.Select(newSelection)
		dtd.selectedPath = string(newSelection)
		dtd.debugPrint("TreeDialog: Move down to %s", newSelection)
	}
}

// ExpandNode expands the currently selected node
func (dtd *DirectoryTreeDialog) ExpandNode() {
	if dtd.selectedPath == "" {
		return
	}

	nodeID := widget.TreeNodeID(dtd.selectedPath)

	// Check if it's a directory (branch)
	if dtd.isDirectory(dtd.selectedPath) {
		if !dtd.tree.IsBranchOpen(nodeID) {
			dtd.tree.OpenBranch(nodeID)
			dtd.debugPrint("TreeDialog: Expanded node %s", dtd.selectedPath)
		} else {
			// If already expanded, move to first child if available
			children := dtd.getDirectoryChildren(dtd.selectedPath)
			if len(children) > 0 {
				firstChild := widget.TreeNodeID(children[0])
				dtd.tree.Select(firstChild)
				dtd.selectedPath = children[0]
				dtd.debugPrint("TreeDialog: Moved to first child %s", children[0])
			}
		}
	}
}

// CollapseNode collapses the currently selected node
func (dtd *DirectoryTreeDialog) CollapseNode() {
	if dtd.selectedPath == "" {
		return
	}

	nodeID := widget.TreeNodeID(dtd.selectedPath)

	// If current node is expanded, collapse it
	if dtd.tree.IsBranchOpen(nodeID) {
		dtd.tree.CloseBranch(nodeID)
		dtd.debugPrint("TreeDialog: Collapsed node %s", dtd.selectedPath)
	} else {
		// If not expanded, move to parent
		parentPath := filepath.Dir(dtd.selectedPath)
		if parentPath != dtd.selectedPath && parentPath != "." {
			parentID := widget.TreeNodeID(parentPath)
			dtd.tree.Select(parentID)
			dtd.selectedPath = parentPath
			dtd.debugPrint("TreeDialog: Moved to parent %s", parentPath)
		}
	}
}

// SelectCurrentNode selects the current node (mainly for Space key handling)
func (dtd *DirectoryTreeDialog) SelectCurrentNode() {
	if dtd.selectedPath != "" {
		nodeID := widget.TreeNodeID(dtd.selectedPath)
		dtd.tree.Select(nodeID)
		dtd.debugPrint("TreeDialog: Confirmed selection of %s", dtd.selectedPath)
	}
}

// AcceptSelection accepts the current selection and closes the dialog
func (dtd *DirectoryTreeDialog) AcceptSelection() {
	if dtd.closed {
		return
	}
	dtd.closed = true
	// Pop the handler first
	dtd.keyManager.PopHandler()

	if dtd.callback != nil && dtd.selectedPath != "" {
		dtd.callback(dtd.selectedPath)
	}
	if dtd.dialog != nil {
		dtd.dialog.Hide()
	}
	// Unfocus parent window canvas if available
	if dtd.parent != nil {
		dtd.parent.Canvas().Unfocus()
	}
}

// CancelDialog cancels the dialog without selection
func (dtd *DirectoryTreeDialog) CancelDialog() {
	if dtd.closed {
		return
	}
	dtd.closed = true
	// Pop the handler first
	dtd.keyManager.PopHandler()

	if dtd.dialog != nil {
		dtd.dialog.Hide()
	}
	if dtd.parent != nil {
		dtd.parent.Canvas().Unfocus()
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
