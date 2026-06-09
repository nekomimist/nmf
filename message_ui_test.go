package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"nmf/internal/keymanager"
)

func TestShowMessageDialogDefersUntilKeysReleased(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	fm := &FileManager{keyManager: km}
	ran := false

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyE})
	fm.showMessageDialog(func() {
		ran = true
	})

	if ran {
		t.Fatal("message show should be deferred while a key is pressed")
	}

	km.HandleKeyUp(&fyne.KeyEvent{Name: fyne.KeyE})

	if !ran {
		t.Fatal("message show should run after all keys are released")
	}
}

func TestShowMessageDialogRunsAfterExternalOpenForceRelease(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	fm := &FileManager{keyManager: km}
	ran := false

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	fm.forceReleaseKeysAfterExternalOpen("test.open-error")
	fm.showMessageDialog(func() {
		ran = true
	})

	if !ran {
		t.Fatal("message show should run after external open force release")
	}
}

func TestShowMessageDialogRunsImmediatelyWithoutKeyManager(t *testing.T) {
	fm := &FileManager{}
	ran := false

	fm.showMessageDialog(func() {
		ran = true
	})

	if !ran {
		t.Fatal("message show should run immediately without a key manager")
	}
}

func TestVersionDialogMessageIncludesAppMetadata(t *testing.T) {
	oldVersion := version
	version = "test-version"
	t.Cleanup(func() {
		version = oldVersion
	})

	got := versionDialogMessage()
	want := strings.Join([]string{
		"Software: Nekomimist Filer (nmf)",
		"Repository: https://github.com/nekomimist/nmf",
		"Version: test-version",
	}, "\n")
	if got != want {
		t.Fatalf("versionDialogMessage() = %q, want %q", got, want)
	}
}
