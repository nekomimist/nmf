//go:build linux

package display

import (
	"testing"

	"github.com/go-gl/glfw/v3.4/glfw"
)

func TestPlatformHintFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantHit glfw.Platform
		wantOK  bool
	}{
		{name: "x11", env: "x11", wantHit: glfw.PlatformX11, wantOK: true},
		{name: "wayland", env: "wayland", wantHit: glfw.PlatformWayland, wantOK: true},
		{name: "unset", env: "", wantOK: false},
		{name: "unrecognized", env: "bogus", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint, ok := platformHintFromEnv(tt.env)
			if ok != tt.wantOK {
				t.Fatalf("platformHintFromEnv(%q) ok = %v, want %v", tt.env, ok, tt.wantOK)
			}
			if ok && hint != tt.wantHit {
				t.Fatalf("platformHintFromEnv(%q) hint = %v, want %v", tt.env, hint, tt.wantHit)
			}
		})
	}
}
