package ime

import "testing"

func TestEnabledDefaultsTrueAndCanBeToggled(t *testing.T) {
	SetEnabled(true)
	if !Enabled() {
		t.Fatal("Enabled = false, want true")
	}

	SetEnabled(false)
	if Enabled() {
		t.Fatal("Enabled = true, want false")
	}

	SetEnabled(true)
}
