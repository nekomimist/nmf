package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
)

func TestPromptProvidersReturnImmediatelyWhenAlreadyCanceled(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("prompt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "SMB credentials",
			run: func() error {
				_, err := NewSMBCredentialsProvider(window, nil).Get(ctx, "host", "share", "")
				return err
			},
		},
		{
			name: "archive password",
			run: func() error {
				_, err := NewArchivePasswordProvider(window, nil).GetArchivePassword(ctx, fileinfo.ArchivePasswordRequest{ArchivePath: "x.7z"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- tt.run() }()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("prompt error = %v, want context.Canceled", err)
				}
			case <-time.After(time.Second):
				t.Fatal("canceled prompt waited for a UI callback")
			}
		})
	}
}
