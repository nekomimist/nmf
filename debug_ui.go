package main

func (fm *FileManager) DumpKeyManagerState() {
	if fm == nil || fm.keyManager == nil {
		debugPrint("FileManager: keymanager dump skipped keyManager=nil")
		return
	}
	debugPrint("FileManager: keymanager dump requested path=%s focused=%s", fm.currentPath, focusedObjectLabel(fm.window))
	debugPrint("KeyManager: dump\n%s", fm.keyManager.DumpState())
	if fm.statusLabel != nil {
		fm.statusLabel.SetText("KeyManager state dumped to debug log")
	}
	fm.FocusFileList()
}
