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
