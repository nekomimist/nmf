package ui

import (
	"strings"
	"testing"
	"time"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/search"
)

func TestFilterDialogDisplayOmitsUseCount(t *testing.T) {
	dialog := NewFilterDialog(
		[]config.FilterEntry{{Pattern: "*.go ;; Go files", UseCount: 7}},
		nil,
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)

	items, err := dialog.dataBinding.Get()
	if err != nil {
		t.Fatalf("data binding get failed: %v", err)
	}
	if len(items) != 1 || items[0] != "*.go ;; Go files" {
		t.Fatalf("display items = %#v, want raw pattern without usage suffix", items)
	}
	if strings.Contains(items[0], "used") {
		t.Fatalf("display item %q should not include usage text", items[0])
	}
}

func TestFilterDialogSearchMatchesCommentWithMigemo(t *testing.T) {
	dialog := NewFilterDialog(
		[]config.FilterEntry{
			{Pattern: "*.go ;; 日本語"},
			{Pattern: "*.md ;; docs"},
		},
		nil,
		nil,
		func(string, ...interface{}) {},
		search.NewProvider(func(string, ...interface{}) {}),
	)

	dialog.updateFilteredEntries("nihongo")

	if len(dialog.filteredEntries) != 1 || dialog.filteredEntries[0].Pattern != "*.go ;; 日本語" {
		t.Fatalf("filtered entries = %#v, want Japanese comment entry", dialog.filteredEntries)
	}
}

func TestFilterDialogPreviewUsesEffectivePattern(t *testing.T) {
	dialog := NewFilterDialog(
		nil,
		[]fileinfo.FileInfo{
			{Name: "main.go"},
			{Name: "notes.md"},
			{Name: "docs", IsDir: true},
		},
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)

	dialog.updatePreview("*.go ;; Go files")

	if got := dialog.previewLabel.Text; got != "Matches: 1 files + 1 directories" {
		t.Fatalf("preview = %q, want match count from effective pattern", got)
	}
}

func TestFilterDialogEnterPrefersSelectedHistoryMatch(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	dialog := NewFilterDialog(
		[]config.FilterEntry{{Pattern: "*.go ;; 日本語", LastUsed: time.Now(), UseCount: 3}},
		nil,
		km,
		func(string, ...interface{}) {},
		search.NewProvider(func(string, ...interface{}) {}),
	)
	dialog.searchEntry.SetText("nihongo")

	var selected *config.FilterEntry
	dialog.callback = func(entry *config.FilterEntry) { selected = entry }
	dialog.AcceptSelection()

	if selected == nil || selected.Pattern != "*.go ;; 日本語" {
		t.Fatalf("selected entry = %#v, want selected history entry", selected)
	}
}

func TestFilterDialogCtrlEnterUsesDirectInput(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	dialog := NewFilterDialog(
		[]config.FilterEntry{{Pattern: "*.go ;; 日本語"}},
		nil,
		km,
		func(string, ...interface{}) {},
		search.NewProvider(func(string, ...interface{}) {}),
	)
	dialog.searchEntry.SetText("nihongo")

	var selected *config.FilterEntry
	dialog.callback = func(entry *config.FilterEntry) { selected = entry }
	dialog.AcceptDirectInput()

	if selected == nil || selected.Pattern != "nihongo" {
		t.Fatalf("selected entry = %#v, want direct search input", selected)
	}
}

func TestFilterDialogDeleteSelectedEntry(t *testing.T) {
	dialog := NewFilterDialog(
		[]config.FilterEntry{
			{Pattern: "*.go ;; Go files"},
			{Pattern: "*.md ;; Markdown"},
		},
		nil,
		nil,
		func(string, ...interface{}) {},
		search.NewPlainProvider(),
	)

	var deleted string
	dialog.deleteCallback = func(pattern string) { deleted = pattern }
	dialog.DeleteSelectedEntry()

	if deleted != "*.go ;; Go files" {
		t.Fatalf("deleted = %q, want selected pattern", deleted)
	}
	if len(dialog.allEntries) != 1 || dialog.allEntries[0].Pattern != "*.md ;; Markdown" {
		t.Fatalf("all entries = %#v, want selected entry removed", dialog.allEntries)
	}
}
