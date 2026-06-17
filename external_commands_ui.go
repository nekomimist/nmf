package main

import (
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
	"nmf/internal/fileinfo"
	"nmf/internal/keymanager"
	"nmf/internal/ui"
)

// ShowExternalCommandMenu displays configured external commands for the
// current cursor item or selected files.
func (fm *FileManager) ShowExternalCommandMenu() {
	targets := fm.collectTargetPaths()
	if len(targets) == 0 {
		fm.showCommandPopup("Run Command", informationalExternalCommandMenuItem("No file selected."))
		return
	}

	commands := fm.matchingExternalCommands(targets[0])
	if len(commands) == 0 {
		fm.showCommandPopup("Run Command", informationalExternalCommandMenuItem("No external commands match this file."))
		return
	}

	items := make([]keymanager.CommandMenuItem, 0, len(commands))
	for _, command := range commands {
		entry := command
		items = append(items, keymanager.CommandMenuItem{
			Label: entry.Name,
			Key:   entry.Key,
			Action: func() {
				if fm.runExternalCommandTemplate(entry.Command, entry.Args, entry.Cwd, targets, entry.Edit) {
					fm.FocusFileList()
				}
			},
		})
	}

	fm.showCommandMenu(items)
}

func informationalExternalCommandMenuItem(label string) *fyne.MenuItem {
	return fyne.NewMenuItem(label, func() {})
}

// ShowCommandMenu displays a generic command menu at the current cursor row.
func (fm *FileManager) ShowCommandMenu(title string, items []keymanager.CommandMenuItem) {
	_ = title
	fm.showCommandMenu(items)
}

func (fm *FileManager) showCommandPopup(title string, items ...*fyne.MenuItem) {
	_ = title
	commandItems := make([]keymanager.CommandMenuItem, 0, len(items))
	for _, item := range items {
		entry := item
		if entry.IsSeparator {
			commandItems = append(commandItems, keymanager.CommandMenuItem{Separator: true})
			continue
		}
		commandItems = append(commandItems, keymanager.CommandMenuItem{
			Label: entry.Label,
			Action: func() {
				if entry.Action != nil {
					entry.Action()
				}
				fm.FocusFileList()
			},
		})
	}
	fm.showCommandMenu(commandItems)
}

