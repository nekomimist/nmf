package main

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	customtheme "nmf/internal/theme"
)

const (
	statusNoticeDuration           = 5 * time.Second
	directoryCacheStatusBadgeDelay = 250 * time.Millisecond
)

type directoryCacheStatusBadge struct {
	container  *fyne.Container
	content    *fyne.Container
	background *canvas.Rectangle
	label      *widget.Label
	theme      fyne.Theme
	afterFunc  func(time.Duration, func())
}

type directoryCacheStatusBadgeTheme struct {
	fyne.Theme
	foreground color.Color
}

func (t *directoryCacheStatusBadgeTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == fynetheme.ColorNameForeground {
		return t.foreground
	}
	return t.Theme.Color(name, variant)
}

func newDirectoryCacheStatusBadge(themeProvider *customtheme.CustomTheme) *directoryCacheStatusBadge {
	background := canvas.NewRectangle(themeProvider.GetCustomColor(customtheme.ColorSearchOverlayBackground))
	label := widget.NewLabel("")
	label.TextStyle = fyne.TextStyle{Monospace: true}
	badgeTheme := &directoryCacheStatusBadgeTheme{
		Theme:      themeProvider,
		foreground: themeProvider.GetCustomColor(customtheme.ColorSearchOverlayForeground),
	}
	content := container.NewStack(background, container.NewThemeOverride(label, badgeTheme))
	content.Hide()
	badge := &directoryCacheStatusBadge{
		container:  container.NewBorder(nil, nil, nil, content),
		content:    content,
		background: background,
		label:      label,
		theme:      badgeTheme,
		afterFunc: func(delay time.Duration, callback func()) {
			time.AfterFunc(delay, callback)
		},
	}
	return badge
}

func (b *directoryCacheStatusBadge) setText(text string) {
	if b == nil || b.container == nil || b.content == nil || b.label == nil {
		return
	}
	if text == "" {
		if b.content.Visible() {
			b.content.Hide()
			b.container.Refresh()
		}
		return
	}
	changed := false
	if b.label.Text != text {
		b.label.SetText(text)
		changed = true
	}
	if !b.content.Visible() {
		b.content.Show()
		changed = true
	}
	if changed {
		// The right-aligning border ignores hidden children. Relayout it when
		// the badge appears or its text width changes.
		b.container.Refresh()
	}
}

func (fm *FileManager) updateStatusBar() {
	if fm == nil {
		return
	}
	if fm.statusLabel != nil {
		fm.statusLabel.SetText(fm.statusBarText())
	}
	if fm.cacheStatusBadge != nil {
		text := ""
		if fm.cacheStatusBadgeReady {
			text = fm.directoryCacheStatusText()
		}
		fm.cacheStatusBadge.setText(text)
	}
}

func (fm *FileManager) onDirectoryListingStateChanged(previous, state directoryListingState) {
	if state == directoryListingFresh {
		fm.cacheStatusBadgeGen++
		fm.cacheStatusBadgeReady = false
		fm.updateStatusBar()
		return
	}

	// Entering a provisional view starts a new delay. A refreshing-to-stale
	// transition keeps the same delay so a fast failure does not flash either.
	if state == directoryListingCachedRefreshing || previous == directoryListingFresh {
		fm.startDirectoryCacheStatusBadgeDelay()
		return
	}
	fm.updateStatusBar()
}

func (fm *FileManager) startDirectoryCacheStatusBadgeDelay() {
	fm.cacheStatusBadgeGen++
	generation := fm.cacheStatusBadgeGen
	fm.cacheStatusBadgeReady = false
	fm.updateStatusBar()

	badge := fm.cacheStatusBadge
	if badge == nil || badge.afterFunc == nil {
		return
	}
	badge.afterFunc(directoryCacheStatusBadgeDelay, func() {
		if fm.isWindowClosed() {
			return
		}
		fyne.Do(func() {
			fm.revealDirectoryCacheStatusBadge(generation)
		})
	})
}

func (fm *FileManager) revealDirectoryCacheStatusBadge(generation uint64) {
	if fm == nil || fm.isWindowClosed() || fm.cacheStatusBadgeGen != generation ||
		fm.directoryListingState == directoryListingFresh {
		return
	}
	fm.cacheStatusBadgeReady = true
	fm.updateStatusBar()
}

func (fm *FileManager) statusBarText() string {
	// While a notice is active it fully replaces the normal status line so
	// the label never grows past one line (see ui_setup.go's Truncation
	// setting on fm.statusLabel, which relies on this invariant).
	if fm.statusNotice != "" {
		return fm.statusNotice
	}

	stats := fm.browserModel().ListingStats()

	free := "-"
	used := "-"
	total := "-"
	if stats.StorageKnown {
		free = fileinfo.FormatFileSize(int64(stats.Storage.Free))
		used = fileinfo.FormatFileSize(int64(stats.Storage.Used))
		total = fileinfo.FormatFileSize(int64(stats.Storage.Total))
	}

	return fmt.Sprintf("Mark: %d | Entry: %d/%d | Free: %s | Used: %s | Total: %s",
		stats.MarkedEntries, stats.VisibleEntries, stats.TotalEntries, free, used, total)
}

func (fm *FileManager) directoryCacheStatusText() string {
	switch fm.directoryListingState {
	case directoryListingCachedRefreshing:
		return "Cached listing — refreshing; navigation only"
	case directoryListingCachedStale:
		return "Cached listing — refresh failed; navigation only"
	default:
		return ""
	}
}

func parentFallbackStatusNotice(requestedPath, openedPath string) string {
	return fmt.Sprintf("Path not found; opened nearest parent: %s → %s", requestedPath, openedPath)
}

// showStatusNotice appends a short-lived, non-modal notice to the status bar.
// The generation token prevents an older timer from clearing a newer message.
func (fm *FileManager) showStatusNotice(notice string) {
	if fm == nil || notice == "" {
		return
	}
	fm.statusNoticeGeneration++
	generation := fm.statusNoticeGeneration
	fm.statusNotice = notice
	fm.updateStatusBar()

	time.AfterFunc(statusNoticeDuration, func() {
		fyne.Do(func() {
			fm.expireStatusNotice(generation)
		})
	})
}

// expireStatusNotice clears the notice if generation still matches the most
// recently shown one, i.e. no newer notice (or clearStatusNotice) has
// superseded it since the timer was scheduled. Split out from showStatusNotice
// so the stale-timer guard can be exercised directly by tests without
// waiting for the real statusNoticeDuration.
func (fm *FileManager) expireStatusNotice(generation uint64) {
	if fm.isWindowClosed() || fm.statusNoticeGeneration != generation {
		return
	}
	fm.statusNotice = ""
	fm.updateStatusBar()
}

// clearStatusNotice removes a previous navigation result when another
// directory load begins. It also invalidates that notice's expiry callback.
func (fm *FileManager) clearStatusNotice() {
	if fm == nil {
		return
	}
	fm.statusNoticeGeneration++
	if fm.statusNotice == "" {
		return
	}
	fm.statusNotice = ""
	fm.updateStatusBar()
}
