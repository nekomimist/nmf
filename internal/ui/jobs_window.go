package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/jobs"
	"nmf/internal/keymanager"
)

// JobsWindow shows the global background job queue and allows cancel.
type JobsWindow struct {
	list        *widget.List
	bind        binding.StringList
	items       []jobs.JobSnapshot
	selectedIdx int
	selectedID  int64
	details     *widget.Label
	window      fyne.Window
	sink        *KeySink
	km          *keymanager.KeyManager
	debugPrint  func(format string, args ...interface{})
	jobsUnsub   func()
	closed      bool
	onClosed    func()
}

func NewJobsWindow(app fyne.App, debugPrint func(format string, args ...interface{})) *JobsWindow {
	jd := &JobsWindow{
		window:      app.NewWindow("Jobs"),
		km:          keymanager.NewKeyManager(debugPrint),
		debugPrint:  debugPrint,
		selectedIdx: -1,
	}
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
		} else {
			jd.selectedID = 0
		}
		jd.updateDetails()
		// keep focus on sink for key flow
		if jd.window != nil && jd.sink != nil {
			jd.window.Canvas().Focus(jd.sink)
		}
	}

	// Buttons
	cancelBtn := widget.NewButton("Cancel Selected", func() { jd.cancelSelected() })
	closeBtn := widget.NewButton("Close", func() {
		jd.Close()
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

	handler := keymanager.NewJobsDialogKeyHandler(jd, jd.debugPrint)
	jd.km.PushHandler(handler)
	jd.sink = NewKeySink(content, jd.km, WithTabCapture(true))

	jd.window.SetContent(jd.sink)
	jd.window.Resize(fyne.NewSize(720, 480))
	jd.window.SetOnClosed(jd.handleClosed)
	return jd
}

func (jd *JobsWindow) Show() {
	if jd.closed {
		return
	}
	if jd.jobsUnsub == nil {
		m := jobs.GetManager()
		jd.jobsUnsub = m.Subscribe(func() {
			fyne.Do(func() {
				if !jd.closed {
					jd.refresh()
				}
			})
		})
	}
	jd.window.Show()
	jd.window.RequestFocus()
	jd.refresh()
	if jd.window != nil && jd.sink != nil {
		jd.window.Canvas().Focus(jd.sink)
	}
}

func (jd *JobsWindow) Window() fyne.Window {
	return jd.window
}

func (jd *JobsWindow) Closed() bool {
	return jd.closed
}

func (jd *JobsWindow) SetOnClosed(fn func()) {
	jd.onClosed = fn
}

func (jd *JobsWindow) refresh() {
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
	// Keep selection stable by job ID.
	selectIdx := -1
	if jd.selectedID != 0 {
		for i, it := range snapshots {
			if it.ID == jd.selectedID {
				selectIdx = i
				break
			}
		}
	}
	if selectIdx == -1 && len(lines) > 0 {
		selectIdx = 0
	}
	if selectIdx >= 0 {
		jd.list.Select(widget.ListItemID(selectIdx))
	} else {
		jd.selectedIdx = -1
		jd.selectedID = 0
	}
	jd.updateDetails()
}

func (jd *JobsWindow) updateDetails() {
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

// Interface methods for key handler
func (jd *JobsWindow) MoveUp() {
	if jd.list != nil && jd.list.Length() > 0 {
		newIdx := jd.selectedIdx - 1
		if newIdx < 0 {
			newIdx = 0
		}
		jd.list.Select(widget.ListItemID(newIdx))
	}
}
func (jd *JobsWindow) MoveDown() {
	if jd.list != nil && jd.list.Length() > 0 {
		max := jd.list.Length() - 1
		newIdx := jd.selectedIdx + 1
		if newIdx > max {
			newIdx = max
		}
		jd.list.Select(widget.ListItemID(newIdx))
	}
}
func (jd *JobsWindow) MoveToTop() {
	if jd.list != nil && jd.list.Length() > 0 {
		jd.list.Select(0)
	}
}
func (jd *JobsWindow) MoveToBottom() {
	if jd.list != nil && jd.list.Length() > 0 {
		jd.list.Select(jd.list.Length() - 1)
	}
}

func (jd *JobsWindow) cancelSelected() {
	if jd.selectedID != 0 {
		m := jobs.GetManager()
		_ = m.Cancel(jd.selectedID)
		fyne.Do(jd.refresh)
	}
}
func (jd *JobsWindow) CancelSelected() { jd.cancelSelected() }

func (jd *JobsWindow) CloseDialog() { jd.Close() }

func (jd *JobsWindow) Close() {
	if jd.closed {
		return
	}
	if jd.window != nil {
		jd.window.Close()
	}
}

func (jd *JobsWindow) handleClosed() {
	if jd.closed {
		return
	}
	jd.closed = true
	if jd.jobsUnsub != nil {
		jd.jobsUnsub()
		jd.jobsUnsub = nil
	}
	if jd.window != nil {
		jd.window.Canvas().Unfocus()
	}
	if jd.onClosed != nil {
		jd.onClosed()
	}
}
