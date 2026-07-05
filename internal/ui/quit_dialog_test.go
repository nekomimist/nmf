package ui

import (
	"testing"

	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/keymanager"
)

func TestQuitConfirmDialogTextWithoutActiveJobs(t *testing.T) {
	dialog := NewQuitConfirmDialog(nil, func(string, ...interface{}) {}, 0)

	if got := dialog.message(); got != "Are you sure you want to quit the file manager?" {
		t.Fatalf("message = %q", got)
	}
	confirm, cancel := dialog.buttonTexts()
	if confirm != "Yes" || cancel != "No" {
		t.Fatalf("button texts = %q/%q, want Yes/No", confirm, cancel)
	}
}

func TestQuitConfirmDialogTextWithActiveJobs(t *testing.T) {
	dialog := NewQuitConfirmDialog(nil, func(string, ...interface{}) {}, 3)

	if got := dialog.message(); got != "There are 3 pending or running job(s). Quit anyway?" {
		t.Fatalf("message = %q", got)
	}
	confirm, cancel := dialog.buttonTexts()
	if confirm != "Quit Anyway" || cancel != "No" {
		t.Fatalf("button texts = %q/%q, want Quit Anyway/No", confirm, cancel)
	}
}

func TestQuitConfirmDialogButtonOrderWithoutActiveJobs(t *testing.T) {
	dialog := NewQuitConfirmDialog(nil, func(string, ...interface{}) {}, 0)
	left, right := rowButtons(t, dialog.buttonRow())

	if left.Text != "No" || right.Text != "Yes" {
		t.Fatalf("button order = %q/%q, want No/Yes", left.Text, right.Text)
	}
	if left.Importance == widget.HighImportance {
		t.Fatal("No should not be high importance without active jobs")
	}
	if right.Importance != widget.HighImportance {
		t.Fatalf("Yes importance = %v, want high", right.Importance)
	}
}

func TestQuitConfirmDialogButtonOrderWithActiveJobs(t *testing.T) {
	dialog := NewQuitConfirmDialog(nil, func(string, ...interface{}) {}, 1)
	left, right := rowButtons(t, dialog.buttonRow())

	if left.Text != "No" || right.Text != "Quit Anyway" {
		t.Fatalf("button order = %q/%q, want No/Quit Anyway", left.Text, right.Text)
	}
	if left.Importance != widget.HighImportance {
		t.Fatalf("No importance = %v, want high (Enter default) with active jobs", left.Importance)
	}
	if right.Importance != widget.DangerImportance {
		t.Fatalf("Quit Anyway importance = %v, want danger", right.Importance)
	}
	if right.Icon == nil || right.Icon.Name() != theme.WarningIcon().Name() {
		t.Fatalf("Quit Anyway icon = %v, want warning icon", right.Icon)
	}
}

func TestQuitConfirmDialogDefaultActionDependsOnActiveJobs(t *testing.T) {
	tests := []struct {
		name       string
		activeJobs int
		want       bool
	}{
		{name: "normal confirms", activeJobs: 0, want: true},
		{name: "active jobs cancels", activeJobs: 1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := keymanager.NewKeyManager(func(string, ...interface{}) {})
			dialog := NewQuitConfirmDialog(km, func(string, ...interface{}) {}, tt.activeJobs)
			var got bool
			var called bool
			dialog.callback = func(confirmed bool) {
				got = confirmed
				called = true
			}

			dialog.DefaultQuitAction()

			if !called {
				t.Fatal("callback was not called")
			}
			if got != tt.want {
				t.Fatalf("confirmed = %t, want %t", got, tt.want)
			}
		})
	}
}
