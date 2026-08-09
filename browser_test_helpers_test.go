package main

import (
	"nmf/internal/browser"
	"nmf/internal/config"
	"nmf/internal/fileinfo"
)

type testBrowserOptions struct {
	path         string
	files        []fileinfo.FileInfo
	selected     map[string]bool
	cursorPath   string
	sort         config.SortConfig
	storage      fileinfo.StorageInfo
	storageKnown bool
}

func newTestBrowser(options testBrowserOptions) *browser.Model {
	sortConfig := options.sort
	if sortConfig.SortBy == "" {
		sortConfig.SortBy = "name"
	}
	if sortConfig.SortOrder == "" {
		sortConfig.SortOrder = "asc"
	}

	model := browser.New(options.path, sortConfig)
	model.ReplaceDirectory(
		options.path,
		options.files,
		options.storage,
		options.storageKnown,
		sortConfig,
	)
	for path, selected := range options.selected {
		model.SetSelected(path, selected)
	}
	if options.cursorPath != "" {
		model.SetCursorPath(options.cursorPath)
	}
	return model
}
