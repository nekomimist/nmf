package main

import (
	"strings"
	"testing"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
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
