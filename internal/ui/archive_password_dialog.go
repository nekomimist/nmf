package ui

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
		var d dialog.Dialog
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
			if d != nil {
				d.Hide()
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

		cancelBtn := dialogCancelButton("Cancel", func() {
			finish(false)
			if d != nil {
				d.Hide()
			}
		})
		openBtn := dialogConfirmButton("Open", func() {
			finish(true)
			if d != nil {
				d.Hide()
			}
		})
		content := container.NewVBox(
			container.NewBorder(nil, nil, widget.NewLabel(label), nil, passEntry),
			dialogButtonBar(cancelBtn, openBtn),
		)

		d = dialog.NewCustomWithoutButtons(title, content, p.parent)
		d.Resize(metricsSize(archivePasswordDialogWidth, archivePasswordDialogHeight))
		d.Show()
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
