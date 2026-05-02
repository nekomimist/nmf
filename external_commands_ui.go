package main

import (
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/ui"
)

// ShowExternalCommandMenu displays configured external commands for the
// current cursor item or selected files.
func (fm *FileManager) ShowExternalCommandMenu() {
	targets := fm.collectTargetPaths()
	if len(targets) == 0 {
		ui.ShowCompactMessageDialog(fm.window, "Command", "No file selected.")
		return
	}

	commands := fm.matchingExternalCommands(targets[0])
	if len(commands) == 0 {
		ui.ShowCompactMessageDialog(fm.window, "Command", "No external commands match this file.")
		return
	}

	var dlg dialog.Dialog
	buttons := make([]fyne.CanvasObject, 0, len(commands))
	for _, command := range commands {
		entry := command
		buttons = append(buttons, widget.NewButton(entry.Name, func() {
			if dlg != nil {
				dlg.Hide()
			}
			fm.runExternalCommand(entry, targets)
		}))
	}

	content := container.NewVBox(buttons...)
	dlg = dialog.NewCustom("Run Command", "Cancel", content, fm.window)
	dlg.Show()
}

func (fm *FileManager) matchingExternalCommands(target string) []config.ExternalCommandEntry {
	var matches []config.ExternalCommandEntry
	for _, entry := range fm.config.UI.ExternalCommands {
		if strings.TrimSpace(entry.Name) == "" || strings.TrimSpace(entry.Command) == "" {
			continue
		}
		if externalCommandMatches(target, entry.Extensions) {
			matches = append(matches, entry)
		}
	}
	return matches
}

func externalCommandMatches(target string, extensions []string) bool {
	if len(extensions) == 0 {
		return true
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(target)), ".")
	for _, candidate := range extensions {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		candidate = strings.TrimPrefix(candidate, ".")
		if candidate == "*" || candidate == ext {
			return true
		}
	}
	return false
}

func (fm *FileManager) runExternalCommand(entry config.ExternalCommandEntry, targets []string) {
	args := expandExternalCommandArgs(entry.Args, targets, fm.currentPath)
	cmd := exec.Command(entry.Command, args...)
	if err := cmd.Start(); err != nil {
		debugPrint("FileManager: external command failed command=%s err=%v", entry.Command, err)
		ui.ShowMessageDialog(fm.window, "Command failed", err.Error())
		return
	}
	debugPrint("FileManager: external command started command=%s pid=%d", entry.Command, cmd.Process.Pid)
}

func expandExternalCommandArgs(templates []string, targets []string, dir string) []string {
	if len(templates) == 0 {
		args := make([]string, len(targets))
		copy(args, targets)
		return args
	}

	first := ""
	name := ""
	if len(targets) > 0 {
		first = targets[0]
		name = fileinfo.BaseName(first)
	}

	var args []string
	for _, template := range templates {
		if template == "{files}" {
			args = append(args, targets...)
			continue
		}
		if strings.Contains(template, "{files}") {
			for _, target := range targets {
				args = append(args, strings.ReplaceAll(template, "{files}", target))
			}
			continue
		}
		arg := strings.ReplaceAll(template, "{file}", first)
		arg = strings.ReplaceAll(arg, "{dir}", dir)
		arg = strings.ReplaceAll(arg, "{name}", name)
		args = append(args, arg)
	}
	return args
}
