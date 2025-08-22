package ui

import (
	"errors"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"nmf/internal/fileinfo"
)

// SMBCredentialsProvider prompts the user for SMB credentials.
type SMBCredentialsProvider struct {
	parent fyne.Window
}

func NewSMBCredentialsProvider(parent fyne.Window) *SMBCredentialsProvider {
	return &SMBCredentialsProvider{parent: parent}
}

func (p *SMBCredentialsProvider) Get(host, share, _ string) (fileinfo.Credentials, error) {
	var mu sync.Mutex
	var creds fileinfo.Credentials
	var retErr error
	done := make(chan struct{})

	fyne.Do(func() {
		userEntry := widget.NewEntry()
		passEntry := widget.NewPasswordEntry()
		domainEntry := widget.NewEntry()
		saveCheck := widget.NewCheck("この端末に保存 (keyring)", nil)
		userEntry.SetPlaceHolder("username")
		domainEntry.SetPlaceHolder("domain (optional)")

		form := dialog.NewForm(
			"SMB Login: "+host+"/"+share,
			"Login",
			"Cancel",
			[]*widget.FormItem{
				widget.NewFormItem("Domain", domainEntry),
				widget.NewFormItem("Username", userEntry),
				widget.NewFormItem("Password", passEntry),
				widget.NewFormItem("", saveCheck),
			},
			func(ok bool) {
				defer close(done)
				if !ok {
					mu.Lock()
					retErr = errors.New("login cancelled")
					mu.Unlock()
					return
				}
				mu.Lock()
				creds = fileinfo.Credentials{
					Domain:   domainEntry.Text,
					Username: userEntry.Text,
					Password: passEntry.Text,
					Persist:  saveCheck.Checked,
				}
				mu.Unlock()
			},
			p.parent,
		)
		form.Resize(fyne.NewSize(420, 200))
		form.Show()
	})

	<-done
	mu.Lock()
	defer mu.Unlock()
	return creds, retErr
}
