package ui

import (
	"testing"

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
