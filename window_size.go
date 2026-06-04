package main

func (fm *FileManager) ResetWindowSize() {
	if fm == nil || fm.window == nil {
		return
	}

	debugPrint("FileManager: reset window size width=%.0f height=%.0f", fm.initialWindowSize.Width, fm.initialWindowSize.Height)
	resyncWindowScaleForReset(fm.window)
	fm.window.Resize(fm.initialWindowSize)
}

func (fm *FileManager) ResetAllWindowSizes() {
	for _, manager := range snapshotFileManagerWindows() {
		manager.ResetWindowSize()
	}
}
