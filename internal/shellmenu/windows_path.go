package shellmenu

import (
	"fmt"
	"strings"
)

// windowsShellPath separates a filesystem path into a Shell root and the
// components below it. Components are kept separate so they can be resolved as
// relative PIDLs without presenting a long full path to the Shell.
type windowsShellPath struct {
	root       string
	components []string
	unc        bool
	server     string
	share      string
}

func parseWindowsShellPath(input string) (windowsShellPath, error) {
	p := normalizeWindowsShellPath(input)
	p = strings.ReplaceAll(p, "/", `\`)
	if p == "" {
		return windowsShellPath{}, fmt.Errorf("path is empty")
	}

	lower := strings.ToLower(p)
	if strings.HasPrefix(lower, `\\?\unc\`) {
		p = `\\` + p[len(`\\?\UNC\`):]
	} else if strings.HasPrefix(p, `\\?\`) {
		p = p[len(`\\?\`):]
	}
	if strings.HasPrefix(p, `\\.\`) {
		return windowsShellPath{}, fmt.Errorf("device namespace is unsupported: %s", input)
	}

	if strings.HasPrefix(p, `\\`) {
		parts := splitWindowsPathComponents(strings.TrimPrefix(p, `\\`))
		if len(parts) < 2 {
			return windowsShellPath{}, fmt.Errorf("invalid UNC path: %s", input)
		}
		return windowsShellPath{
			root:       `\\` + parts[0] + `\` + parts[1],
			components: parts[2:],
			unc:        true,
			server:     parts[0],
			share:      parts[1],
		}, nil
	}

	if len(p) >= 3 && p[1] == ':' && p[2] == '\\' {
		return windowsShellPath{
			root:       p[:3],
			components: splitWindowsPathComponents(p[3:]),
		}, nil
	}
	return windowsShellPath{}, fmt.Errorf("path is not absolute: %s", input)
}

// normalizeWindowsShellPath converts smb:// display paths to UNC paths without
// depending on the VFS package. Keeping this conversion here allows Windows
// Shell operations to be reused by fileinfo without an import cycle.
func normalizeWindowsShellPath(input string) string {
	p := strings.TrimSpace(input)
	if !strings.HasPrefix(strings.ToLower(p), "smb://") {
		return p
	}

	authorityAndPath := p[len("smb://"):]
	if at := strings.Index(authorityAndPath, "@"); at >= 0 {
		authorityAndPath = authorityAndPath[at+1:]
	}
	parts := strings.Split(authorityAndPath, "/")
	if len(parts) == 0 || parts[0] == "" {
		return `\\`
	}

	host := strings.ToLower(strings.TrimSpace(parts[0]))
	if host == "wsl$" {
		host = "wsl.localhost"
	}
	if len(parts) == 1 {
		return `\\` + host
	}
	return `\\` + host + `\` + strings.Join(parts[1:], `\`)
}

func splitWindowsPathComponents(path string) []string {
	parts := strings.Split(path, `\`)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (p windowsShellPath) parentAndName() (windowsShellPath, string, error) {
	if len(p.components) == 0 {
		return windowsShellPath{}, "", fmt.Errorf("path has no child item: %s", p.root)
	}
	parent := p
	parent.components = append([]string(nil), p.components[:len(p.components)-1]...)
	return parent, p.components[len(p.components)-1], nil
}

func (p windowsShellPath) sameFolder(other windowsShellPath) bool {
	if p.unc != other.unc || !strings.EqualFold(p.root, other.root) || len(p.components) != len(other.components) {
		return false
	}
	for i, component := range p.components {
		if !strings.EqualFold(component, other.components[i]) {
			return false
		}
	}
	return true
}
