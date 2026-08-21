package main

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	fynetheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	customtheme "nmf/internal/theme"
)

func TestParentFallbackStatusNoticeNamesRequestedAndOpenedPaths(t *testing.T) {
	requested := "/home/neko/removed/child"
	opened := "/home/neko"
	notice := parentFallbackStatusNotice(requested, opened)
	for _, want := range []string{requested, opened, "opened nearest parent"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("notice %q does not contain %q", notice, want)
		}
	}
}

func TestStatusBarTextIncludesAndClearsNotice(t *testing.T) {
	fm := &FileManager{
		browser:      newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{{Name: "entry"}}}),
		statusNotice: "opened parent",
	}
	if got := fm.statusBarText(); !strings.Contains(got, "opened parent") {
		t.Fatalf("statusBarText %q does not include notice", got)
	}

	fm.clearStatusNotice()
	if fm.statusNotice != "" {
		t.Fatalf("statusNotice = %q, want empty", fm.statusNotice)
	}
	if fm.statusNoticeGeneration != 1 {
		t.Fatalf("statusNoticeGeneration = %d, want 1", fm.statusNoticeGeneration)
	}
	if got := fm.statusBarText(); strings.Contains(got, "opened parent") {
		t.Fatalf("statusBarText %q retained cleared notice", got)
	}
}

// TestStatusBarTextNoticeReplacesNormalLine verifies the fix in status_bar.go:
// a notice must fully replace the "Mark/Entry/Free/Used/Total" line rather
// than being appended to it, and the result must always be a single line
// (no "\n") so fm.statusLabel's height never changes and the file list does
// not visibly shift.
func TestStatusBarTextNoticeReplacesNormalLine(t *testing.T) {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{
			files:        []fileinfo.FileInfo{{Name: "entry", Path: "/tmp/entry"}},
			selected:     map[string]bool{"/tmp/entry": true},
			storage:      fileinfo.StorageInfo{Free: 1024, Used: 2048, Total: 3072},
			storageKnown: true,
		}),
		statusNotice: parentFallbackStatusNotice("/removed/child", "/removed"),
	}

	got := fm.statusBarText()
	if got != fm.statusNotice {
		t.Fatalf("statusBarText() = %q, want exactly the notice %q", got, fm.statusNotice)
	}
	if strings.Contains(got, "Mark:") || strings.Contains(got, "Entry:") {
		t.Fatalf("statusBarText %q should not contain the normal status fields while a notice is active", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("statusBarText %q must be a single line, found a newline", got)
	}
}

// TestStatusBarTextEmptyNoticeShowsNormalLine covers the "notice empty"
// branch explicitly (as opposed to the notice-was-cleared branch covered by
// TestStatusBarTextIncludesAndClearsNotice).
func TestStatusBarTextEmptyNoticeShowsNormalLine(t *testing.T) {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{{Name: "entry"}}}),
	}

	got := fm.statusBarText()
	if !strings.Contains(got, "Mark: 0 | Entry: 1/1") {
		t.Fatalf("statusBarText %q should show the normal status line when no notice is active", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("statusBarText %q must be a single line", got)
	}
}

// TestExpireStatusNoticeRestoresNormalLine exercises showStatusNotice's
// expiry path (extracted as expireStatusNotice so it can be driven directly
// here instead of waiting for the real statusNoticeDuration timer, and
// without depending on a running Fyne app for fyne.Do/CurrentApp).
func TestExpireStatusNoticeRestoresNormalLine(t *testing.T) {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{{Name: "entry"}}}),
	}
	fm.statusNoticeGeneration = 1
	fm.statusNotice = "opened parent"

	fm.expireStatusNotice(fm.statusNoticeGeneration)
	if fm.statusNotice != "" {
		t.Fatalf("statusNotice = %q after expiry, want empty", fm.statusNotice)
	}
	if got := fm.statusBarText(); strings.Contains(got, "opened parent") {
		t.Fatalf("statusBarText %q retained expired notice", got)
	}
}

