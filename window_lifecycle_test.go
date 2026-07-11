package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"fyne.io/fyne/v2/test"

	"nmf/internal/jobs"
)

func TestActiveJobCountCountsOnlyPendingAndRunning(t *testing.T) {
	snaps := []jobs.JobSnapshot{
		{Status: jobs.StatusPending},
		{Status: jobs.StatusRunning},
		{Status: jobs.StatusCompleted},
		{Status: jobs.StatusFailed},
		{Status: jobs.StatusCanceled},
	}

	if got := activeJobCount(snaps); got != 2 {
		t.Fatalf("activeJobCount = %d, want 2", got)
	}
}

func TestWindowCloseNeedsConfirmationOnlyForLastWindow(t *testing.T) {
	tests := []struct {
		name        string
		openWindows int32
		want        bool
	}{
		{name: "no registered window", openWindows: 0, want: true},
		{name: "last window", openWindows: 1, want: true},
		{name: "another window remains", openWindows: 2, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := windowCloseNeedsConfirmation(tt.openWindows); got != tt.want {
				t.Fatalf("windowCloseNeedsConfirmation(%d) = %t, want %t", tt.openWindows, got, tt.want)
			}
		})
	}
}

func TestWindowLifecycleGuardsDuplicateCloseAndConfirmation(t *testing.T) {
	fm := &FileManager{}

	if !fm.beginQuitConfirmation() {
		t.Fatal("first quit confirmation should open")
	}
	if fm.beginQuitConfirmation() {
		t.Fatal("duplicate quit confirmation should be rejected")
	}
	fm.endQuitConfirmation()
	if !fm.beginQuitConfirmation() {
		t.Fatal("quit confirmation should be allowed after cancellation")
	}
	if !fm.beginWindowClose() {
		t.Fatal("first window close should proceed")
	}
	if fm.beginWindowClose() {
		t.Fatal("duplicate window close should be rejected")
	}
	if fm.beginQuitConfirmation() {
		t.Fatal("closed window should not open a quit confirmation")
	}
}

func TestCloseWindowIsIdempotentAndInvalidatesLoad(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	resetFileManagerWindowTestRegistry(t)
	atomic.StoreInt32(&windowCount, 2)
	t.Cleanup(func() {
		atomic.StoreInt32(&windowCount, 0)
	})

	unsubscribed := 0
	transferUnsubscribed := 0
	fm := &FileManager{
		window: app.NewWindow("closing"),
		jobsUnsub: func() {
			unsubscribed++
		},
	}
	if _, installed := fm.installTransferDestinationSubscription(func() {
		transferUnsubscribed++
	}); !installed {
		t.Fatal("transfer subscription should install before close")
	}
	ctx, loadID := fm.beginDirectoryLoad()

	fm.closeWindow()
	fm.closeWindow()

	if got := atomic.LoadInt32(&windowCount); got != 1 {
		t.Fatalf("window count = %d, want 1 after one effective close", got)
	}
	if unsubscribed != 1 {
		t.Fatalf("jobs unsubscribe calls = %d, want 1", unsubscribed)
	}
	if transferUnsubscribed != 1 {
		t.Fatalf("transfer unsubscribe calls = %d, want 1", transferUnsubscribed)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("load context error = %v, want context.Canceled", ctx.Err())
	}
	if fm.directoryLoadActive(loadID) {
		t.Fatal("closing the window should invalidate its directory load")
	}
}

func TestClosedWindowRejectsTransferDestinationSubscription(t *testing.T) {
	fm := &FileManager{}
	if !fm.beginWindowClose() {
		t.Fatal("window close should begin")
	}
	unsubscribed := 0
	if _, installed := fm.installTransferDestinationSubscription(func() {
		unsubscribed++
	}); installed {
		t.Fatal("closed window should reject a new transfer subscription")
	}
	if unsubscribed != 1 {
		t.Fatalf("rejected subscription cleanup calls = %d, want 1", unsubscribed)
	}
}
