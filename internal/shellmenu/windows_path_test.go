package shellmenu

import (
	"reflect"
	"testing"
)

func TestParseWindowsShellPath(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		root       string
		components []string
		unc        bool
	}{
		{
			name:       "drive path",
			input:      `C:\Users\hiro\file.txt`,
			root:       `C:\`,
			components: []string{"Users", "hiro", "file.txt"},
		},
		{
			name:       "UNC path",
			input:      `\\server\share\directory\file.txt`,
			root:       `\\server\share`,
			components: []string{"directory", "file.txt"},
			unc:        true,
		},
		{
			name:       "extended UNC path",
			input:      `\\?\UNC\server\share\directory\file.txt`,
			root:       `\\server\share`,
			components: []string{"directory", "file.txt"},
			unc:        true,
		},
		{
			name:       "extended drive path",
			input:      `\\?\C:\directory\file.txt`,
			root:       `C:\`,
			components: []string{"directory", "file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseWindowsShellPath(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.root != tt.root || !reflect.DeepEqual(got.components, tt.components) || got.unc != tt.unc {
				t.Fatalf("parseWindowsShellPath(%q) = %+v, want root=%q components=%q unc=%t", tt.input, got, tt.root, tt.components, tt.unc)
			}
		})
	}
}

func TestNormalizeWindowsShellPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "SMB URL",
			input: "smb://naja.local/neko/dir/file.txt",
			want:  `\\naja.local\neko\dir\file.txt`,
		},
		{
			name:  "SMB URL removes credentials",
			input: "smb://domain;user:password@naja.local/neko/dir/file.txt",
			want:  `\\naja.local\neko\dir\file.txt`,
		},
		{
			name:  "WSL host alias",
			input: "smb://wsl$/Ubuntu/home/neko",
			want:  `\\wsl.localhost\Ubuntu\home\neko`,
		},
		{
			name:  "local path",
			input: " C:\\Users\\hiro\\file.txt ",
			want:  `C:\Users\hiro\file.txt`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeWindowsShellPath(tt.input); got != tt.want {
				t.Fatalf("normalizeWindowsShellPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseWindowsShellPathRejectsUnsupportedRoots(t *testing.T) {
	for _, input := range []string{
		`C:relative\file.txt`,
		`\\server`,
		`\\.\PhysicalDrive0`,
	} {
		if _, err := parseWindowsShellPath(input); err == nil {
			t.Fatalf("parseWindowsShellPath(%q) succeeded, want error", input)
		}
	}
}

func TestWindowsShellPathParentAndName(t *testing.T) {
	path, err := parseWindowsShellPath(`\\server\share\one\two\file.txt`)
	if err != nil {
		t.Fatal(err)
	}
	parent, name, err := path.parentAndName()
	if err != nil {
		t.Fatal(err)
	}
	if name != "file.txt" || !reflect.DeepEqual(parent.components, []string{"one", "two"}) {
		t.Fatalf("parentAndName() = (%+v, %q), want components [one two] and file.txt", parent, name)
	}
	if !parent.sameFolder(windowsShellPath{root: `\\SERVER\SHARE`, components: []string{"ONE", "two"}, unc: true}) {
		t.Fatal("sameFolder should compare Windows paths case-insensitively")
	}
}

func TestWindowsShellPathParentAndNameRejectsRoot(t *testing.T) {
	path, err := parseWindowsShellPath(`C:\`)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := path.parentAndName(); err == nil {
		t.Fatal("parentAndName() for a root path succeeded, want error")
	}
}
