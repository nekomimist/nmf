package main

import "fyne.io/fyne/v2"

type canvasObjectLookup interface {
	CanvasForObject(fyne.CanvasObject) fyne.Canvas
}

func (fm *FileManager) DumpKeyManagerState() {
	if fm == nil || fm.keyManager == nil {
		debugPrint("FileManager: keymanager dump skipped keyManager=nil")
		return
	}
	debugPrint("FileManager: keymanager dump requested path=%s focused=%s", fm.currentPath, focusedObjectLabel(fm.window))
	debugPrint("KeyManager: dump\n%s", fm.keyManager.DumpState())
	canvasWidth, canvasHeight := float32(0), float32(0)
	var windowCanvas fyne.Canvas
	if fm.window != nil {
		windowCanvas = fm.window.Canvas()
		canvasWidth = windowCanvas.Size().Width
		canvasHeight = windowCanvas.Size().Height
	}
	listWidth, listHeight := float32(0), float32(0)
	if fm.fileList != nil {
		listWidth = fm.fileList.Size().Width
		listHeight = fm.fileList.Size().Height
	}
	debugPrint("FileManager: cursor dump index=%d count=%d refreshSeq=%d itemUpdateSeq=%d active=%t focused=%s canvas=%.0fx%.0f list=%.0fx%.0f path=%q cursor=%q",
		fm.GetCurrentCursorIndex(), len(fm.files), fm.cursorRefreshSeq, fm.cursorItemUpdateSeq,
		fm.windowActive, focusedObjectLabel(fm.window), canvasWidth, canvasHeight, listWidth, listHeight,
		fm.currentPath, fm.cursorPath)
	fm.dumpCanvasMappings(windowCanvas)
	if fm.statusLabel != nil {
		fm.statusLabel.SetText("KeyManager state dumped to debug log")
	}
	fm.FocusFileList()
}

func (fm *FileManager) dumpCanvasMappings(windowCanvas fyne.Canvas) {
	if windowCanvas == nil {
		debugPrint("FileManager: canvas map skipped windowCanvas=nil path=%q", fm.currentPath)
		return
	}
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		debugPrint("FileManager: canvas map skipped driver=nil path=%q", fm.currentPath)
		return
	}

	// CanvasForObject also renews Fyne's cache entry when one exists. Keep
	// these reverse lookups confined to this explicitly requested dump.
	lookup := app.Driver()
	windows := lookup.AllWindows()
	registered := false
	for _, window := range windows {
		if window == fm.window {
			registered = true
			break
		}
	}
	debugPrint("FileManager: canvas map driver=%T windows=%d registered=%t window=%T content=%s sink=%s list=%s row=%s rowPath=%q status=%s highlight=%s path=%q",
		lookup, len(windows), registered, windowCanvas,
		canvasMappingState(lookup, windowCanvas.Content(), windowCanvas),
		canvasMappingState(lookup, fm.fileListView, windowCanvas),
		canvasMappingState(lookup, fm.fileList, windowCanvas),
		canvasMappingState(lookup, fm.cursorAnchor.object, windowCanvas),
		fm.cursorAnchor.path,
		canvasMappingState(lookup, fm.statusLabel, windowCanvas),
		canvasMappingState(lookup, fm.windowHighlight, windowCanvas),
		fm.currentPath)
}

func canvasMappingState(lookup canvasObjectLookup, object fyne.CanvasObject, windowCanvas fyne.Canvas) string {
	if object == nil {
		return "object-nil"
	}
	if lookup == nil {
		return "lookup-nil"
	}
	mapped := lookup.CanvasForObject(object)
	if mapped == nil {
		return "nil"
	}
	if mapped == windowCanvas {
		return "window"
	}
	return "other"
}
