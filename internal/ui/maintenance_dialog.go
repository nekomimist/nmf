package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/keymanager"
	"nmf/internal/maintenance"
)

type MaintenanceDialog struct {
	state      *config.State
	keyManager *keymanager.KeyManager
	kmToken    keymanager.HandlerToken
	debugPrint func(format string, args ...interface{})

	cursorMemoryCheck      *widget.Check
	navigationHistoryCheck *widget.Check
	skipNetworkCheck       *widget.Check
	skipRemovableCheck     *widget.Check
	skipUnavailableCheck   *widget.Check
	summaryLabel           *widget.Label
	resultList             *widget.List
	resultBinding          binding.StringList
	scanButton             *widget.Button
	applyButton            *widget.Button

	lastScan maintenance.Result
	scanned  bool
	onApply  func(maintenance.Result) (int, error)
	parent   fyne.Window
	dialog   dialog.Dialog
	sink     *KeySink
	closed   bool
	scanning bool

	scanCancel context.CancelFunc
	scanSerial uint64
	plan       func(context.Context, maintenance.Targets, maintenance.Options, maintenance.ClassifyFunc, maintenance.AccessibleContextFunc) (maintenance.Result, error)
	runOnMain  func(func())
}

func NewMaintenanceDialog(state *config.State, km *keymanager.KeyManager, debugPrint func(format string, args ...interface{})) *MaintenanceDialog {
	d := &MaintenanceDialog{
		state:      state,
		keyManager: km,
		debugPrint: debugPrint,
		plan:       maintenance.PlanTargets,
		runOnMain:  fyne.Do,
	}
	d.createWidgets()
	return d
}

func (d *MaintenanceDialog) createWidgets() {
	options := maintenance.DefaultOptions()
	d.cursorMemoryCheck = widget.NewCheck("Cursor Memory", func(bool) { d.invalidateScan() })
	d.cursorMemoryCheck.SetChecked(options.CleanCursorMemory)
	d.navigationHistoryCheck = widget.NewCheck("Navigation History", func(bool) { d.invalidateScan() })
	d.navigationHistoryCheck.SetChecked(options.CleanNavigationHistory)
	d.skipNetworkCheck = widget.NewCheck("Skip network paths", func(bool) { d.invalidateScan() })
	d.skipNetworkCheck.SetChecked(options.SkipNetworkPaths)
	d.skipRemovableCheck = widget.NewCheck("Skip removable media paths", func(bool) { d.invalidateScan() })
	d.skipRemovableCheck.SetChecked(options.SkipRemovablePaths)
	d.skipUnavailableCheck = widget.NewCheck("Skip unavailable volumes", func(bool) { d.invalidateScan() })
	d.skipUnavailableCheck.SetChecked(options.SkipUnavailableVolumes)

	d.summaryLabel = widget.NewLabel("Scan maintenance targets to find inaccessible directories.")
	d.summaryLabel.Wrapping = fyne.TextWrapWord

	d.resultBinding = binding.NewStringList()
	d.resultList = widget.NewListWithData(
		d.resultBinding,
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapOff
			return label
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			value, _ := item.(binding.String).Get()
			if label, ok := obj.(*widget.Label); ok {
				label.SetText(value)
			}
		},
	)

	d.scanButton = dialogAuxButton("Scan", theme.SearchIcon(), d.Scan)
	d.applyButton = dialogConfirmButton("Apply Cleanup", d.Apply)
	d.applyButton.Disable()
}

func (d *MaintenanceDialog) ShowDialog(parent fyne.Window, onApply func(maintenance.Result) (int, error)) {
	d.parent = parent
	d.onApply = onApply

	content := d.createContent()
	// Keep the dialog's keyboard ownership on the sink so Escape remains
	// available throughout a scan instead of focus traversing into a control.
	d.sink = NewKeySink(content, d.keyManager, WithTabCapture(true))

	handler := keymanager.NewMaintenanceDialogKeyHandler(d)
	d.kmToken = d.keyManager.PushHandler(handler)

	d.dialog = dialog.NewCustomWithoutButtons("Maintenance", d.sink, parent)
	d.dialog.SetOnClosed(func() {
		d.Cancel()
	})
	d.dialog.Resize(metricsSize(maintenanceDialogWidth, maintenanceDialogHeight))
	d.dialog.Show()
	if d.parent != nil && d.sink != nil {
		d.parent.Canvas().Focus(d.sink)
	}
}

func (d *MaintenanceDialog) createContent() fyne.CanvasObject {
	tasks := container.NewVBox(
		widget.NewLabel("Cleanup targets"),
		d.cursorMemoryCheck,
		d.navigationHistoryCheck,
	)
	options := container.NewVBox(
		widget.NewLabel("Safety"),
		d.skipNetworkCheck,
		d.skipRemovableCheck,
		d.skipUnavailableCheck,
	)

	listScroll := container.NewScroll(d.resultList)
	listScroll.SetMinSize(metricsSize(maintenanceDialogWidth-60, maintenanceListHeight))

	buttons := dialogButtonBar(
		d.scanButton,
		dialogCancelButton("Close", d.Cancel),
		d.applyButton,
	)

	return container.NewBorder(
		container.NewVBox(
			container.NewGridWithColumns(2, tasks, options),
			widget.NewSeparator(),
			d.summaryLabel,
		),
		buttons,
		nil,
		nil,
		listScroll,
	)
}

