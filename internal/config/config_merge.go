package config

import "strings"

// mergeConfigs validates the entire overlay before changing any defaults.
func mergeConfigs(defaultConfig *Config, fileConfig *rawConfig) error {
	if err := validateRawConfig(fileConfig); err != nil {
		return err
	}
	mergeWindowConfig(&defaultConfig.Window, &fileConfig.Window)
	if fileConfig.Startup.Directory != nil {
		defaultConfig.Startup.Directory = strings.TrimSpace(*fileConfig.Startup.Directory)
	}
	mergeThemeConfig(&defaultConfig.Theme, &fileConfig.Theme)
	mergeDebugConfig(&defaultConfig.Debug, &fileConfig.Debug)
	mergeUIConfig(&defaultConfig.UI, &fileConfig.UI)
	return nil
}

func mergeWindowConfig(dst *WindowConfig, src *rawWindowConfig) {
	if src.Width != nil {
		dst.Width = *src.Width
	}
	if src.Height != nil {
		dst.Height = *src.Height
	}
	if src.X != nil {
		x := *src.X
		dst.X = &x
	}
	if src.Y != nil {
		y := *src.Y
		dst.Y = &y
	}
	if src.BringAllToFront != nil {
		dst.BringAllToFront = *src.BringAllToFront
	}
	if src.MoveSourceOnNewWindow != nil {
		dst.MoveSourceOnNewWindow = *src.MoveSourceOnNewWindow
	}
}

func mergeThemeConfig(dst *ThemeConfig, src *rawThemeConfig) {
	if src.Dark != nil {
		dst.Dark = *src.Dark
	}
	if src.FontSize != nil {
		dst.FontSize = *src.FontSize
	}
	if src.FontName != nil {
		dst.FontName = strings.TrimSpace(*src.FontName)
	}
	if src.FontPath != nil && *src.FontPath != "" {
		dst.FontPath = *src.FontPath
	}
	if src.MonospaceFontName != nil {
		dst.MonospaceFontName = strings.TrimSpace(*src.MonospaceFontName)
	}
	if src.MonospaceFontPath != nil && *src.MonospaceFontPath != "" {
		dst.MonospaceFontPath = *src.MonospaceFontPath
	}
	if src.Colors != nil {
		dst.Colors = make(map[string]ThemeColorConfig, len(src.Colors))
		for k, v := range src.Colors {
			dst.Colors[strings.TrimSpace(k)] = cloneThemeColorConfig(v)
		}
	}
}

func mergeDebugConfig(dst *DebugConfig, src *rawDebugConfig) {
	if src.Enabled != nil {
		dst.Enabled = *src.Enabled
	}
	if src.LogDirectory != nil {
		dst.LogDirectory = strings.TrimSpace(*src.LogDirectory)
	}
	if src.MaxLogFiles != nil && *src.MaxLogFiles > 0 {
		dst.MaxLogFiles = *src.MaxLogFiles
	}
}

func mergeUIConfig(dst *UIConfig, src *rawUIConfig) {
	if src.ShowHiddenFiles != nil {
		dst.ShowHiddenFiles = *src.ShowHiddenFiles
	}
	mergeSortConfig(&dst.Sort, &src.Sort)
	if src.ItemSpacing != nil && *src.ItemSpacing != 0 {
		dst.ItemSpacing = *src.ItemSpacing
	}
	if src.ScrollMargin != nil {
		dst.ScrollMargin = *src.ScrollMargin
	}
	if src.Copy.PreserveTimestamps != nil {
		dst.Copy.PreserveTimestamps = *src.Copy.PreserveTimestamps
	}
	mergeViewerConfig(&dst.Viewer, &src.Viewer)
	if src.Archive.ZipNameEncoding != nil && strings.TrimSpace(*src.Archive.ZipNameEncoding) != "" {
		dst.Archive.ZipNameEncoding = strings.TrimSpace(*src.Archive.ZipNameEncoding)
	}
	if src.IME.Enabled != nil {
		dst.IME.Enabled = *src.IME.Enabled
	}
	mergeCursorStyleConfig(&dst.CursorStyle, &src.CursorStyle)
	mergeHistoryLimits(dst, src)
	mergeUIEntries(dst, src)
}

func mergeSortConfig(dst *SortConfig, src *rawSortConfig) {
	if src.SortBy != nil && *src.SortBy != "" {
		dst.SortBy = *src.SortBy
	}
	if src.SortOrder != nil && *src.SortOrder != "" {
		dst.SortOrder = *src.SortOrder
	}
	if src.DirectoriesFirst != nil {
		dst.DirectoriesFirst = *src.DirectoriesFirst
	}
}

func mergeViewerConfig(dst *ViewerConfig, src *rawViewerConfig) {
	if src.MaxWidth != nil && *src.MaxWidth >= 0 {
		dst.MaxWidth = *src.MaxWidth
	}
	if src.MaxHeight != nil && *src.MaxHeight >= 0 {
		dst.MaxHeight = *src.MaxHeight
	}
	if src.DefaultPane != nil {
		if pane := NormalizeViewerDefaultPane(*src.DefaultPane); pane != "" {
			dst.DefaultPane = pane
		}
	}
	if src.DefaultWrap != nil {
		dst.DefaultWrap = *src.DefaultWrap
	}
}

func mergeCursorStyleConfig(dst *CursorStyleConfig, src *rawCursorStyleConfig) {
	if src.Type != nil && *src.Type != "" {
		dst.Type = *src.Type
	}
	if src.Thickness != nil && *src.Thickness != 0 {
		dst.Thickness = *src.Thickness
	}
}

func mergeHistoryLimits(dst *UIConfig, src *rawUIConfig) {
	if src.CursorMemory.MaxEntries != nil && *src.CursorMemory.MaxEntries != 0 {
		dst.CursorMemory.MaxEntries = *src.CursorMemory.MaxEntries
	}
	if src.NavigationHistory.MaxEntries != nil && *src.NavigationHistory.MaxEntries != 0 {
		dst.NavigationHistory.MaxEntries = *src.NavigationHistory.MaxEntries
	}
	if src.FileFilter.MaxEntries != nil && *src.FileFilter.MaxEntries != 0 {
		dst.FileFilter.MaxEntries = *src.FileFilter.MaxEntries
	}
}

func mergeUIEntries(dst *UIConfig, src *rawUIConfig) {
	if src.DirectoryJumps.Entries != nil {
		dst.DirectoryJumps.Entries = src.DirectoryJumps.Entries
	}
	if src.KeyBindings != nil {
		dst.KeyBindings = src.KeyBindings
	}
	if src.ExternalCommands != nil {
		dst.ExternalCommands = src.ExternalCommands
	}
}
