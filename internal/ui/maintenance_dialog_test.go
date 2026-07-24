package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
	"nmf/internal/keymanager"
	"nmf/internal/maintenance"
)

func TestMaintenanceDialogEscapeClosesWhileScanIsBlocked(t *testing.T) {
	state := &config.State{
		CursorMemory: config.CursorMemoryState{
			Entries: map[string]string{"/blocked": "file.txt"},
		},
	}
	d := NewMaintenanceDialog(state, nil, func(string, ...interface{}) {})

	started := make(chan struct{})
	d.plan = func(ctx context.Context, _ maintenance.Targets, _ maintenance.Options, _ maintenance.ClassifyFunc, _ maintenance.AccessibleContextFunc) (maintenance.Result, error) {
		close(started)
		<-ctx.Done()
		return maintenance.Result{}, ctx.Err()
	}
	onMain := make(chan func(), 1)
	d.runOnMain = func(fn func()) {
		onMain <- fn
	}

	d.Scan()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background scan did not start")
	}

	handler := keymanager.NewMaintenanceDialogKeyHandler(d)
	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyEscape}, keymanager.ModifierState{}) {
		t.Fatal("Escape should be handled while scanning")
	}
	if !d.closed {
		t.Fatal("dialog was not closed by Escape")
	}

	select {
	case applyResult := <-onMain:
		applyResult()
	case <-time.After(time.Second):
		t.Fatal("background scan did not observe cancellation")
	}
	if d.scanned {
		t.Fatal("cancelled scan result was applied after the dialog closed")
	}
}

func TestMaintenanceDialogDefaultsToSkippingUnavailableVolumes(t *testing.T) {
	d := NewMaintenanceDialog(nil, nil, func(string, ...interface{}) {})

	if !d.skipUnavailableCheck.Checked {
		t.Fatal("Skip unavailable volumes should be checked by default")
	}
	if !d.options().SkipUnavailableVolumes {
		t.Fatal("dialog options should skip unavailable volumes by default")
	}
}
