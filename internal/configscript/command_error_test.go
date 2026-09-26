package configscript

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"nmf/internal/keymanager"
)

func TestRuntimeCommandErrorsNotifyAllEntryPoints(t *testing.T) {
	for _, failure := range []struct {
		name, expression, message string
	}{
		{"configuration", `nmf.window(width = 800)`, "cannot be used while a custom command is running"},
		{"sort", `nmf.sort(by = "size")`, "use temporary=True"},
		{"arguments", `nmf.exec("")`, "exec command must not be empty"},
		{"evaluation", `{}["missing"]`, "missing"},
	} {
		for _, route := range []string{"command", "key", "menu"} {
			for _, logging := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/logging=%t", failure.name, route, logging), func(t *testing.T) {
					dir := t.TempDir()
					module := filepath.Join(dir, "helper.star")
					moduleSource := "def broken(ctx):\n    nmf.set_clipboard(\"before\")\n    " + failure.expression + "\n    nmf.set_clipboard(\"after\")\n"
					if err := os.WriteFile(module, []byte(moduleSource), 0600); err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(dir, FileName)
					source := `load("helper.star", "broken")
def invoke(ctx):
    broken(ctx)
nmf.command("user.broken", invoke)
nmf.key("C-E", fn = invoke)
nmf.menu("audit", title = "Tools")
nmf.menu_item("audit", "Broken action", fn = invoke)
def show(ctx):
    nmf.show_menu("audit")
nmf.command("user.show", show)
`
					if err := os.WriteFile(path, []byte(source), 0600); err != nil {
						t.Fatal(err)
					}
					var logs []string
					opts := Options{}
					if logging {
						opts.DebugPrint = func(format string, args ...interface{}) {
							logs = append(logs, fmt.Sprintf(format, args...))
						}
					}
					rt, err := Load(path, testConfig(), opts)
					if err != nil {
						t.Fatal(err)
					}
					logs = nil
					var queued []func()
					var effects []string
					var items []keymanager.CommandMenuItem
					var action, details string
					notifications := 0
					ctx := keymanager.CommandContext{
						FileManager: &configScriptFakeFileManager{},
						SetClipboard: func(text string) bool {
							effects = append(effects, text)
							return true
						},
						ShowCommandMenu: func(_ string, entries []keymanager.CommandMenuItem) { items = entries },
						ShowCommandError: func(a, d string) {
							notifications++
							action, details = a, d
						},
						DeferTransition: func(_ string, fn func()) { queued = append(queued, fn) },
					}
					wantAction := ""
					switch route {
					case "command":
						wantAction = "Command user.broken"
						rt.Commands["user.broken"](ctx)
					case "key":
						wantAction = "Key C-E"
						rt.Commands["user.__key.1"](ctx)
					case "menu":
						wantAction = `Menu "Tools" > "Broken action"`
						rt.Commands["user.show"](ctx)
						if len(queued) != 1 {
							t.Fatalf("menu transitions = %d", len(queued))
						}
						queued[0]()
						queued = nil
						if len(items) != 1 {
							t.Fatalf("menu items = %d", len(items))
						}
						items[0].Action()
					}
					if notifications != 0 || len(queued) != 1 {
						t.Fatalf("notifications=%d transitions=%d, want one deferred notification", notifications, len(queued))
					}
					queued[0]()
					if notifications != 1 || action != wantAction {
						t.Fatalf("notifications=%d action=%q, want %q", notifications, action, wantAction)
					}
					for _, want := range []string{"Error: ", failure.message, "Location: " + module + ":3:", "in broken", "Traceback", path + ":3:", "in invoke"} {
						if !strings.Contains(details, want) {
							t.Errorf("details missing %q:\n%s", want, details)
						}
					}
					if !reflect.DeepEqual(effects, []string{"before"}) {
						t.Errorf("effects = %v, want only the action before the failure", effects)
					}
					if logging && (len(logs) != 1 || !strings.Contains(logs[0], failure.message)) {
						t.Errorf("debug log = %v, want original failure", logs)
					}
				})
			}
		}
	}
}

func TestCommandErrorDetailsWithoutSourceLocation(t *testing.T) {
	if got := commandErrorDetails(errors.New("evaluation stopped")); got != "Error: evaluation stopped" {
		t.Fatalf("details = %q", got)
	}
}

func TestSuccessfulCommandDoesNotReportError(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("def ok(ctx):\n    nmf.set_clipboard(\"done\")\nnmf.command(\"user.ok\", ok)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	rt, err := Load(path, testConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got string
	rt.Commands["user.ok"](keymanager.CommandContext{
		SetClipboard: func(text string) bool { got = text; return true },
		ShowCommandError: func(string, string) {
			t.Error("successful command reported an error")
		},
	})
	if got != "done" {
		t.Fatalf("clipboard = %q", got)
	}
}
