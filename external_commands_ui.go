package main

import (
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
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
		fm.showExternalCommandPopup(informationalExternalCommandMenuItem("No file selected."))
		return
	}

	commands := fm.matchingExternalCommands(targets[0])
	if len(commands) == 0 {
		fm.showExternalCommandPopup(informationalExternalCommandMenuItem("No external commands match this file."))
		return
	}

	items := make([]*fyne.MenuItem, 0, len(commands))
	for _, command := range commands {
		entry := command
		items = append(items, fyne.NewMenuItem(entry.Name, func() {
			if fm.runExternalCommand(entry, targets) {
				fm.FocusFileList()
			}
		}))
	}

	fm.showExternalCommandPopup(items...)
}

func informationalExternalCommandMenuItem(label string) *fyne.MenuItem {
	return fyne.NewMenuItem(label, func() {})
}

func (fm *FileManager) showExternalCommandPopup(items ...*fyne.MenuItem) {
	if fm.window == nil || fm.window.Canvas() == nil {
		return
	}

	menu := fyne.NewMenu("Run Command", items...)
	widget.ShowPopUpMenuAtPosition(menu, fm.window.Canvas(), fm.externalCommandMenuPosition())
}

func (fm *FileManager) externalCommandMenuPosition() fyne.Position {
	anchor := fm.cursorAnchor
	if anchor.object != nil && anchor.path != "" && anchor.path == fm.cursorPath {
		canvas := fyne.CurrentApp().Driver().CanvasForObject(anchor.object)
		if canvas != nil {
			pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(anchor.object)
			size := anchor.object.Size()
			if size.Width > 0 && size.Height > 0 {
				return pos.AddXY(8, size.Height)
			}
		}
	}

	return fyne.NewPos(8, 8)
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

func (fm *FileManager) runExternalCommand(entry config.ExternalCommandEntry, targets []string) bool {
	args := expandExternalCommandArgs(entry.Args, targets, fm.currentPath)
	cmd := exec.Command(entry.Command, args...)
	if err := cmd.Start(); err != nil {
		debugPrint("FileManager: external command failed command=%s err=%v", entry.Command, err)
		ui.ShowCompactMessageDialog(fm.window, "Command failed", err.Error())
		return false
	}
	debugPrint("FileManager: external command started command=%s pid=%d", entry.Command, cmd.Process.Pid)
	return true
}

func expandExternalCommandArgs(templates []string, targets []string, dir string) []string {
	commandTargets := make([]string, len(targets))
	for i, target := range targets {
		commandTargets[i] = externalCommandArgumentPath(target)
	}
	commandDir := externalCommandArgumentPath(dir)

	if len(templates) == 0 {
		args := make([]string, len(commandTargets))
		copy(args, commandTargets)
		return args
	}

	first := ""
	name := ""
	if len(targets) > 0 {
		first = commandTargets[0]
		name = fileinfo.BaseName(targets[0])
	}

	var args []string
	for _, template := range templates {
		if template == "{files}" {
			args = append(args, commandTargets...)
			continue
		}
		if strings.Contains(template, "{files}") {
			for _, target := range commandTargets {
				args = append(args, strings.ReplaceAll(template, "{files}", target))
			}
			continue
		}
		arg := strings.ReplaceAll(template, "{file}", first)
		arg = strings.ReplaceAll(arg, "{dir}", commandDir)
		arg = strings.ReplaceAll(arg, "{name}", name)
		args = append(args, arg)
	}
	return args
}

func externalCommandArgumentPath(displayPath string) string {
	_, parsed, err := fileinfo.ResolveRead(displayPath)
	if err != nil {
		return fileinfo.NormalizeInputPath(displayPath)
	}
	if parsed.Provider == "local" && parsed.Native != "" {
		return parsed.Native
	}
	if parsed.Scheme == fileinfo.SchemeFile && parsed.Native != "" {
		return parsed.Native
	}
	return fileinfo.NormalizeInputPath(displayPath)
}
