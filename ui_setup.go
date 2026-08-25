package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

func (fm *FileManager) setupUI() {
	// Path display. Editing is handled through the line edit dialog.
	fm.pathDisplay = widget.NewLabel(fm.GetCurrentPath())
	fm.pathDisplay.TextStyle = fyne.TextStyle{Monospace: true}
	fm.pathDisplay.Truncation = fyne.TextTruncateClip
	fm.statusLabel = widget.NewLabel("")
	fm.statusLabel.TextStyle = fyne.TextStyle{Monospace: true}
	// Truncate instead of wrap so the label always renders as exactly one
	// line; a status notice can contain two paths and would otherwise wrap
	// to multiple lines and shift the file list, defeating status_bar.go's
	// single-line invariant.
	fm.statusLabel.Truncation = fyne.TextTruncateClip
	fm.cacheStatusBadge = newDirectoryCacheStatusBadge(fm.customTheme)
	statusBar := container.NewStack(
		fm.statusLabel,
		fm.cacheStatusBadge.container,
	)

	// Create file list
	fm.fileListItemHeight = fm.newFileListRow().MinSize().Height
	fm.fileList = widget.NewList(
		fm.FileCount,
		fm.newFileListRow,
		fm.updateFileListRow,
	)

	// Hide separators for compact spacing if itemSpacing is small
	if fm.config.UI.ItemSpacing <= 2 {
		fm.fileList.HideSeparators = true
	}

	// Handle cursor movement (both mouse and keyboard)
	fm.fileList.OnSelected = func(id widget.ListItemID) {
		debugPrint("FileManager: List selected id=%d active=%t focused=%s path=%q",
			id, fm.windowActive, focusedObjectLabel(fm.window), fm.GetCurrentPath())
		fm.SetCursorByIndex(id)
		// Clear list selection to avoid double cursor effect when switching back to keyboard
		fm.fileList.UnselectAll()
		// Keep focus on the KeySink so Tab does not move focus
		fm.FocusFileList()
		fm.RefreshCursor()
	}

	// Fyne buttons clear canvas focus before running their callback. Restore
	// the persistent main-screen owner before the action starts so a dialog or
	// another window can return to the same KeySink instead of a nil owner.
	toolbarAction := func(icon fyne.Resource, commandID string, action func()) widget.ToolbarItem {
		return widget.NewToolbarAction(icon, fm.mainScreenPointerCommand(commandID, action))
	}

	// Create toolbar (left side)
	toolbarItems := []widget.ToolbarItem{
		toolbarAction(theme.NavigateBackIcon(), keymanager.CommandHistoryBack, fm.HistoryBack),
		toolbarAction(theme.HomeIcon(), keymanager.CommandHome, func() {
			home, _ := os.UserHomeDir()
			fm.LoadDirectory(home)
		}),
		toolbarAction(theme.ViewRefreshIcon(), keymanager.CommandRefresh, func() {
			fm.LoadDirectory(fm.GetCurrentPath())
		}),
		toolbarAction(theme.FolderIcon(), keymanager.CommandTreeShow, func() {
			fm.ShowDirectoryTreeDialog()
		}),
		toolbarAction(theme.FolderNewIcon(), keymanager.CommandWindowNew, func() {
			fm.OpenNewWindow()
		}),
	}
	if debugMode {
		toolbarItems = append(toolbarItems, toolbarAction(theme.SettingsIcon(), "debug.dump", func() {
			fm.DumpKeyManagerState()
		}))
	}
	toolbarItems = append(toolbarItems,
		toolbarAction(theme.InfoIcon(), "app.info", func() {
			fm.ShowVersionDialog()
		}),
	)
	toolbar := widget.NewToolbar(toolbarItems...)

	// Jobs button on the right
	fm.jobsButton = widget.NewButton("Jobs", fm.mainScreenPointerCommand(keymanager.CommandJobsShow, fm.ShowJobsDialog))
	fm.jobsButton.Importance = widget.MediumImportance

	// Layout with search overlay
	// Top row: toolbar on left, Jobs button on right
	toolbarRow := container.NewBorder(nil, nil, nil, fm.jobsButton, toolbar)
	// Subscribe to job updates to update indicator
	fm.jobsUnsub = fm.jobManager().Subscribe(func() { fyne.Do(fm.onJobsUpdated) })
	mainContent := container.NewBorder(
		container.NewVBox(toolbarRow, fm.pathDisplay, statusBar),
		nil, nil, nil,
		fm.fileList,
	)
	fm.windowHighlight = ui.NewHighlightFrame(fm.customTheme)

	// Keep normal content clear of the full-bounds highlight frame. This also
	// leaves stable room beside the list scrollbar when no highlight is active.
	highlightedMainContent := fm.windowHighlight.WrapContent(mainContent)

	// Stack main content with overlays on top (search, busy)
	content := container.NewMax(
		highlightedMainContent,
		container.NewBorder(
			fm.searchOverlay.GetContainer(), // Top overlay
			nil, nil, nil,
			nil, // Center is empty, overlay is at top
		),
		fm.busy.GetContainer(), // Highest overlay to block interactions
	)

	// The whole main surface belongs to one focusable KeySink. Keeping the
	// toolbar, passive labels, padding, and overlays inside it prevents a mouse
	// press outside the list from dropping into Fyne's no-focus fallback.
	fm.fileListView = ui.NewKeySink(
		content,
		fm.keyManager,
		ui.WithTabCapture(true),
		ui.WithTapFocus(true),
		ui.WithFocusChanged(fm.setWindowActive),
	)
	fm.window.SetContent(fm.fileListView)
	fm.setupDropHandler()
	fm.window.Resize(fm.initialWindowSize)

	// Initialize jobs indicator state
	fm.onJobsUpdated()

	// Ensure initial focus sits on the main-screen KeySink.
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

// mainScreenPointerAction restores the persistent keyboard owner before a
// pointer-triggered action. This ordering matters for actions that open an
// overlay or another window: Fyne can then reactivate the retained KeySink
// when that temporary owner closes.
func (fm *FileManager) mainScreenPointerAction(action func()) func() {
	return func() {
		fm.focusFileList("main-screen-pointer-action")
		if action != nil {
			action()
		}
	}
}

// mainScreenPointerCommand applies the same semantic policy as keyboard
// bindings before a toolbar or button action runs.
func (fm *FileManager) mainScreenPointerCommand(commandID string, action func()) func() {
	return fm.mainScreenPointerAction(func() {
		if !fm.mainScreenCommandAllowed(commandID) {
			fm.logCachedListingBlocked(commandID)
			return
		}
		if action != nil {
			action()
		}
	})
}
