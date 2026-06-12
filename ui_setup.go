package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	customtheme "nmf/internal/theme"
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
	fm.fileList = widget.NewListWithData(
		fm.fileBinding,
		func() fyne.CanvasObject {
			// Create tappable icon (onTapped will be set in UpdateItem)
			icon := ui.NewTappableIcon(theme.FolderIcon(), nil)
			nameLabel := ui.NewFileNameLabel("filename", fm.customTheme.GetCustomColor(customtheme.ColorFileRegular))
			info := widget.NewLabel("info")
			info.TextStyle = fyne.TextStyle{Monospace: true}

			// Size icon based on text height for consistency
			textSize := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
			icon.Resize(fyne.NewSize(textSize, textSize))

			// The name is the middle object, so it only receives the space left
			// after the icon and right-aligned info fields have been placed.
			borderContainer := container.NewBorder(
				nil, nil, icon, info, nameLabel,
			)

			// Use normal container with max layout to hold content and decorations
			return container.NewMax(borderContainer)
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			dataItem := item.(binding.Untyped)
			data, _ := dataItem.Get()
			listItem := data.(fileinfo.ListItem)
			fileInfo := listItem.FileInfo
			index := listItem.Index

			// obj is a container with border and optional cursor/selection elements
			outerContainer := obj.(*fyne.Container)

			// Find the border container (should be first element)
			var border *fyne.Container
			if len(outerContainer.Objects) > 0 {
				if container, ok := outerContainer.Objects[0].(*fyne.Container); ok {
					border = container
				}
			}

			if border != nil {
				// Find widgets within border
				var icon *ui.TappableIcon
				var nameLabel *ui.FileNameLabel
				var infoLabel *widget.Label

				for _, obj := range border.Objects {
					if obj == nil {
						continue
					}
					if tappableIcon, ok := obj.(*ui.TappableIcon); ok {
						icon = tappableIcon
					} else if fileNameLabel, ok := obj.(*ui.FileNameLabel); ok {
						nameLabel = fileNameLabel
					} else if label, ok := obj.(*widget.Label); ok {
						infoLabel = label
					}
				}

				if icon != nil && nameLabel != nil && infoLabel != nil {
					// Set icon resource with async service (Windows uses real icons if available)
					// Default placeholders
					folderRes := theme.FolderIcon()
					fileRes := theme.FileIcon()
					if fileInfo.IsDir {
						icon.SetResource(folderRes)
					} else {
						// Desired icon size roughly equals text size
						textSize := int(fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText))
						ext := strings.ToLower(filepath.Ext(fileInfo.Name))
						if fm.iconSvc != nil {
							if res, ok := fm.iconSvc.GetCachedOrRequest(fileInfo.Path, fileInfo.IsDir, ext, textSize); ok && res != nil {
								icon.SetResource(res)
							} else {
								icon.SetResource(fileRes)
							}
						} else {
							icon.SetResource(fileRes)
						}
					}

					// Set onTapped handler for icon
					icon.SetOnTapped(func() {
						debugPrint("FileManager: Icon tapped path=%s dir=%t", fileInfo.Path, fileInfo.IsDir)
						if fileInfo.IsDir {
							fm.LoadDirectory(fileInfo.Path)
						}
					})
					icon.SetOnDragged(func() {
						debugPrint("FileManager: Icon dragged path=%s", fileInfo.Path)
						fm.StartFileDrag(fileInfo)
					})

					// Get text color based on file type
					textColor := fileinfo.GetTextColor(fileInfo.FileType, fm.customTheme)
					nameLabel.SetFile(fileInfo.Name, textColor, fileInfo.Status == fileinfo.StatusDeleted)

					if fileInfo.IsDir {
						infoLabel.SetText(fmt.Sprintf("<dir> %s %s",
							fileInfo.Modified.Format("2006-01-02"),
							fileInfo.Modified.Format("15:04:05")))
					} else {
						infoLabel.SetText(fmt.Sprintf("%s %s %s",
							fileinfo.FormatFileSize(fileInfo.Size),
							fileInfo.Modified.Format("2006-01-02"),
							fileInfo.Modified.Format("15:04:05")))
					}
				}
			}

			// Handle 4 display states
			currentCursorIdx := fm.GetCurrentCursorIndex()
			isCursor := index == currentCursorIdx
			isSelected := fm.selectedFiles[fileInfo.Path]
			if isCursor {
				fm.cursorAnchor = cursorRowAnchor{path: fileInfo.Path, object: obj}
			} else if fm.cursorAnchor.object == obj {
				fm.cursorAnchor = cursorRowAnchor{}
			}

			// Clear all decoration elements first
			outerContainer.Objects = []fyne.CanvasObject{border}

			// Add status background if file has a status (covers entire item like selection)
			statusBGColor := fileinfo.GetStatusBackgroundColor(fileInfo.Status, fm.customTheme)
			if statusBGColor != nil {
				statusBG := canvas.NewRectangle(*statusBGColor)
				statusBG.Resize(obj.Size())
				statusBG.Move(fyne.NewPos(0, 0))
				// Wrap status background in WithoutLayout container
				statusContainer := container.NewWithoutLayout(statusBG)
				outerContainer.Objects = append(outerContainer.Objects, statusContainer)
			}

			// Add selection background if selected (covers entire item)
			if isSelected {
				selectionColor := fm.customTheme.GetCustomColor(customtheme.ColorSelectionBackground)
				selectionBG := canvas.NewRectangle(selectionColor)
				selectionBG.Resize(obj.Size())
				selectionBG.Move(fyne.NewPos(0, 0))
				// Wrap selection background in WithoutLayout container
				selectionContainer := container.NewWithoutLayout(selectionBG)
				outerContainer.Objects = append(outerContainer.Objects, selectionContainer)
			}

			// Add cursor if at cursor position (covers entire item like status/selection)
			if isCursor {
				cursor := fm.cursorRenderer.RenderCursor(obj.Size(), fyne.NewPos(0, 0), fm.config.UI.CursorStyle, fm.cursorThemeProvider())

				// Wrap cursor in a container that won't be affected by NewMax
				cursorContainer := container.NewWithoutLayout(cursor)
				outerContainer.Objects = append(outerContainer.Objects, cursorContainer)
			}
		},
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
		debugPrint("FileManager: List selected id=%d", id)
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
	fm.jobsUnsub = jobs.GetManager().Subscribe(func() { fyne.Do(fm.onJobsUpdated) })
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

	// Setup window close handler to properly stop DirectoryWatcher
	fm.window.SetCloseIntercept(func() {
		debugPrint("FileManager: Window close intercepted - initiating cleanup for path: %s", fm.currentPath)
		if fm.dirWatcher != nil {
			debugPrint("FileManager: Stopping DirectoryWatcher...")
			fm.dirWatcher.Stop()
			debugPrint("FileManager: DirectoryWatcher.Stop() completed successfully")
		} else {
			debugPrint("FileManager: DirectoryWatcher was nil, skipping stop")
		}
		debugPrint("FileManager: Proceeding with window close")
		fm.window.Close()
	})

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
