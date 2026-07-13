package main

func (fm *FileManager) DumpKeyManagerState() {
	if fm == nil || fm.keyManager == nil {
		debugPrint("FileManager: keymanager dump skipped keyManager=nil")
		return
	}
	debugPrint("FileManager: keymanager dump requested path=%s focused=%s", fm.currentPath, focusedObjectLabel(fm.window))
	debugPrint("KeyManager: dump\n%s", fm.keyManager.DumpState())
	canvasWidth, canvasHeight := float32(0), float32(0)
	if fm.window != nil {
		canvasWidth = fm.window.Canvas().Size().Width
		canvasHeight = fm.window.Canvas().Size().Height
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
	if fm.statusLabel != nil {
		fm.statusLabel.SetText("KeyManager state dumped to debug log")
	}
	fm.FocusFileList()
}
