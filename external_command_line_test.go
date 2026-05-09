package main

import (
	"reflect"
	"testing"
)

func TestBuildExternalCommandLineQuotesUnsafeWords(t *testing.T) {
	got := buildExternalCommandLine("vim", []string{"/tmp/has space.txt", "plain", "quote's"})
	want := "vim '/tmp/has space.txt' plain 'quote'\\''s'"

	if got != want {
		t.Fatalf("buildExternalCommandLine() = %q, want %q", got, want)
	}
}

func TestBuildExternalCommandLineAllowsEmptyCommand(t *testing.T) {
	got := buildExternalCommandLine("", nil)
	if got != "" {
		t.Fatalf("buildExternalCommandLine() = %q, want empty", got)
	}

	got = buildExternalCommandLine(" ", []string{"echo", "hello world"})
	want := "echo 'hello world'"
	if got != want {
		t.Fatalf("buildExternalCommandLine() = %q, want %q", got, want)
	}
}

func TestParseExternalCommandLine(t *testing.T) {
	command, args, err := parseExternalCommandLine(`vim '/tmp/has space.txt' "two words" plain\ arg`)
	if err != nil {
		t.Fatalf("parseExternalCommandLine returned error: %v", err)
	}
	if command != "vim" {
		t.Fatalf("command = %q, want vim", command)
	}
	wantArgs := []string{"/tmp/has space.txt", "two words", "plain arg"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestParseExternalCommandLineRejectsInvalidLines(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"vim 'unterminated",
		`vim trailing\`,
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if _, _, err := parseExternalCommandLine(tt); err == nil {
				t.Fatal("parseExternalCommandLine returned nil error")
			}
		})
	}
}
