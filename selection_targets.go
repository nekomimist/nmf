package main

import "nmf/internal/fileinfo"

// CommandTargets returns visible marks in list order, falling back to the cursor.
func (fm *FileManager) CommandTargets() []fileinfo.FileInfo {
	return fm.browserModel().CommandTargets()
}

func (fm *FileManager) selectedFileInfos() []fileinfo.FileInfo {
	return fm.browserModel().SelectedFiles()
}

// GetAllSelectedFiles returns marked files from all open file manager windows
// in window order, then visible list order within each window.
func (fm *FileManager) GetAllSelectedFiles() []fileinfo.FileInfo {
	windows := fm.registeredWindows()

	var targets []fileinfo.FileInfo
	for _, windowFM := range windows {
		if windowFM == nil {
			continue
		}
		targets = append(targets, windowFM.selectedFileInfos()...)
	}
	return targets
}

func (fm *FileManager) collectAllSelectedTargetPaths() []string {
	files := fm.GetAllSelectedFiles()
	paths := make([]string, len(files))
	for i, fi := range files {
		paths[i] = fi.Path
	}
	return paths
}
