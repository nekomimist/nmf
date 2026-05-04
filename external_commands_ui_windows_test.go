//go:build windows

package main

import "testing"

func TestExternalCommandArgumentPathConvertsSMBToUNC(t *testing.T) {
	got := externalCommandArgumentPath("smb://wsl.localhost/Ubuntu/home/neko/src/nmf/main.go")
	want := `\\wsl.localhost\Ubuntu\home\neko\src\nmf\main.go`

	if got != want {
		t.Fatalf("externalCommandArgumentPath() = %q, want %q", got, want)
	}
}