// TestExpireStatusNoticeIgnoresStaleGeneration ensures an expiry callback
// scheduled for an older notice cannot clear a newer one that has since
// replaced it (the generation-counter guard in expireStatusNotice). This
// simulates the sequence showStatusNotice/clearStatusNotice would produce
// (generation bumped, notice replaced) without involving real timers.
func TestExpireStatusNoticeIgnoresStaleGeneration(t *testing.T) {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{{Name: "entry"}}}),
	}

	// First notice shown at generation 1; its expiry timer would later fire
	// with generation=1 captured.
	fm.statusNoticeGeneration = 1
	staleGeneration := fm.statusNoticeGeneration

	// A second notice replaces it before the first timer fires.
	fm.statusNoticeGeneration = 2
	fm.statusNotice = "second"

	fm.expireStatusNotice(staleGeneration)
	if fm.statusNotice != "second" {
		t.Fatalf("statusNotice = %q, want %q to survive the stale expiry", fm.statusNotice, "second")
	}

	// A non-stale expiry (matching the current generation) still clears it.
	fm.expireStatusNotice(fm.statusNoticeGeneration)
	if fm.statusNotice != "" {
		t.Fatalf("statusNotice = %q, want empty after matching-generation expiry", fm.statusNotice)
	}
}

func TestStatusBarTextShowsVisibleAndTotalEntries(t *testing.T) {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{
			files: []fileinfo.FileInfo{
				{Name: "..", Path: "/tmp", IsDir: true},
				{Name: "visible.txt", Path: "/tmp/visible.txt"},
				{Name: "filtered.log", Path: "/tmp/filtered.log"},
			},
			selected: map[string]bool{"/tmp/visible.txt": true},
			storage: fileinfo.StorageInfo{
				Free:  1024,
				Used:  2048,
				Total: 3072,
			},
			storageKnown: true,
		}),
	}
	if _, _, err := fm.browserModel().ApplyFilter(&config.FilterEntry{Pattern: "*.txt"}); err != nil {
		t.Fatalf("ApplyFilter returned error: %v", err)
	}

	text := fm.statusBarText()
	for _, want := range []string{
		"Mark: 1",
		"Entry: 1/2",
		"Free: 1.0 KB",
		"Used: 2.0 KB",
		"Total: 3.0 KB",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("statusBarText %q does not contain %q", text, want)
		}
	}
}

func TestStatusBarTextUsesDashForUnknownStorage(t *testing.T) {
	fm := &FileManager{
		browser: newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{{Name: "a.txt"}}}),
	}

	text := fm.statusBarText()
	if !strings.Contains(text, "Free: - | Used: - | Total: -") {
		t.Fatalf("statusBarText %q should use dashes for unknown storage", text)
	}
}

