package config

import (
	"fmt"
	"strings"
)

func validateRawConfig(cfg *rawConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateRawWindowConfig(&cfg.Window); err != nil {
		return err
	}
	if cfg.Theme.FontSize != nil && *cfg.Theme.FontSize < 0 {
		return fmt.Errorf("theme.fontSize must be zero or positive")
	}
	if cfg.Debug.MaxLogFiles != nil && *cfg.Debug.MaxLogFiles <= 0 {
		return fmt.Errorf("debug.maxLogFiles must be positive")
	}
	return validateRawUIConfig(&cfg.UI)
}

func validateRawUIConfig(cfg *rawUIConfig) error {
	if err := validateRawSortConfig(&cfg.Sort); err != nil {
		return err
	}
	if cfg.ItemSpacing != nil && *cfg.ItemSpacing < 0 {
		return fmt.Errorf("ui.itemSpacing must be zero or positive")
	}
	if cfg.ScrollMargin != nil && *cfg.ScrollMargin < 0 {
		return fmt.Errorf("ui.scrollMargin must be zero or positive")
	}
	if err := validateRawViewerConfig(&cfg.Viewer); err != nil {
		return err
	}
	if cfg.Archive.ZipNameEncoding != nil && strings.TrimSpace(*cfg.Archive.ZipNameEncoding) == "" {
		return fmt.Errorf("ui.archive.zipNameEncoding must not be empty")
	}
	if err := validateRawCursorStyleConfig(&cfg.CursorStyle); err != nil {
		return err
	}
	return validateRawHistoryLimits(cfg)
}

func validateRawWindowConfig(cfg *rawWindowConfig) error {
	if cfg.Width != nil && *cfg.Width <= 0 {
		return fmt.Errorf("window.width must be positive")
	}
	if cfg.Height != nil && *cfg.Height <= 0 {
		return fmt.Errorf("window.height must be positive")
	}
	return nil
}

func validateRawSortConfig(cfg *rawSortConfig) error {
	if cfg.SortBy != nil && !IsValidSortBy(*cfg.SortBy) {
		return fmt.Errorf("ui.sort.sortBy must be name, size, modified, or extension")
	}
	if cfg.SortOrder != nil && !IsValidSortOrder(*cfg.SortOrder) {
		return fmt.Errorf("ui.sort.sortOrder must be asc or desc")
	}
	return nil
}

func validateRawViewerConfig(cfg *rawViewerConfig) error {
	if cfg.MaxWidth != nil && *cfg.MaxWidth < 0 {
		return fmt.Errorf("ui.viewer.maxWidth must be zero or positive")
	}
	if cfg.MaxHeight != nil && *cfg.MaxHeight < 0 {
		return fmt.Errorf("ui.viewer.maxHeight must be zero or positive")
	}
	if cfg.DefaultPane != nil && NormalizeViewerDefaultPane(*cfg.DefaultPane) == "" {
		return fmt.Errorf("ui.viewer.defaultPane must be auto, text, markdown, or hex")
	}
	return nil
}

func validateRawCursorStyleConfig(cfg *rawCursorStyleConfig) error {
	if cfg.Type != nil && !IsValidCursorStyleType(*cfg.Type) {
		return fmt.Errorf("ui.cursorStyle.type must be underline, border, or background")
	}
	if cfg.Thickness != nil && *cfg.Thickness < 0 {
		return fmt.Errorf("ui.cursorStyle.thickness must be zero or positive")
	}
	return nil
}

func validateRawHistoryLimits(cfg *rawUIConfig) error {
	if cfg.CursorMemory.MaxEntries != nil && *cfg.CursorMemory.MaxEntries <= 0 {
		return fmt.Errorf("ui.cursorMemory.maxEntries must be positive")
	}
	if cfg.NavigationHistory.MaxEntries != nil && *cfg.NavigationHistory.MaxEntries <= 0 {
		return fmt.Errorf("ui.navigationHistory.maxEntries must be positive")
	}
	if cfg.FileFilter.MaxEntries != nil && *cfg.FileFilter.MaxEntries <= 0 {
		return fmt.Errorf("ui.fileFilter.maxEntries must be positive")
	}
	return nil
}
