package ui

import (
	"fmt"
	"strings"

	"nmf/internal/config"
	"nmf/internal/keymanager"
)

// newLineEditKeyHandler explicitly wires help for standalone and embedded inputs.
func newLineEditKeyHandler(d keymanager.LineEditDialogInterface, km *keymanager.KeyManager, bindings []config.KeyBindingEntry) *keymanager.LineEditDialogKeyHandler {
	handler := keymanager.NewLineEditDialogKeyHandler(d, km.Debugf, bindings)
	handler.SetHelpAction(func() { km.ShowBindingHelp(keymanager.KeyBindingTargetLineEdit) })
	return handler
}

// BindingHelpMarkdown formats effective bindings with the originating mode first.
func BindingHelpMarkdown(origin string, modes map[string][]keymanager.BindingDescription) string {
	var text strings.Builder
	text.WriteString("# Key Bindings\n\n")
	text.WriteString("Escape: leave a search/line input, or close help and resume your previous input or preview.\n\n")
	text.WriteString("C- = Ctrl, A- = Alt, S- = Shift. J means j; S-J means Shift+j. " +
		"Source: Default or Custom. noop disables a key.\n\n")
	order := []string{keymanager.KeyBindingTargetMain, keymanager.KeyBindingTargetLineEdit, keymanager.KeyBindingTargetFileViewer}
	for i, target := range order {
		if target == origin {
			order = append([]string{target}, append(order[:i], order[i+1:]...)...)
			break
		}
	}
	for _, target := range order {
		fmt.Fprintf(&text, "## %s\n\n", target)
		if target == keymanager.KeyBindingTargetLineEdit {
			text.WriteString("Common configurable editing bindings. Standard text entry, clipboard and " +
				"selection shortcuts are also handled by the input widget. " +
				"Composite dialogs may add their own shortcuts.\n\n")
		}
		if target == keymanager.KeyBindingTargetFileViewer {
			text.WriteString("Bindings for the normal File Viewer. Some actions apply only to images or text panes. " +
				"Use these navigation/search keys to browse help. " +
				"File-list navigation and marking are inactive here; F1 does not stack help. " +
				"Search and line inputs retain standard text-entry keys.\n\n")
		}
		text.WriteString("| Key | Command | Description | Source |\n| --- | --- | --- | --- |\n")
		for _, binding := range modes[target] {
			source := "Default"
			if binding.Custom {
				source = "Custom"
			}
			fmt.Fprintf(&text, "| %s | %s | %s | %s |\n",
				bindingHelpCell(binding.Key), bindingHelpCell(binding.Command), bindingHelpCell(binding.Description), source)
		}
		text.WriteString("\n")
	}
	return text.String()
}

func bindingHelpCell(value string) string {
	// Escape Markdown syntax as well as pipe separators in user-provided names.
	var text strings.Builder
	for _, r := range value {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			text.WriteByte(' ')
		case strings.ContainsRune("\\`*_{}[]<>()#+!|~&", r):
			// The simplified renderer decodes entities but preserves backslash
			// escapes in text nodes, so use entities for literal punctuation.
			fmt.Fprintf(&text, "&#%d;", r)
		default:
			text.WriteRune(r)
		}
	}
	return text.String()
}
