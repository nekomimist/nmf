package keymanager

import (
	"testing"

	"fyne.io/fyne/v2"

	"nmf/internal/config"
)

func TestBindingDescriptionsResolveOverridesAliasesAndInvalidEntries(t *testing.T) {
	configured := []config.KeyBindingEntry{
		{Target: "line-edit", Key: "C-A", Command: "invalid.command"},
		{Target: "lineEdit", Key: "C-A", Command: CommandNoop},
		{Target: "lineEdit", Key: "C-A", Command: CommandLineEditCursorEnd},
		{Target: "lineEdit", Key: "KP_Enter", Command: CommandNoop},
		{Target: "lineEdit", Key: "InvalidKey", Command: CommandNoop},
		{Target: "fileViewer", Key: "C-A", Command: CommandFileViewerClose},
	}
	rows := LineEditBindingDescriptions(configured)
	byKey := make(map[string]BindingDescription)
	for _, row := range rows {
		if _, exists := byKey[row.Key]; exists {
			t.Fatalf("duplicate effective key: %s", row.Key)
		}
		byKey[row.Key] = row
	}
	for _, key := range []string{"C-A", "Return"} {
		if row := byKey[key]; row.Command != CommandNoop || !row.Custom {
			t.Fatalf("%s = %+v, want custom noop", key, row)
		}
	}
	if row := byKey["C-E"]; row.Command != CommandLineEditCursorEnd || row.Custom {
		t.Fatalf("C-E = %+v, want default cursor end", row)
	}
	if _, exists := byKey["KP_Enter"]; exists {
		t.Fatal("keypad Enter should be folded to Return")
	}
	if len(rows) != len(defaultLineEditBindings())-1 {
		t.Fatalf("got %d rows, invalid/duplicate entries should not add rows", len(rows))
	}
}

func TestBindingDescriptionsIncludeUserCommandsAndBothViewerPaths(t *testing.T) {
	fm := &mainScreenFakeFileManager{}
	h := NewMainScreenKeyHandlerWithCommands(mainScreenDependenciesForTest(fm), func(string, ...interface{}) {},
		[]config.KeyBindingEntry{{Key: "F6", Command: "user.__key.1"}, {Key: "F7", Command: "user.named"}},
		CommandRegistry{"user.__key.1": func(CommandContext) {}, "user.named": func(CommandContext) {}})
	rows := h.BindingDescriptions(map[string]string{"user.__key.1": "my_action"})
	byKey := make(map[string]BindingDescription)
	for _, row := range rows {
		byKey[row.Key] = row
	}
	if row := byKey["F6"]; row.Description != "my_action" || !row.Custom {
		t.Fatalf("generated command = %+v", row)
	}
	if row := byKey["F7"]; row.Description != "user.named" {
		t.Fatalf("named command = %+v", row)
	}
	viewer := FileViewerBindingDescriptions([]config.KeyBindingEntry{
		{Target: "fileViewer", Key: "J", Command: CommandFileViewerPageDown},
	})
	byKey = make(map[string]BindingDescription)
	for _, row := range viewer {
		byKey[row.Key] = row
	}
	for key, command := range map[string]string{"J": CommandFileViewerPageDown, "Down": CommandFileViewerLineDown, "S-G": CommandFileViewerEnd} {
		if row := byKey[key]; row.Command != command {
			t.Fatalf("viewer %s = %+v, want %s", key, row, command)
		}
	}
}

func TestAllBuiltinCommandsHaveHelpDescriptions(t *testing.T) {
	main := newMainScreenKeyHandlerForTest(&mainScreenFakeFileManager{}, func(string, ...interface{}) {})
	for _, commands := range []map[string]func(){
		NewLineEditDialogKeyHandler(&fakeLineEditDialog{}, nil).commands,
		NewFileViewerKeyHandler(&fakeFileViewer{}, nil).commands,
	} {
		for id := range commands {
			if commandDescriptions[id] == "" {
				t.Errorf("missing description for %s", id)
			}
		}
	}
	for id := range main.commands {
		if commandDescriptions[id] == "" {
			t.Errorf("missing description for %s", id)
		}
	}
}

func TestHelpBindingIsConfigurableInEveryMode(t *testing.T) {
	for _, target := range []string{KeyBindingTargetMain, KeyBindingTargetLineEdit, KeyBindingTargetFileViewer} {
		t.Run(target, func(t *testing.T) {
			configured := []config.KeyBindingEntry{
				{Target: target, Key: "F1", Command: CommandNoop},
				{Target: target, Key: "F6", Command: CommandDescribeBindings},
			}
			calls := 0
			var handler KeyHandler
			switch target {
			case KeyBindingTargetMain:
				h := newMainScreenKeyHandlerForTest(&mainScreenFakeFileManager{}, func(string, ...interface{}) {}, configured)
				h.SetActions(DialogActions{ShowBindingHelp: func() { calls++ }})
				handler = h
			case KeyBindingTargetLineEdit:
				h := NewLineEditDialogKeyHandler(&fakeLineEditDialog{}, nil, configured)
				h.SetHelpAction(func() { calls++ })
				handler = h
			case KeyBindingTargetFileViewer:
				h := NewFileViewerKeyHandler(&fakeFileViewer{}, nil, configured)
				h.SetHelpAction(func() { calls++ })
				handler = h
			}
			for _, key := range []fyne.KeyName{fyne.KeyF1, fyne.KeyF6} {
				if !handler.OnKeyActivated(&fyne.KeyEvent{Name: key}, ModifierState{}) {
					t.Fatalf("%s was not handled", key)
				}
			}
			if calls != 1 {
				t.Fatalf("help calls = %d, want 1", calls)
			}
		})
	}
}

func TestBindingHelpBridgeRequiresFreshInputAndQueuesOnce(t *testing.T) {
	km := NewKeyManager(func(string, ...interface{}) {})
	var queued []func()
	km.queueOnMain = func(action func()) { queued = append(queued, action) }
	calls := 0
	km.SetBindingHelpAction(func(string) { calls++ })
	km.ShowBindingHelp(KeyBindingTargetLineEdit)
	if len(queued) != 0 {
		t.Fatal("help should not open before a fresh input event")
	}
	km.HandleKeyDown(&fyne.KeyEvent{Name: fyne.KeyF1})
	km.ShowBindingHelp(KeyBindingTargetLineEdit)
	km.ShowBindingHelp(KeyBindingTargetLineEdit)
	if len(queued) != 1 {
		t.Fatalf("queued %d help transitions, want 1", len(queued))
	}
	queued[0]()
	km.ShowBindingHelp(KeyBindingTargetLineEdit)
	if calls != 1 || len(queued) != 1 {
		t.Fatal("held-key repeats should not reopen help after the owner changes")
	}
}
