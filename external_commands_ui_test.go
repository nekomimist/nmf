package main

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"nmf/internal/config"
)

func TestMatchingExternalCommandsAllowsEmptyCommandWhenEditable(t *testing.T) {
	fm := &FileManager{
		config: &config.Config{
			UI: config.UIConfig{
				ExternalCommands: []config.ExternalCommandEntry{
					{Name: "skip empty command", Command: ""},
					{Name: "edit anything", Command: "", Edit: true},
					{Name: "run vim", Command: "vim"},
					{Name: "", Command: "ignored", Edit: true},
				},
			},
		},
	}

	got := fm.matchingExternalCommands("/tmp/a.txt")
	if len(got) != 2 {
		t.Fatalf("matchingExternalCommands() = %+v, want two commands", got)
	}
	if got[0].Name != "edit anything" || got[1].Name != "run vim" {
		t.Fatalf("matchingExternalCommands() = %+v, want editable empty command and vim", got)
	}
}

func TestExternalCommandMatchesExtensions(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		extensions []string
		want       bool
	}{
		{name: "empty matches", target: "/tmp/a.txt", want: true},
		{name: "dot extension", target: "/tmp/a.txt", extensions: []string{".txt"}, want: true},
		{name: "case insensitive", target: "/tmp/a.TXT", extensions: []string{"txt"}, want: true},
		{name: "wildcard", target: "/tmp/a.bin", extensions: []string{"*"}, want: true},
		{name: "mismatch", target: "/tmp/a.bin", extensions: []string{"txt"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := externalCommandMatches(tt.target, tt.extensions); got != tt.want {
				t.Fatalf("externalCommandMatches() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestExpandExternalCommandArgs(t *testing.T) {
	got := expandExternalCommandArgs(
		[]string{"--dir", "{dir}", "--name={name}", "{files}"},
		[]string{"/tmp/a.txt", "/tmp/b.txt"},
		[]string{"/tmp/a.txt", "/tmp/b.txt"},
		"/tmp",
	)
	want := []string{"--dir", "/tmp", "--name=a.txt", "/tmp/a.txt", "/tmp/b.txt"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandExternalCommandArgs() = %#v, want %#v", got, want)
	}
}

func TestExpandExternalCommandArgsWithoutTargets(t *testing.T) {
	got := expandExternalCommandArgs(
		[]string{"--dir", "{dir}", "--file={file}", "--name={name}"},
		nil,
		nil,
		"/tmp",
	)
	want := []string{"--dir", "/tmp", "--file=", "--name="}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandExternalCommandArgs() = %#v, want %#v", got, want)
	}
}

func TestExpandExternalCommandArgsAllFiles(t *testing.T) {
	got := expandExternalCommandArgs(
		[]string{"--current", "{files}", "--all", "{all_files}"},
		[]string{"/tmp/current/a.txt"},
		[]string{"/tmp/left/a.txt", "/tmp/right/b.txt"},
		"/tmp/current",
	)
	want := []string{"--current", "/tmp/current/a.txt", "--all", "/tmp/left/a.txt", "/tmp/right/b.txt"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandExternalCommandArgs() = %#v, want %#v", got, want)
	}
}

func TestExpandExternalCommandCwd(t *testing.T) {
	got := expandExternalCommandCwd(
		"{dir}/sub-{name}",
		[]string{"/tmp/work/a.txt"},
		"/tmp/work",
	)
	want := filepath.Join("/tmp/work", "sub-a.txt")

	if got != want {
		t.Fatalf("expandExternalCommandCwd() = %q, want %q", got, want)
	}
}

func TestExpandExternalCommandCwdDoesNotExpandFiles(t *testing.T) {
	got := expandExternalCommandCwd("{files}", []string{"/tmp/a.txt"}, "/tmp")

	if got != "{files}" {
		t.Fatalf("expandExternalCommandCwd() = %q, want literal {files}", got)
	}
}

func TestExternalCommandWorkingDirectoryLocalPath(t *testing.T) {
	dir := t.TempDir()

	got, ignored := externalCommandWorkingDirectory(dir)

	if ignored {
		t.Fatal("local cwd should not be ignored")
	}
	if got != dir {
		t.Fatalf("externalCommandWorkingDirectory() = %q, want %q", got, dir)
	}
}

func TestExternalCommandWorkingDirectoryEmpty(t *testing.T) {
	got, ignored := externalCommandWorkingDirectory("  ")

	if got != "" || ignored {
		t.Fatalf("externalCommandWorkingDirectory() = %q ignored=%t, want empty false", got, ignored)
	}
}

func TestExternalCommandWorkingDirectoryIgnoresArchivePath(t *testing.T) {
	got, ignored := externalCommandWorkingDirectory("/tmp/archive.zip!/docs")

	if got != "" || !ignored {
		t.Fatalf("externalCommandWorkingDirectory() = %q ignored=%t, want empty true", got, ignored)
	}
}

func TestExternalCommandWorkingDirectoryIgnoresDirectSMBPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows resolves smb:// paths to local UNC paths")
	}
	got, ignored := externalCommandWorkingDirectory("smb://host/share/docs")

	if got != "" || !ignored {
		t.Fatalf("externalCommandWorkingDirectory() = %q ignored=%t, want empty true", got, ignored)
	}
}
