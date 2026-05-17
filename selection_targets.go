package main

import "nmf/internal/fileinfo"

func (fm *FileManager) selectedFileInfos() []fileinfo.FileInfo {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	targets := make([]fileinfo.FileInfo, 0, len(fm.selectedFiles))
	for _, fi := range fm.files {
		if !fm.selectedFiles[fi.Path] || !isTargetFileInfo(fi) {
			continue
		}
		targets = append(targets, fi)
	}
	return targets
}

// GetAllSelectedFiles returns marked files from all open file manager windows
// in window order, then visible list order within each window.
func (fm *FileManager) GetAllSelectedFiles() []fileinfo.FileInfo {
	windows := snapshotFileManagerWindows()
	if len(windows) == 0 {
		windows = []*FileManager{fm}
	}

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

func isTargetFileInfo(fi fileinfo.FileInfo) bool {
	return fi.Name != ".." && fi.Status != fileinfo.StatusDeleted
}
