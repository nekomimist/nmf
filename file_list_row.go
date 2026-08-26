package main

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
)

func (fm *FileManager) newFileListRow() fyne.CanvasObject {
	return ui.NewFileListRow(
		fm.config.UI.CursorStyle,
		fm.customTheme.GetCustomColor(customtheme.ColorFileRegular),
	)
}

func (fm *FileManager) updateFileListRow(id widget.ListItemID, obj fyne.CanvasObject) {
	fileInfo, ok := fm.FileAt(int(id))
	if !ok {
		return
	}
	index := int(id)

	row, ok := obj.(*ui.FileListRow)
	if !ok {
		return
	}
	fm.trackFileListRow(index, fileInfo.Path, row)

	// Set the icon with the async service (Windows uses decoded native images if available).
	folderRes := theme.FolderIcon()
	fileRes := theme.FileIcon()
	if fileInfo.IsDir {
		row.Icon.SetResource(folderRes)
	} else {
		textSize := int(fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText))
		ext := strings.ToLower(filepath.Ext(fileInfo.Name))
		if fm.iconSvc != nil {
			if img, ok := fm.iconSvc.GetCachedOrRequest(fileInfo.Path, fileInfo.IsDir, ext, textSize); ok && img != nil {
				row.Icon.SetImage(img)
			} else {
				row.Icon.SetResource(fileRes)
			}
		} else {
			row.Icon.SetResource(fileRes)
		}
	}

	// Set callbacks for the file currently assigned to this recycled row.
	row.Icon.SetOnTapped(func() {
		debugPrint("FileManager: Icon tapped path=%s dir=%t", fileInfo.Path, fileInfo.IsDir)
		if fileInfo.IsDir {
			fm.LoadDirectory(fileInfo.Path)
		}
	})
	row.Icon.SetOnDragged(func() {
		debugPrint("FileManager: Icon dragged path=%s", fileInfo.Path)
		fm.StartFileDrag(fileInfo)
	})

	textColor := fileinfo.GetTextColor(fileInfo.FileType, fm.customTheme)
	row.NameLabel.SetFile(fileInfo.Name, textColor, fileInfo.Status == fileinfo.StatusDeleted)
	row.NameLabel.SetOnTapped(func(modifier fyne.KeyModifier) {
		debugPrint("FileManager: File name tapped file=%q modifier=%d active=%t focused=%s path=%q",
			fileInfo.Path, modifier, fm.windowActive, focusedObjectLabel(fm.window), fm.GetCurrentPath())
		fm.handleFileNameClick(index, fileInfo, modifier)
	})
	row.NameLabel.SetOnDragged(func() {
		debugPrint("FileManager: File name dragged path=%s", fileInfo.Path)
		fm.StartFileDrag(fileInfo)
	})

	if fileInfo.IsDir {
		row.InfoLabel.SetText(fmt.Sprintf("<dir> %s %s",
			fileInfo.Modified.Format("2006-01-02"),
			fileInfo.Modified.Format("15:04:05")))
	} else {
		row.InfoLabel.SetText(fmt.Sprintf("%s %s %s",
			fileinfo.FormatFileSize(fileInfo.Size),
			fileInfo.Modified.Format("2006-01-02"),
			fileInfo.Modified.Format("15:04:05")))
	}

	currentCursorIdx := fm.GetCurrentCursorIndex()
	isCursor := index == currentCursorIdx
	isSelected := fm.browserModel().IsSelected(fileInfo.Path)
	if isCursor {
		fm.cursorAnchor = cursorRowAnchor{path: fileInfo.Path, object: row}
	} else if fm.cursorAnchor.object == row {
		fm.cursorAnchor = cursorRowAnchor{}
	}

	statusColor := fileinfo.GetStatusBackgroundColor(fileInfo.Status, fm.customTheme)
	selectionColor := fm.customTheme.GetCustomColor(customtheme.ColorSelectionBackground)
	cursorColor := fm.cursorThemeProvider().GetCustomColor(customtheme.ColorCursor)
	row.SetDecorations(statusColor, isSelected, selectionColor, isCursor, cursorColor)
	if isCursor {
		fm.noteCursorItemUpdated(index)
	}
}

func (fm *FileManager) trackFileListRow(index int, path string, row *ui.FileListRow) {
	if fm.fileListRows == nil {
		fm.fileListRows = make(map[int]*ui.FileListRow)
		fm.fileListRowAssignments = make(map[*ui.FileListRow]fileListRowAssignment)
	}

	if previous, ok := fm.fileListRowAssignments[row]; ok && previous.index != index {
		if fm.fileListRows[previous.index] == row {
			delete(fm.fileListRows, previous.index)
		}
	}
	if previous := fm.fileListRows[index]; previous != nil && previous != row {
		delete(fm.fileListRowAssignments, previous)
	}

	fm.fileListRows[index] = row
	fm.fileListRowAssignments[row] = fileListRowAssignment{index: index, path: path}
}

func (fm *FileManager) trackedVisibleFileListRow(index int) (*ui.FileListRow, string, bool) {
	if fm.fileList == nil || index < 0 || !fm.fileListItemVisible(index) {
		return nil, "", false
	}
	row := fm.fileListRows[index]
	if row == nil {
		return nil, "", false
	}
	assignment, ok := fm.fileListRowAssignments[row]
	if !ok || assignment.index != index {
		return nil, "", false
	}
	fileInfo, ok := fm.FileAt(index)
	if !ok || assignment.path != fileInfo.Path {
		return nil, "", false
	}
	return row, assignment.path, true
}

func (fm *FileManager) clearTrackedCursorRows(except *ui.FileListRow) {
	for row := range fm.fileListRowAssignments {
		if row != except {
			row.SetCursor(false, color.RGBA{})
		}
	}
	if except == nil || fm.cursorAnchor.object != except {
		fm.cursorAnchor = cursorRowAnchor{}
	}
}

func (fm *FileManager) resetFileListRowTracking() {
	clear(fm.fileListRows)
	clear(fm.fileListRowAssignments)
	fm.cursorAnchor = cursorRowAnchor{}
}
