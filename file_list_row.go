package main

import (
	"fmt"
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
	if id < 0 || int(id) >= len(fm.files) {
		return
	}
	fileInfo := fm.files[id]
	index := int(id)

	row, ok := obj.(*ui.FileListRow)
	if !ok {
		return
	}

	// Set icon resource with async service (Windows uses real icons if available).
	folderRes := theme.FolderIcon()
	fileRes := theme.FileIcon()
	if fileInfo.IsDir {
		row.Icon.SetResource(folderRes)
	} else {
		textSize := int(fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText))
		ext := strings.ToLower(filepath.Ext(fileInfo.Name))
		if fm.iconSvc != nil {
			if res, ok := fm.iconSvc.GetCachedOrRequest(fileInfo.Path, fileInfo.IsDir, ext, textSize); ok && res != nil {
				row.Icon.SetResource(res)
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
			fileInfo.Path, modifier, fm.windowActive, focusedObjectLabel(fm.window), fm.currentPath)
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
	isSelected := fm.selectedFiles[fileInfo.Path]
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
