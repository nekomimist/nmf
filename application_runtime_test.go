package main

import (
	"context"
	"errors"
	"testing"

	"nmf/internal/fileinfo"
	"nmf/internal/jobs"
)

type promptCredentialsProvider struct {
	username string
}

func (p promptCredentialsProvider) Get(_, _, _ string) (fileinfo.Credentials, error) {
	return fileinfo.Credentials{Username: p.username}, nil
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
	creds, err := broker.Get("host", "share", "path")
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
	if _, err := broker.Get("host", "share", "path"); !errors.Is(err, errNoInteractiveWindow) {
		t.Fatalf("credentials error = %v, want errNoInteractiveWindow", err)
	}
	if secondID == firstID {
		t.Fatal("prompt target IDs should be unique")
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