func TestUpdateStatusBarUsesOverlayForDirectoryCacheState(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	themeProvider := customtheme.NewCustomTheme(config.Default(), nil)
	badge := newDirectoryCacheStatusBadge(themeProvider)
	statusLabel := widget.NewLabel("")
	statusLabel.TextStyle = fyne.TextStyle{Monospace: true}
	fm := &FileManager{
		browser:               newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{{Name: "a.txt"}}}),
		statusLabel:           statusLabel,
		cacheStatusBadge:      badge,
		cacheStatusBadgeReady: true,
		directoryListingState: directoryListingCachedRefreshing,
	}
	statusBar := container.NewStack(fm.statusLabel, badge.container)
	badge.container.Resize(fyne.NewSize(800, fm.statusLabel.MinSize().Height))
	heightWithoutBadge := statusBar.MinSize().Height

	fm.updateStatusBar()
	if !badge.content.Visible() {
		t.Fatal("cache status badge is hidden while a cached listing is refreshing")
	}
	if got, want := badge.label.Text, "Cached listing — refreshing; navigation only"; got != want {
		t.Fatalf("cache status badge = %q, want %q", got, want)
	}
	fullTextLabel := widget.NewLabel(badge.label.Text)
	fullTextLabel.TextStyle = badge.label.TextStyle
	fullText := container.NewThemeOverride(fullTextLabel, badge.theme)
	if got, want := badge.content.MinSize().Width, fullText.MinSize().Width; got < want {
		t.Fatalf("cache badge width = %v, want at least full text width %v", got, want)
	}
	if got := statusBar.MinSize().Height; got != heightWithoutBadge {
		t.Fatalf("status bar height with badge = %v, want unchanged height %v", got, heightWithoutBadge)
	}
	if badge.label.TextStyle != fm.statusLabel.TextStyle {
		t.Fatalf("cache badge style = %+v, want status label style %+v", badge.label.TextStyle, fm.statusLabel.TextStyle)
	}
	if got := fm.statusLabel.Text; strings.Contains(got, "Cached listing") {
		t.Fatalf("normal status text %q includes cache state", got)
	}
	if got, want := color.RGBAModel.Convert(badge.background.FillColor), color.Color(themeProvider.GetCustomColor(customtheme.ColorSearchOverlayBackground)); got != want {
		t.Fatalf("cache badge background = %v, want search overlay background %v", got, want)
	}
	if got, want := color.RGBAModel.Convert(badge.theme.Color(fynetheme.ColorNameForeground, fynetheme.VariantLight)), color.Color(themeProvider.GetCustomColor(customtheme.ColorSearchOverlayForeground)); got != want {
		t.Fatalf("cache badge foreground = %v, want search overlay foreground %v", got, want)
	}
	if got, want := badge.content.Position().X, badge.container.Size().Width-badge.content.MinSize().Width; got != want {
		t.Fatalf("cache badge x = %v, want right-aligned x %v", got, want)
	}

	fm.directoryListingState = directoryListingCachedStale
	fm.updateStatusBar()
	if got, want := badge.label.Text, "Cached listing — refresh failed; navigation only"; got != want {
		t.Fatalf("stale cache status badge = %q, want %q", got, want)
	}

	fm.directoryListingState = directoryListingFresh
	fm.updateStatusBar()
	if badge.content.Visible() {
		t.Fatal("cache status badge remains visible for a fresh listing")
	}
}

func TestDirectoryCacheStatusBadgeDelayUsesCurrentGeneration(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	badge := newDirectoryCacheStatusBadge(customtheme.NewCustomTheme(config.Default(), nil))
	var delays []time.Duration
	var callbacks []func()
	badge.afterFunc = func(delay time.Duration, callback func()) {
		delays = append(delays, delay)
		callbacks = append(callbacks, callback)
	}
	fm := &FileManager{
		browser:          newTestBrowser(testBrowserOptions{files: []fileinfo.FileInfo{{Name: "a.txt"}}}),
		statusLabel:      widget.NewLabel(""),
		cacheStatusBadge: badge,
	}

	// A fast real read invalidates the pending reveal, so no badge appears.
	fm.setDirectoryListingState(directoryListingCachedRefreshing)
	if !fm.directoryListingNavigationOnly() {
		t.Fatal("cached listing did not become navigation-only immediately")
	}
	if badge.content.Visible() {
		t.Fatal("cache badge was visible before its delay")
	}
	if len(delays) != 1 || delays[0] != directoryCacheStatusBadgeDelay {
		t.Fatalf("scheduled delays = %v, want [%v]", delays, directoryCacheStatusBadgeDelay)
	}
	fm.setDirectoryListingState(directoryListingFresh)
	callbacks[0]()
	if badge.content.Visible() {
		t.Fatal("completed listing was revealed by a stale timer")
	}

	// Replacing one provisional listing with another restarts the delay. If
	// the current one fails, the surviving timer reveals the stale state.
	fm.setDirectoryListingState(directoryListingCachedRefreshing)
	fm.setDirectoryListingState(directoryListingCachedRefreshing)
	if len(callbacks) != 3 {
		t.Fatalf("scheduled callback count = %d, want 3", len(callbacks))
	}
	callbacks[1]()
	if badge.content.Visible() {
		t.Fatal("new provisional listing was revealed by the previous timer")
	}
	fm.setDirectoryListingState(directoryListingCachedStale)
	callbacks[2]()
	if !badge.content.Visible() {
		t.Fatal("stale cache badge was not shown after the current delay")
	}
	if got, want := badge.label.Text, "Cached listing — refresh failed; navigation only"; got != want {
		t.Fatalf("stale cache badge = %q, want %q", got, want)
	}
}
