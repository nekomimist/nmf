package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func TestCompactMessageSinkClosesOnDismissKeyUp(t *testing.T) {
	tests := []fyne.KeyName{fyne.KeyReturn, fyne.KeyEnter, fyne.KeyEscape}
	for _, key := range tests {
		closed := false
		sink := newCompactMessageSink(nil, func() { closed = true })

		sink.KeyDown(&fyne.KeyEvent{Name: key})
		sink.TypedKey(&fyne.KeyEvent{Name: key})
		if closed {
			t.Fatalf("TypedKey(%s) closed dialog before key release", key)
		}

		sink.KeyUp(&fyne.KeyEvent{Name: key})

		if !closed {
			t.Fatalf("KeyUp(%s) did not close dialog", key)
		}
	}
}

func TestCompactMessageSinkTypedKeyStillClosesWithoutKeyDown(t *testing.T) {
	closed := false
	sink := newCompactMessageSink(nil, func() { closed = true })

	sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if !closed {
		t.Fatal("TypedKey(Return) without KeyDown should close dialog")
	}
}

func TestCompactMessageSinkIgnoresCarriedKeyUp(t *testing.T) {
	closed := false
	sink := newCompactMessageSink(nil, func() { closed = true })

	sink.KeyUp(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if closed {
		t.Fatal("KeyUp(Return) without matching KeyDown should not close dialog")
	}
}

func TestCompactMessageSinkIgnoresOtherKeys(t *testing.T) {
	closed := false
	sink := newCompactMessageSink(nil, func() { closed = true })

	sink.TypedKey(&fyne.KeyEvent{Name: fyne.KeyA})

	if closed {
		t.Fatal("non-dismiss key should not close dialog")
	}
}

func TestCompactMessageDialogSizesGrowForLongMessages(t *testing.T) {
	shortMessage, shortDialog := compactMessageDialogSizes("short")
	longMessage, longDialog := compactMessageDialogSizes(strings.Repeat("x", 180))

	if longMessage.Height <= shortMessage.Height {
		t.Fatalf("long message height = %v, want > %v", longMessage.Height, shortMessage.Height)
	}
	if longDialog.Height <= shortDialog.Height {
		t.Fatalf("long dialog height = %v, want > %v", longDialog.Height, shortDialog.Height)
	}
	if longMessage.Width != shortMessage.Width {
		t.Fatalf("message width changed from %v to %v", shortMessage.Width, longMessage.Width)
	}
}

func TestVersionDialogRepositoryValueUsesHyperlinkForURL(t *testing.T) {
	got := versionDialogRepositoryValue("https://github.com/nekomimist/nmf")

	link, ok := got.(*widget.Hyperlink)
	if !ok {
		t.Fatalf("versionDialogRepositoryValue() = %T, want *widget.Hyperlink", got)
	}
	if link.Text != "https://github.com/nekomimist/nmf" {
		t.Fatalf("hyperlink text = %q", link.Text)
	}
	if link.URL.String() != "https://github.com/nekomimist/nmf" {
		t.Fatalf("hyperlink URL = %q", link.URL.String())
	}
	if link.Wrapping != fyne.TextWrapBreak {
		t.Fatalf("hyperlink wrapping = %v, want %v", link.Wrapping, fyne.TextWrapBreak)
	}
}

func TestVersionDialogMessageTextOrder(t *testing.T) {
	got := versionDialogMessageText("Nekomimist Filer (nmf)", "https://github.com/nekomimist/nmf", "20260607+abc")
	want := strings.Join([]string{
		"Software: Nekomimist Filer (nmf)",
		"Repository: https://github.com/nekomimist/nmf",
		"Version: 20260607+abc",
	}, "\n")

	if got != want {
		t.Fatalf("versionDialogMessageText() = %q, want %q", got, want)
	}
}

func TestVersionDialogValueLabelWrapsLongText(t *testing.T) {
	got := versionDialogValueLabel(strings.Repeat("a", 80))

	if got.Wrapping != fyne.TextWrapBreak {
		t.Fatalf("value label wrapping = %v, want %v", got.Wrapping, fyne.TextWrapBreak)
	}
}

func TestVersionDialogSizesGrowForLongVersion(t *testing.T) {
	_, shortDialog := versionDialogSizes("Nekomimist Filer (nmf)", "https://github.com/nekomimist/nmf", "short")
	_, longDialog := versionDialogSizes("Nekomimist Filer (nmf)", "https://github.com/nekomimist/nmf", strings.Repeat("a", 80))

	if longDialog.Height <= shortDialog.Height {
		t.Fatalf("long version dialog height = %v, want > %v", longDialog.Height, shortDialog.Height)
	}
}
