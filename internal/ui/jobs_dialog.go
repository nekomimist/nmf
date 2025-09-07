package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/jobs"
	"nmf/internal/keymanager"
)

// JobsDialog shows background job queue and allows cancel.
type JobsDialog struct {
	list        *widget.List
	bind        binding.StringList
	items       []jobs.JobSnapshot
	selectedIdx int
	selectedID  int64
	details     *widget.Label
	dialog      dialog.Dialog
	sink        *KeySink
	parent      fyne.Window
	km          *keymanager.KeyManager
	debugPrint  func(format string, args ...interface{})
}

func NewJobsDialog(km *keymanager.KeyManager, debugPrint func(format string, args ...interface{})) *JobsDialog {
	jd := &JobsDialog{km: km, debugPrint: debugPrint}
	jd.bind = binding.NewStringList()
	jd.list = widget.NewListWithData(jd.bind,
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(item binding.DataItem, obj fyne.CanvasObject) {
			s, _ := item.(binding.String).Get()
			if l, ok := obj.(*widget.Label); ok {
				l.SetText(s)
			}
		},
	)
	jd.details = widget.NewLabel("")
	jd.details.Wrapping = fyne.TextWrapWord
	jd.list.OnSelected = func(id widget.ListItemID) {
		jd.selectedIdx = int(id)
		if id >= 0 && int(id) < len(jd.items) {
			jd.selectedID = jd.items[id].ID
		}
		jd.updateDetails()
		// keep focus on sink for key flow
		if jd.parent != nil && jd.sink != nil {
			jd.parent.Canvas().Focus(jd.sink)
		}
	}
	return jd
}

func (jd *JobsDialog) ShowDialog(parent fyne.Window) {
	jd.parent = parent

	// Buttons
	cancelBtn := widget.NewButton("Cancel Selected", func() {
		if jd.selectedID != 0 {
			m := jobs.GetManager()
			_ = m.Cancel(jd.selectedID)
			// refresh soon
			jd.refresh()
		}
	})
	closeBtn := widget.NewButton("Close", func() {
		if jd.dialog != nil {
			jd.dialog.Hide()
		}
	})

	// header and layout
	header := widget.NewLabel("Job Queue")
	header.TextStyle.Bold = true
	jd.list.Resize(fyne.NewSize(680, 320))
	split := container.NewVSplit(jd.list, container.NewVScroll(jd.details))
	split.Offset = 0.7
	// Fix size by wrapping in WithoutLayout and explicitly resizing
	fixed := container.NewWithoutLayout(split)
	fixed.Resize(fyne.NewSize(720, 420))
	split.Resize(fyne.NewSize(720, 420))
	bottom := container.NewHBox(layout.NewSpacer(), cancelBtn, closeBtn)
	content := container.NewBorder(container.NewVBox(header), bottom, nil, nil, fixed)

	// wrap with KeySink and show dialog
	m := jobs.GetManager()
	// subscribe for updates and refresh on UI thread
	m.Subscribe(func() { fyne.Do(jd.refresh) })
	jd.sink = NewKeySink(content, jd.km, WithTabCapture(true))
	jd.dialog = dialog.NewCustomWithoutButtons("Jobs", jd.sink, parent)
	jd.dialog.Resize(fyne.NewSize(720, 480))
	jd.dialog.Show()
	jd.refresh()
	if jd.parent != nil && jd.sink != nil {
		jd.parent.Canvas().Focus(jd.sink)
	}
}

func (jd *JobsDialog) refresh() {
	m := jobs.GetManager()
	snapshots := m.List()
	jd.items = snapshots
	lines := make([]string, len(snapshots))
	for i, it := range snapshots {
		status := string(it.Status)
		when := it.EnqueuedAt
		if it.Status == jobs.StatusRunning && !it.StartedAt.IsZero() {
			when = it.StartedAt
		}
		ts := when.Format("15:04:05")
		lines[i] = fmt.Sprintf("[%s] %s %d/%d → %s  (%s)", ts, string(it.Type), it.DoneFiles, it.TotalFiles, it.DestDir, status)
		if it.Status == jobs.StatusFailed && it.Error != "" {
			lines[i] += "  ERROR"
		}
	}
	jd.bind.Set(lines)
	jd.list.Refresh()
	// Keep selection stable
	if jd.selectedIdx >= 0 && jd.selectedIdx < len(lines) {
		jd.list.Select(widget.ListItemID(jd.selectedIdx))
	} else if len(lines) > 0 {
		jd.list.Select(0)
	}
	jd.updateDetails()
}

func (jd *JobsDialog) updateDetails() {
	if jd.selectedIdx < 0 || jd.selectedIdx >= len(jd.items) {
		jd.details.SetText("")
		return
	}
	it := jd.items[jd.selectedIdx]
	b := &strings.Builder{}
	fmt.Fprintf(b, "Job #%d %s → %s\nStatus: %s, %d/%d completed\n", it.ID, string(it.Type), it.DestDir, string(it.Status), it.DoneFiles, it.TotalFiles)
	if it.Status == jobs.StatusFailed {
		if len(it.Failures) > 0 {
			fmt.Fprintln(b, "Failures:")
			for _, f := range it.Failures {
				if f.TopSource != "" {
					fmt.Fprintf(b, "  - item: %s\n", f.TopSource)
				}
				if f.Path != "" {
					fmt.Fprintf(b, "    path: %s\n", f.Path)
				}
				if f.Error != "" {
					fmt.Fprintf(b, "    error: %s\n", f.Error)
				}
			}
		} else if it.Error != "" {
			fmt.Fprintf(b, "Error: %s\n", it.Error)
		}
	}
	jd.details.SetText(b.String())
}