func (d *MaintenanceDialog) options() maintenance.Options {
	return maintenance.Options{
		CleanCursorMemory:      d.cursorMemoryCheck.Checked,
		CleanNavigationHistory: d.navigationHistoryCheck.Checked,
		SkipNetworkPaths:       d.skipNetworkCheck.Checked,
		SkipRemovablePaths:     d.skipRemovableCheck.Checked,
		SkipUnavailableVolumes: d.skipUnavailableCheck.Checked,
	}
}

func (d *MaintenanceDialog) Scan() {
	if d.closed || d.scanning {
		return
	}

	targets := maintenance.Snapshot(d.state)
	options := d.options()
	ctx, cancel := context.WithCancel(context.Background())
	d.scanCancel = cancel
	d.scanSerial++
	serial := d.scanSerial
	d.scanning = true
	d.scanned = false
	d.lastScan = maintenance.Result{}
	d.setScanControlsEnabled(false)
	d.summaryLabel.SetText("Scanning maintenance targets... Press Escape to close.")
	d.debugPrint("MaintenanceDialog: Scan started")

	go func() {
		result, err := d.plan(ctx, targets, options, nil, nil)
		d.runOnMain(func() {
			if d.closed || serial != d.scanSerial {
				return
			}
			d.scanning = false
			d.scanCancel = nil
			d.setScanControlsEnabled(true)
			if err != nil {
				d.summaryLabel.SetText(fmt.Sprintf("Scan stopped: %v", err))
				d.debugPrint("MaintenanceDialog: Scan stopped err=%v", err)
				return
			}
			d.lastScan = result
			d.scanned = true
			d.updateResults()
			d.debugPrint("MaintenanceDialog: Scan finished candidates=%d", len(d.lastScan.Candidates))
		})
	}()
}

func (d *MaintenanceDialog) Apply() {
	if d.closed || d.scanning || !d.scanned || len(d.lastScan.Candidates) == 0 {
		return
	}
	if d.onApply == nil {
		return
	}

	removed, err := d.onApply(d.lastScan)
	if err != nil {
		d.summaryLabel.SetText(fmt.Sprintf("Cleanup failed: %v", err))
		return
	}
	d.summaryLabel.SetText(fmt.Sprintf("Cleanup applied. Removed %d entries.", removed))
	d.resultBinding.Set([]string{})
	d.lastScan = maintenance.Result{}
	d.scanned = false
	d.applyButton.Disable()
}

func (d *MaintenanceDialog) Cancel() {
	if d.closed {
		return
	}
	d.closed = true
	d.scanSerial++
	d.scanning = false
	if d.scanCancel != nil {
		d.scanCancel()
		d.scanCancel = nil
	}
	deferDialogClose(d.keyManager, "maintenance.close", func() {
		if d.keyManager != nil {
			d.keyManager.RemoveHandler(d.kmToken)
		}
		if d.dialog != nil {
			d.dialog.Hide()
		}
		unfocusIfDialogOwned(d.parent, d.sink)
	})
}

func (d *MaintenanceDialog) invalidateScan() {
	if d.scanning {
		return
	}
	if d.applyButton != nil {
		d.applyButton.Disable()
	}
	d.scanned = false
	if d.summaryLabel != nil {
		d.summaryLabel.SetText("Options changed. Scan again before applying cleanup.")
	}
}

func (d *MaintenanceDialog) setScanControlsEnabled(enabled bool) {
	checks := []*widget.Check{
		d.cursorMemoryCheck,
		d.navigationHistoryCheck,
		d.skipNetworkCheck,
		d.skipRemovableCheck,
		d.skipUnavailableCheck,
	}
	for _, check := range checks {
		if check == nil {
			continue
		}
		if enabled {
			check.Enable()
		} else {
			check.Disable()
		}
	}
	if d.scanButton != nil {
		if enabled {
			d.scanButton.Enable()
		} else {
			d.scanButton.Disable()
		}
	}
	if d.applyButton != nil {
		d.applyButton.Disable()
	}
}

func (d *MaintenanceDialog) updateResults() {
	lines := make([]string, 0, len(d.lastScan.Candidates))
	for _, candidate := range d.lastScan.Candidates {
		lines = append(lines, fmt.Sprintf("%s: %s", candidate.Task, candidate.Path))
	}
	d.resultBinding.Set(lines)

	d.summaryLabel.SetText(fmt.Sprintf(
		"Scanned %d Cursor Memory entries, %d Navigation History entries. Skipped %d network, %d removable, and %d unavailable-volume paths. Found %d cleanup candidates.",
		d.lastScan.ScannedCursorMemory,
		d.lastScan.ScannedNavigationHistory,
		d.lastScan.SkippedNetwork,
		d.lastScan.SkippedRemovable,
		d.lastScan.SkippedUnavailable,
		len(d.lastScan.Candidates),
	))
	if len(d.lastScan.Candidates) > 0 {
		d.applyButton.Enable()
	} else {
		d.applyButton.Disable()
	}
}
