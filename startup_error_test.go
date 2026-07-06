package main

import (
	"errors"
	"strings"
	"testing"
)

func TestStartupConfigScriptErrorMessage(t *testing.T) {
	msg := startupConfigScriptErrorMessage(errors.New("executing Starlark config /tmp/init.star: boom"))

	for _, want := range []string{
		"Failed to load init.star.",
		"executing Starlark config /tmp/init.star: boom",
		"nmf will exit.",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("startupConfigScriptErrorMessage() = %q, want %q", msg, want)
		}
	}
}

func TestStartupFailureMessage(t *testing.T) {
	msg := startupFailureMessage("Failed to load config.json", errors.New("open config.json: permission denied"))

	for _, want := range []string{
		"Failed to load config.json",
		"open config.json: permission denied",
		"nmf will exit.",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("startupFailureMessage() = %q, want %q", msg, want)
		}
	}
}
