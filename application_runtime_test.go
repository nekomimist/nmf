package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
)

type promptCredentialsProvider struct {
	username string
}

func (p promptCredentialsProvider) Get(context.Context, string, string, string) (fileinfo.Credentials, error) {
	return fileinfo.Credentials{Username: p.username}, nil
}

type blockingPromptCredentialsProvider struct {
	started chan struct{}
}

func (p blockingPromptCredentialsProvider) Get(ctx context.Context, _, _, _ string) (fileinfo.Credentials, error) {
	close(p.started)
	<-ctx.Done()
	return fileinfo.Credentials{}, ctx.Err()
}

type promptArchiveProvider struct {
	password string
}

func (p promptArchiveProvider) GetArchivePassword(context.Context, fileinfo.ArchivePasswordRequest) (string, error) {
	return p.password, nil
}

func TestApplicationPromptBrokerUsesActiveWindowAndFallback(t *testing.T) {
	broker := newApplicationPromptBroker()
	firstID, unregisterFirst := broker.Register(applicationPromptTarget{
		smb:     promptCredentialsProvider{username: "first"},
		archive: promptArchiveProvider{password: "first-pass"},
	})
	secondID, unregisterSecond := broker.Register(applicationPromptTarget{
		smb:     promptCredentialsProvider{username: "second"},
		archive: promptArchiveProvider{password: "second-pass"},
	})
	t.Cleanup(unregisterSecond)

	broker.SetActive(firstID, true)
	creds, err := broker.Get(context.Background(), "host", "share", "path")
	if err != nil {
		t.Fatalf("active target credentials: %v", err)
	}
	if creds.Username != "first" {
		t.Fatalf("active target username = %q, want first", creds.Username)
	}

	unregisterFirst()
	password, err := broker.GetArchivePassword(context.Background(), fileinfo.ArchivePasswordRequest{ArchivePath: "x.7z"})
	if err != nil {
		t.Fatalf("fallback target archive password: %v", err)
	}
	if password != "second-pass" {
		t.Fatalf("fallback target password = %q, want second-pass", password)
	}

	unregisterSecond()
	if _, err := broker.Get(context.Background(), "host", "share", "path"); !errors.Is(err, errNoInteractiveWindow) {
		t.Fatalf("credentials error = %v, want errNoInteractiveWindow", err)
	}
	if secondID == firstID {
		t.Fatal("prompt target IDs should be unique")
	}
}

func TestApplicationPromptBrokerUnregisterCancelsPromptAndReleasesSlot(t *testing.T) {
	broker := newApplicationPromptBroker()
	started := make(chan struct{})
	id, unregister := broker.Register(applicationPromptTarget{
		smb: blockingPromptCredentialsProvider{started: started},
	})
	broker.SetActive(id, true)

	done := make(chan error, 1)
	go func() {
		_, err := broker.Get(context.Background(), "host", "share", "path")
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("credentials prompt did not start")
	}
	unregister()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("credentials prompt error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("unregistered credentials prompt did not return")
	}

	nextID, unregisterNext := broker.Register(applicationPromptTarget{
		smb: promptCredentialsProvider{username: "next"},
	})
	t.Cleanup(unregisterNext)
	broker.SetActive(nextID, true)
	creds, err := broker.Get(context.Background(), "host", "share", "path")
	if err != nil || creds.Username != "next" {
		t.Fatalf("next credentials prompt = %+v, %v; want username next", creds, err)
	}
}

func TestApplicationConflictResolverDoesNotCaptureOriginWindow(t *testing.T) {
	broker := newApplicationPromptBroker()
	firstID, unregisterFirst := broker.Register(applicationPromptTarget{
		conflict: func(context.Context, jobs.ConflictRequest) jobs.ConflictResolution {
			return jobs.ConflictResolution{Action: jobs.ConflictSkip}
		},
	})
	broker.SetActive(firstID, true)

	resolver := broker.ResolveConflict
	unregisterFirst()
	secondID, unregisterSecond := broker.Register(applicationPromptTarget{
		conflict: func(context.Context, jobs.ConflictRequest) jobs.ConflictResolution {
			return jobs.ConflictResolution{Action: jobs.ConflictOverwrite}
		},
	})
	t.Cleanup(unregisterSecond)
	broker.SetActive(secondID, true)

	got := resolver(context.Background(), jobs.ConflictRequest{})
	if got.Action != jobs.ConflictOverwrite {
		t.Fatalf("resolver action = %q, want current target action %q", got.Action, jobs.ConflictOverwrite)
	}
}

func TestApplicationPromptBrokerHonorsCanceledWait(t *testing.T) {
	broker := newApplicationPromptBroker()
	broker.prompt <- struct{}{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := broker.ResolveConflict(ctx, jobs.ConflictRequest{})
	broker.release()

	if got.Action != jobs.ConflictCancelJob {
		t.Fatalf("canceled resolver action = %q, want %q", got.Action, jobs.ConflictCancelJob)
	}
}

func TestWindowConflictResolverReturnsWithoutWaitingForUICancel(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resolver := newWindowConflictResolver(app.NewWindow("conflict"), nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan jobs.ConflictResolution, 1)
	go func() {
		done <- resolver(ctx, jobs.ConflictRequest{})
	}()

	select {
	case got := <-done:
		if got.Action != jobs.ConflictCancelJob {
			t.Fatalf("resolver action = %q, want %q", got.Action, jobs.ConflictCancelJob)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled conflict resolver waited for a UI callback")
	}
}
