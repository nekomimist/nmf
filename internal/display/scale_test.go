package display

import "testing"

func TestEffectiveScaleMatchesFyneRounding(t *testing.T) {
	tests := []struct {
		name     string
		user     float32
		system   float32
		detected float32
		want     float32
	}{
		{name: "linux auto low dpi", user: 1, system: scaleAuto, detected: 1, want: 1},
		{name: "linux auto high dpi", user: 1, system: scaleAuto, detected: 2, want: 2},
		{name: "linux user scale high dpi", user: 1.5, system: scaleAuto, detected: 2, want: 3},
		{name: "windows content scale", user: 1, system: 1.5, detected: 1, want: 1.5},
		{name: "windows content and user scale", user: 1.25, system: 1.5, detected: 1, want: 1.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveScale(tt.user, tt.system, tt.detected); got != tt.want {
				t.Fatalf("effectiveScale() = %g, want %g", got, tt.want)
			}
		})
	}
}

func TestUserScaleFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want float32
	}{
		{name: "unset", want: 1},
		{name: "auto", env: "auto", want: 1},
		{name: "explicit", env: "1.5", want: 1.5},
		{name: "zero", env: "0", want: 1},
		{name: "invalid", env: "large", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env == "" {
				t.Setenv(scaleEnvKey, "")
			} else {
				t.Setenv(scaleEnvKey, tt.env)
			}
			if got := userScaleFromEnv(); got != tt.want {
				t.Fatalf("userScaleFromEnv() = %g, want %g", got, tt.want)
			}
		})
	}
}

func TestScaledIntUsesEffectiveScale(t *testing.T) {
	if got := scaledInt(3000, 1.5); got != 2000 {
		t.Fatalf("scaledInt() = %d, want 2000", got)
	}
}
