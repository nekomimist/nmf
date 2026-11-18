//go:build linux

package fileinfo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolveDirectoryPath_SMBCanonicalDisplay(t *testing.T) {
	resolved, parsed, err := ResolveDirectoryPath("smb://example.local/share/path/to/dir")
	if err != nil {
		t.Fatalf("expected SMB path parse to succeed: %v", err)
	}
	if parsed.Scheme != SchemeSMB {
		t.Fatalf("expected smb scheme, got %q", parsed.Scheme)
	}
	if !strings.HasPrefix(strings.ToLower(resolved), "smb://example.local/share") {
		t.Fatalf("unexpected SMB resolved path: %q", resolved)
	}
}

func TestCreateTextFilePortableRejectsDirectSMBPath(t *testing.T) {
	_, err := CreateTextFilePortable("smb://example.invalid/share", "note.txt", "hello")
	if err == nil || !strings.Contains(err.Error(), "direct SMB") {
		t.Fatalf("CreateTextFilePortable error = %v, want direct SMB rejection", err)
	}
}

type blockingCredentialPrompt struct {
	started chan context.Context
	unblock chan struct{}
}

func (p blockingCredentialPrompt) Get(ctx context.Context, _, _, _ string) (Credentials, error) {
	p.started <- ctx
	select {
	case <-ctx.Done():
		return Credentials{}, ctx.Err()
	case <-p.unblock:
		return Credentials{}, errors.New("test finished")
	}
}

func TestSMBInteractiveReadsCancelCredentialPrompt(t *testing.T) {
	const path = "smb://nmf-test.invalid/share/notes"
	previous := currentCredentialsProvider()
	store := currentSecretStore()
	SetSecretStore(nil)
	t.Cleanup(func() { SetCredentialsProvider(previous); SetSecretStore(store) })
	methods := map[string]func(context.Context) error{
		"preview": func(ctx context.Context) error { _, err := ReadPreviewFileContext(ctx, path); return err },
		"storage": func(ctx context.Context) error { _, err := StatStoragePortableContext(ctx, path); return err },
		"archive probe": func(ctx context.Context) error {
			if IsSupportedArchiveContext(ctx, path) {
				return errors.New("archive probe succeeded")
			}
			return ctx.Err()
		},
	}
	for name, read := range methods {
		t.Run(name, func(t *testing.T) {
			prompt := blockingCredentialPrompt{make(chan context.Context, 1), make(chan struct{})}
			SetCredentialsProvider(prompt)
			defer close(prompt.unblock)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- read(ctx) }()
			select {
			case request := <-prompt.started:
				if request.Done() == nil {
					t.Fatal("request dropped the caller context")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("credential prompt was not reached")
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("got %v after cancellation", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("credential prompt remained blocked")
			}
		})
	}
}
