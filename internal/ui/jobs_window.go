package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
	"nmf/internal/keymanager"
)

const completedJobTargetLimit = 10

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
		jd.acknowledgeSelectedFailure()
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
	detailsScroll := container.NewVScroll(jd.details)
	detailsScroll.SetMinSize(metricsSize(jobsDetailsWidth, jobsDetailsHeight))
	split := container.NewVSplit(dialogListThemeOverride(jd.list), detailsScroll)
	split.Offset = 0.5
	bottom := container.NewHBox(layout.NewSpacer(), cancelBtn, closeBtn)
	content := container.NewBorder(container.NewVBox(header), bottom, nil, nil, split)

	handler := keymanager.NewJobsDialogKeyHandler(jd, jd.debugPrint)
	jd.km.PushHandler(handler)
	jd.sink = NewKeySink(content, jd.km, WithTabCapture(true))

	jd.window.SetContent(jd.sink)
	jd.window.Resize(metricsSize(jobsWindowWidth, jobsWindowHeight))
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
		target := it.DestDir
		if it.Type == jobs.TypeDelete {
			target = string(it.DeleteMode)
		}
		lines[i] = fmt.Sprintf("[%s] %s %d/%d → %s  (%s)", ts, string(it.Type), it.DoneFiles, it.TotalFiles, target, status)
		if summary := runningProgressSummary(it); summary != "" {
			lines[i] += "  " + summary
		}
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
	target := it.DestDir
	if it.Type == jobs.TypeDelete {
		target = string(it.DeleteMode)
	}
	fmt.Fprintf(b, "Job #%d %s → %s\nStatus: %s, %d/%d completed\n", it.ID, string(it.Type), target, string(it.Status), it.DoneFiles, it.TotalFiles)
	if it.Status == jobs.StatusRunning {
		writeRunningProgress(b, it)
	} else if it.Status == jobs.StatusFailed {
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
	} else if it.Status == jobs.StatusCompleted {
		writeCompletedTargets(b, it.Sources)
	}
	jd.details.SetText(b.String())
}

func runningProgressSummary(it jobs.JobSnapshot) string {
	if it.Status != jobs.StatusRunning || it.CurrentFile == "" {
		return ""
	}
	parts := []string{}
	if it.CurrentTotalBytes > 0 {
		parts = append(parts, fmt.Sprintf("%.1f%%", progressPercent(it.CurrentBytes, it.CurrentTotalBytes)))
	}
	if eta := formatETA(it); eta != "" {
		parts = append(parts, "ETA "+eta)
	}
	if len(parts) == 0 {
		return formatBytes(it.CurrentBytes)
	}
	return strings.Join(parts, " ")
}

func writeRunningProgress(b *strings.Builder, it jobs.JobSnapshot) {
	if it.CurrentFile == "" {
		return
	}
	fmt.Fprintf(b, "Current: %s\n", fileinfo.BaseName(it.CurrentFile))
	progress := formatBytes(it.CurrentBytes)
	if it.CurrentTotalBytes > 0 {
		progress += fmt.Sprintf(" / %s (%.1f%%)", formatBytes(it.CurrentTotalBytes), progressPercent(it.CurrentBytes, it.CurrentTotalBytes))
	}
	if rate := bytesPerSecond(it); rate > 0 {
		progress += fmt.Sprintf(", %s/s", formatBytes(int64(rate)))
	}
	if eta := formatETA(it); eta != "" {
		progress += ", ETA " + eta
	}
	fmt.Fprintf(b, "Progress: %s\n", progress)
}

func progressPercent(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	return float64(done) * 100 / float64(total)
}

func bytesPerSecond(it jobs.JobSnapshot) float64 {
	if it.CurrentBytes <= 0 || it.CurrentStartedAt.IsZero() {
		return 0
	}
	end := it.CurrentUpdatedAt
	if end.IsZero() {
		end = time.Now()
	}
	elapsed := end.Sub(it.CurrentStartedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(it.CurrentBytes) / elapsed
}

func formatETA(it jobs.JobSnapshot) string {
	if it.CurrentTotalBytes <= 0 || it.CurrentBytes <= 0 || it.CurrentBytes >= it.CurrentTotalBytes {
		return ""
	}
	rate := bytesPerSecond(it)
	if rate <= 0 {
		return ""
	}
	remaining := float64(it.CurrentTotalBytes-it.CurrentBytes) / rate
	if remaining < 0 {
		return ""
	}
	return formatDuration(time.Duration(remaining * float64(time.Second)))
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "00:01"
	}
	total := int(d.Round(time.Second).Seconds())
	hours := total / 3600
	minutes := (total % 3600) / 60
	seconds := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB", "PiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f EiB", value/unit)
}

func (jd *JobsWindow) acknowledgeSelectedFailure() {
	if jd.selectedIdx < 0 || jd.selectedIdx >= len(jd.items) {
		return
	}
	it := jd.items[jd.selectedIdx]
	if it.Status != jobs.StatusFailed || it.FailureAcknowledged {
		return
	}
	if jobs.GetManager().AcknowledgeFailure(it.ID) {
		jd.items[jd.selectedIdx].FailureAcknowledged = true
	}
}

func writeCompletedTargets(b *strings.Builder, sources []string) {
	if len(sources) == 0 {
		return
	}
	fmt.Fprintln(b, "Targets:")
	limit := len(sources)
	if limit > completedJobTargetLimit {
		limit = completedJobTargetLimit
	}
	for _, src := range sources[:limit] {
		fmt.Fprintf(b, "  - %s\n", fileinfo.BaseName(src))
	}
	if more := len(sources) - limit; more > 0 {
		fmt.Fprintf(b, "  ... and %d more\n", more)
	}
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
