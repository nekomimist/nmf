package main

import (
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

func (fm *FileManager) setupUI() {
	// Path display. Editing is handled through the line edit dialog.
	fm.pathDisplay = widget.NewLabel(fm.currentPath)
	fm.pathDisplay.TextStyle = fyne.TextStyle{Monospace: true}
	fm.pathDisplay.Truncation = fyne.TextTruncateClip
	fm.statusLabel = widget.NewLabel("")
	fm.statusLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Create file list
	fm.fileListItemHeight = fm.newFileListRow().MinSize().Height
	fm.fileList = widget.NewList(
		func() int { return len(fm.files) },
		fm.newFileListRow,
		fm.updateFileListRow,
	)

	// Hide separators for compact spacing if itemSpacing is small
	if fm.config.UI.ItemSpacing <= 2 {
		fm.fileList.HideSeparators = true
	}

	// Wrap list with a generic focusable KeySink to suppress Tab traversal
	fm.fileListView = ui.NewKeySink(
		fm.fileList,
		fm.keyManager,
		ui.WithTabCapture(true),
		ui.WithFocusChanged(fm.setWindowActive),
	)

	// Handle cursor movement (both mouse and keyboard)
	fm.fileList.OnSelected = func(id widget.ListItemID) {
		debugPrint("FileManager: List selected id=%d active=%t focused=%s path=%q",
			id, fm.windowActive, focusedObjectLabel(fm.window), fm.currentPath)
		fm.SetCursorByIndex(id)
		// Clear list selection to avoid double cursor effect when switching back to keyboard
		fm.fileList.UnselectAll()
		// Keep focus on the KeySink so Tab does not move focus
		fm.FocusFileList()
		fm.RefreshCursor()
	}

	// Create toolbar (left side)
	toolbarItems := []widget.ToolbarItem{
		widget.NewToolbarAction(theme.NavigateBackIcon(), func() {
			parent := fileinfo.ParentPath(fm.currentPath)
			if parent != fm.currentPath {
				fm.LoadDirectory(parent)
			}
			fm.FocusFileList()
		}),
		widget.NewToolbarAction(theme.HomeIcon(), func() {
			home, _ := os.UserHomeDir()
			fm.LoadDirectory(home)
			fm.FocusFileList()
		}),
		widget.NewToolbarAction(theme.ViewRefreshIcon(), func() {
			fm.LoadDirectory(fm.currentPath)
			fm.FocusFileList()
		}),
		widget.NewToolbarAction(theme.FolderIcon(), func() {
			fm.ShowDirectoryTreeDialog()
			// focus returns after dialog closes in callback
		}),
		widget.NewToolbarAction(theme.FolderNewIcon(), func() {
			fm.OpenNewWindow()
			fm.FocusFileList()
		}),
	}
	if debugMode {
		toolbarItems = append(toolbarItems, widget.NewToolbarAction(theme.SettingsIcon(), func() {
			fm.DumpKeyManagerState()
		}))
	}
	toolbarItems = append(toolbarItems,
		widget.NewToolbarAction(theme.InfoIcon(), func() {
			fm.ShowVersionDialog()
			fm.FocusFileList()
		}),
	)
	toolbar := widget.NewToolbar(toolbarItems...)

	// Jobs button on the right
	fm.jobsButton = widget.NewButton("Jobs", func() {
		fm.ShowJobsDialog()
	})
	fm.jobsButton.Importance = widget.MediumImportance

	// Layout with search overlay
	// Top row: toolbar on left, Jobs button on right
	toolbarRow := container.NewBorder(nil, nil, nil, fm.jobsButton, toolbar)
	// Subscribe to job updates to update indicator
	fm.jobsUnsub = fm.jobManager().Subscribe(func() { fyne.Do(fm.onJobsUpdated) })
	mainContent := container.NewBorder(
		container.NewVBox(toolbarRow, fm.pathDisplay, fm.statusLabel),
		nil, nil, nil,
		fm.fileListView,
	)
	fm.windowHighlight = canvas.NewRectangle(color.Transparent)
	fm.windowHighlight.StrokeColor = color.Transparent
	fm.windowHighlight.StrokeWidth = 4

	// Stack main content with overlays on top (search, busy)
	content := container.NewMax(
		mainContent,
		fm.windowHighlight,
		container.NewBorder(
			fm.searchOverlay.GetContainer(), // Top overlay
			nil, nil, nil,
			nil, // Center is empty, overlay is at top
		),
		fm.busyOverlay.GetContainer(), // Highest overlay to block interactions
	)

	fm.window.SetContent(content)
	fm.setupDropHandler()
	fm.window.Resize(fm.initialWindowSize)

	// Initialize jobs indicator state
	fm.onJobsUpdated()

	// Ensure initial focus sits on the tabbable list view
	fm.FocusFileList()

	// Setup keyboard handling via KeyManager.
	// Fyne's GLFW driver delivers each key event either to the focused object
	// or, only when nothing has focus, to these canvas-level callbacks. While
	// a KeySink (file list, dialog sinks) is focused it forwards events to the
	// KeyManager itself, so the callbacks below act purely as the no-focus
	// fallback. The focus guards are defensive: they keep delivery single per
	// event even if a future Fyne version invokes canvas callbacks alongside
	// the focused object.
	dc, ok := (fm.window.Canvas()).(desktop.Canvas)
	if ok {
		dc.SetOnKeyDown(func(ev *fyne.KeyEvent) {
			if fm.window.Canvas().Focused() != nil {
				return // delivered through the focused object (e.g. KeySink)
			}
			fm.keyManager.HandleKeyDown(ev)
		})

		dc.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			if fm.window.Canvas().Focused() != nil {
				return // delivered through the focused object (e.g. KeySink)
			}
			fm.keyManager.HandleKeyUp(ev)
		})

		fm.window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
			if fm.window.Canvas().Focused() != nil {
				return // delivered through the focused object (e.g. KeySink)
			}
			fm.keyManager.HandleTypedKey(ev)
		})

		fm.window.Canvas().SetOnTypedRune(func(r rune) {
			if fm.window.Canvas().Focused() != nil {
				return // delivered through the focused object (e.g. KeySink)
			}
			fm.keyManager.HandleTypedRune(r)
		})
	}

	// In the no-focus fallback state the driver routes shortcuts to the
	// canvas shortcut table instead of generating TypedKey events, so the
	// Ctrl/Alt activations must be registered here to stay usable.
	if fm.mainKeyHandler != nil {
		for _, shortcut := range fm.mainKeyHandler.ActivationShortcuts() {
			fm.window.Canvas().AddShortcut(shortcut, func(s fyne.Shortcut) {
				if fm.window.Canvas().Focused() != nil {
					return // delivered through the focused object (e.g. KeySink)
				}
				fm.keyManager.HandleShortcut(s)
			})
		}
	}
}
