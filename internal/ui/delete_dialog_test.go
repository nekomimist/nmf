package ui

import (
	"testing"

	"fyne.io/fyne/v2"

	"nmf/internal/keymanager"
)

func TestPermanentDeleteRequiresConfirmWord(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewDeleteConfirmDialog([]string{"file.txt"}, true, km)
	accepted := false
	d.onAccept = func() { accepted = true }

	d.entry.SetText("delete")
	d.ConfirmDelete()

	if accepted {
		t.Fatal("permanent delete should not accept without exact DELETE")
	}
	if d.closed {
		t.Fatal("dialog should remain open after wrong confirmation text")
	}
}

func TestPermanentDeleteAcceptsConfirmWord(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewDeleteConfirmDialog([]string{"file.txt"}, true, km)
	accepted := false
	d.onAccept = func() { accepted = true }

	d.entry.SetText("DELETE")
	d.ConfirmDelete()

	if !accepted {
		t.Fatal("permanent delete should accept exact DELETE")
	}
	if !d.closed {
		t.Fatal("dialog should close after correct confirmation text")
	}
}

func TestPermanentDeleteWrongReturnDoesNotTrapLaterCancel(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewDeleteConfirmDialog([]string{"file.txt"}, true, km)
	d.kmToken = km.PushHandler(keymanager.NewDeleteConfirmDialogKeyHandler(d))

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if d.closed {
		t.Fatal("dialog should remain open after Return without DELETE")
	}

	d.CancelDelete()

	if !d.closed {
		t.Fatal("cancel should close after a wrong Return confirmation")
	}
	if got := km.GetStackSize(); got != 0 {
		t.Fatalf("key handler stack size = %d, want 0", got)
	}
}

func TestPermanentDeleteWrongReturnDoesNotTrapLaterAccept(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewDeleteConfirmDialog([]string{"file.txt"}, true, km)
	accepted := false
	d.onAccept = func() { accepted = true }
	d.kmToken = km.PushHandler(keymanager.NewDeleteConfirmDialogKeyHandler(d))

	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyReturn})
	km.HandleTypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	d.entry.SetText("DELETE")
	d.ConfirmDelete()

	if !accepted {
		t.Fatal("confirm should accept after a wrong Return confirmation")
	}
	if got := km.GetStackSize(); got != 0 {
		t.Fatalf("key handler stack size = %d, want 0", got)
	}
}

func TestTrashDeleteDoesNotRequireConfirmWord(t *testing.T) {
	km := keymanager.NewKeyManager(func(string, ...interface{}) {})
	d := NewDeleteConfirmDialog([]string{"file.txt"}, false, km)
	accepted := false
	d.onAccept = func() { accepted = true }

	d.ConfirmDelete()

	if !accepted {
		t.Fatal("trash delete should accept without confirm word")
	}
}
