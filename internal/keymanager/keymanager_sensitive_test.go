package keymanager

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func TestSensitiveHandlerSuppressesKeyDiagnostics(t *testing.T) {
	var logs strings.Builder
	km := NewKeyManager(func(format string, args ...interface{}) { fmt.Fprintf(&logs, format, args...) })
	handler := &recordingHandler{}
	token := km.PushSensitiveHandler(handler)
	ev := &fyne.KeyEvent{Name: "private-key"}
	km.HandleKeyDown(ev)
	km.HandleTypedKey(ev)
	km.HandleTypedRune('x')
	km.HandleKeyUp(ev)
	if len(handler.typedKeys) != 1 || len(handler.runes) != 1 {
		t.Fatal("sensitive input was not dispatched")
	}
	if logs.Len() != 0 || strings.Contains(km.DumpState(), string(ev.Name)) {
		t.Fatal("sensitive input produced key diagnostics")
	}
	km.RemoveHandler(token)
	if km.handlers[:cap(km.handlers)][0].handler != nil {
		t.Fatal("removed handler still referenced by stack")
	}
	if strings.Contains(km.DumpState(), string(ev.Name)) || logs.Len() == 0 {
		t.Fatal("key diagnostics were not reset on close")
	}
}
