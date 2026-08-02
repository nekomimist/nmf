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

func TestApplyPlatformHintUsesEnv(t *testing.T) {
	// applyPlatformHint reads the env directly; exercise it end to end (minus
	// the actual glfw.InitHint call, which is untestable without a GLFW
	// context) by checking it doesn't panic and logs via debugPrint for both
	// the hinted and unhinted cases.
	tests := []struct {
		name string
		env  string
	}{
		{name: "x11", env: "x11"},
		{name: "wayland", env: "wayland"},
		{name: "unset", env: ""},
		{name: "unrecognized", env: "bogus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(platformEnvKey, tt.env)
			called := false
			applyPlatformHint(func(string, ...interface{}) { called = true })
			if !called {
				t.Fatalf("applyPlatformHint(%q) did not call debugPrint", tt.env)
			}
		})
	}
}
