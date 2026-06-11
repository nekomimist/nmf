package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"
)

type fakeJobsDialog struct {
	up     int
	down   int
	top    int
	bottom int
	cancel int
	close  int
}

func (f *fakeJobsDialog) MoveUp()         { f.up++ }
func (f *fakeJobsDialog) MoveDown()       { f.down++ }
func (f *fakeJobsDialog) MoveToTop()      { f.top++ }
func (f *fakeJobsDialog) MoveToBottom()   { f.bottom++ }
func (f *fakeJobsDialog) CancelSelected() { f.cancel++ }
func (f *fakeJobsDialog) CloseDialog()    { f.close++ }

func TestJobsDialogHandlerReturnClosesDialog(t *testing.T) {
	tests := []fyne.KeyName{fyne.KeyReturn, fyne.KeyEnter}
	for _, key := range tests {
		dialog := &fakeJobsDialog{}
		handler := NewJobsDialogKeyHandler(dialog, func(string, ...interface{}) {})

		if !handler.OnKeyActivated(&fyne.KeyEvent{Name: key}, ModifierState{}) {
			t.Fatalf("%s should be handled", key)
		}
		if dialog.close != 1 {
			t.Fatalf("close count = %d, want 1", dialog.close)
		}
		if dialog.cancel != 0 {
			t.Fatalf("cancel count = %d, want 0", dialog.cancel)
		}
	}
}

func TestJobsDialogHandlerDeleteCancelsSelected(t *testing.T) {
	dialog := &fakeJobsDialog{}
	handler := NewJobsDialogKeyHandler(dialog, func(string, ...interface{}) {})

	if !handler.OnKeyActivated(&fyne.KeyEvent{Name: fyne.KeyDelete}, ModifierState{}) {
		t.Fatal("Delete should be handled")
	}
	if dialog.cancel != 1 {
		t.Fatalf("cancel count = %d, want 1", dialog.cancel)
	}
	if dialog.close != 0 {
		t.Fatalf("close count = %d, want 0", dialog.close)
	}
}
