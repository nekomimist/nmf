package main

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"nmf/internal/config"
	customtheme "nmf/internal/theme"
	"nmf/internal/ui"
)

func TestCancelCompareReleasesInputAndCancelsWork(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("compare")
	cfg := config.Default()
	busy := ui.NewBusyController(window, nil, customtheme.NewCustomTheme(cfg, nil), time.Hour, nil)
	ctx, cancel := context.WithCancel(t.Context())
	fm := &FileManager{busy: busy, compareCancel: cancel, compareGeneration: 1}
	busy.Begin("Comparing", fm.cancelCompare)
	fm.cancelCompare()
	if ctx.Err() != context.Canceled || busy.Active() {
		t.Fatalf("canceled=%v busy=%t", ctx.Err(), busy.Active())
	}
	if fm.compareGeneration == 1 {
		t.Fatal("queued completion still has a valid generation")
	}
	// A late/repeated cancel must not release an unrelated operation's guard.
	busy.Begin("Loading", nil)
	fm.cancelCompare()
	if !busy.Active() {
		t.Fatal("repeated comparison cancel released another operation")
	}
	busy.End()
}
