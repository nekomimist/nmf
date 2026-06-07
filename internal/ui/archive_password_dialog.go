package ui

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/fileinfo"
)

// ArchivePasswordProvider prompts the user for encrypted archive passwords.
type ArchivePasswordProvider struct {
	parent fyne.Window
}

func NewArchivePasswordProvider(parent fyne.Window) *ArchivePasswordProvider {
	return &ArchivePasswordProvider{parent: parent}
}

func (p *ArchivePasswordProvider) GetArchivePassword(ctx context.Context, req fileinfo.ArchivePasswordRequest) (string, error) {
	var mu sync.Mutex
	var password string
	var retErr error
	var finishOnce sync.Once
	done := make(chan struct{})

	fyne.Do(func() {
		passEntry := widget.NewPasswordEntry()
		var form dialog.Dialog
		finish := func(ok bool) {
			finishOnce.Do(func() {
				defer close(done)
				if !ok {
					mu.Lock()
					retErr = errors.New("archive password cancelled")
					mu.Unlock()
					return
				}
				mu.Lock()
				password = passEntry.Text
				mu.Unlock()
			})
		}
		passEntry.SetPlaceHolder("password")
		passEntry.OnSubmitted = func(string) {
			finish(true)
			if form != nil {
				form.Hide()
			}
		}
		passEntry.OnChanged = func(string) {
			setIMEAnchorAtTextEnd(p.parent, passEntry, passEntry.Text, passEntry.TextStyle)
		}
		passEntry.OnCursorChanged = func() {
			setIMEAnchorAtTextEnd(p.parent, passEntry, passEntry.Text, passEntry.TextStyle)
		}

		title := "Archive Password: " + filepath.Base(req.ArchivePath)
		if req.Retry {
			title = "Archive Password Retry: " + filepath.Base(req.ArchivePath)
		}
		label := "Password"
		if req.Format != "" {
			label = req.Format + " Password"
		}

		form = dialog.NewForm(
			title,
			"Open",
			"Cancel",
			[]*widget.FormItem{
				widget.NewFormItem(label, passEntry),
			},
			func(ok bool) {
				finish(ok)
			},
			p.parent,
		)
		form.Resize(metricsSize(archivePasswordDialogWidth, archivePasswordDialogHeight))
		form.Show()
		if p.parent != nil {
			p.parent.Canvas().Focus(passEntry)
		}
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
	}
	mu.Lock()
	defer mu.Unlock()
	return password, retErr
}
