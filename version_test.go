package main

import (
	"runtime/debug"
	"testing"
)

func TestAppVersionFromBuildSettings(t *testing.T) {
	const revision = "d6b711c78c22fa218d6884697c258c2b18277811"
	got := appVersionFromBuildSettings([]debug.BuildSetting{
		{Key: "vcs.revision", Value: revision},
		{Key: "vcs.time", Value: "2026-05-24T12:34:56Z"},
	})
	want := "20260524+" + revision
	if got != want {
		t.Fatalf("appVersionFromBuildSettings() = %q, want %q", got, want)
	}
}

func TestAppVersionFromBuildSettingsRequiresRevisionAndTime(t *testing.T) {
	if got := appVersionFromBuildSettings([]debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}); got != "" {
		t.Fatalf("appVersionFromBuildSettings() = %q, want empty", got)
	}
}