func (fm *FileManager) showCommandMenu(items []keymanager.CommandMenuItem) {
	if fm.window == nil || fm.window.Canvas() == nil {
		return
	}

	menu := ui.NewCommandMenu(items, fm.FocusFileList)
	menu.ShowAtPosition(fm.window.Canvas(), fm.externalCommandMenuPosition())
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
		if strings.TrimSpace(entry.Name) == "" || (!entry.Edit && strings.TrimSpace(entry.Command) == "") {
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

func (fm *FileManager) RunExternalCommand(command string, args []string, edit bool, cwd string) bool {
	return fm.runExternalCommandMaybeEdit(command, args, edit, cwd)
}

func (fm *FileManager) runExternalCommandTemplate(command string, argTemplates []string, cwdTemplate string, targets []string, edit bool) bool {
	return fm.runExternalCommandMaybeEdit(
		command,
		expandExternalCommandArgs(argTemplates, targets, fm.collectAllSelectedTargetPaths(), fm.currentPath),
		edit,
		expandExternalCommandCwd(cwdTemplate, targets, fm.currentPath),
	)
}

func (fm *FileManager) runExternalCommandMaybeEdit(command string, args []string, edit bool, cwd string) bool {
	if !edit {
		return fm.runExternalCommand(command, args, cwd)
	}
	line := buildExternalCommandLine(command, args)
	dlg := ui.NewLineEditDialog(ui.LineEditDialogOptions{
		Title:       "Edit Command",
		Prompt:      "Command line:",
		InitialText: line,
		ConfirmText: "Run",
		Width:       760,
	}, fm.keyManager, fm.config.UI.KeyBindings)
	dlg.ShowDialog(fm.window, func(edited string) bool {
		parsedCommand, parsedArgs, err := parseExternalCommandLine(edited)
		if err != nil {
			debugPrint("FileManager: command line parse failed err=%v", err)
			fm.ShowMessageDialog("Command parse failed", err.Error())
			return false
		}
		if strings.TrimSpace(parsedCommand) == "" {
			return false
		}
		if fm.runExternalCommand(parsedCommand, parsedArgs, cwd) {
			fm.FocusFileList()
			return true
		}
		return false
	})
	return false
}

func (fm *FileManager) runExternalCommand(command string, args []string, cwd string) bool {
	cmd := exec.Command(command, args...)
	if dir := fm.resolveExternalCommandCwd(cwd); dir != "" {
		cmd.Dir = dir
	}
	if err := cmd.Start(); err != nil {
		debugPrint("FileManager: external command failed command=%s err=%v", command, err)
		fm.ShowMessageDialog("Command failed", err.Error())
		return false
	}
	debugPrint("FileManager: external command started command=%s pid=%d cwd=%s", command, cmd.Process.Pid, cmd.Dir)
	return true
}

func (fm *FileManager) resolveExternalCommandCwd(cwd string) string {
	dir, ignored := externalCommandWorkingDirectory(cwd)
	if ignored {
		debugPrint("FileManager: external command cwd ignored cwd=%s", cwd)
	}
	return dir
}

func expandExternalCommandArgs(templates []string, targets []string, allSelectedTargets []string, dir string) []string {
	commandTargets := make([]string, len(targets))
	for i, target := range targets {
		commandTargets[i] = fileinfo.CommandArgumentPath(target)
	}
	commandAllSelectedTargets := make([]string, len(allSelectedTargets))
	for i, target := range allSelectedTargets {
		commandAllSelectedTargets[i] = fileinfo.CommandArgumentPath(target)
	}
	commandDir := fileinfo.CommandArgumentPath(dir)

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
		if template == "{all_files}" {
			args = append(args, commandAllSelectedTargets...)
			continue
		}
		if strings.Contains(template, "{all_files}") {
			for _, target := range commandAllSelectedTargets {
				args = append(args, strings.ReplaceAll(template, "{all_files}", target))
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

func expandExternalCommandCwd(template string, targets []string, dir string) string {
	cwd := strings.TrimSpace(template)
	if cwd == "" {
		return ""
	}
	commandDir := fileinfo.CommandArgumentPath(dir)
	first := ""
	name := ""
	if len(targets) > 0 {
		first = fileinfo.CommandArgumentPath(targets[0])
		name = fileinfo.BaseName(targets[0])
	}
	cwd = strings.ReplaceAll(cwd, "{file}", first)
	cwd = strings.ReplaceAll(cwd, "{dir}", commandDir)
	cwd = strings.ReplaceAll(cwd, "{name}", name)
	return cwd
}

func externalCommandWorkingDirectory(cwd string) (string, bool) {
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		return "", false
	}
	if fileinfo.IsArchivePath(trimmed) {
		return "", true
	}
	_, parsed, err := fileinfo.ResolveRead(trimmed)
	if err != nil {
		if fileinfo.IsSMBDisplay(trimmed) {
			return "", true
		}
		return fileinfo.CommandArgumentPath(trimmed), false
	}
	if parsed.Scheme == fileinfo.SchemeArchive {
		return "", true
	}
	if parsed.Scheme == fileinfo.SchemeSMB && parsed.Provider != "local" {
		return "", true
	}
	if parsed.Provider == "local" && parsed.Native != "" {
		return parsed.Native, false
	}
	if parsed.Scheme == fileinfo.SchemeFile && parsed.Native != "" {
		return parsed.Native, false
	}
	return "", true
}

func externalCommandArgumentPath(displayPath string) string {
	return fileinfo.CommandArgumentPath(displayPath)
}
