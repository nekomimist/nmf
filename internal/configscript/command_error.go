package configscript

import (
	"errors"
	"fmt"

	"go.starlark.net/starlark"

	"nmf/internal/keymanager"
)

// runCommand reports evaluation errors at each user-invoked script boundary.
// CommandFunc deliberately keeps its existing void result: nmf.run's return
// value still describes dispatch, not the success of the invoked script.
func (rt *Runtime) runCommand(id, action string, callable starlark.Callable, ctx keymanager.CommandContext) {
	if err := rt.callCommand(id, callable, ctx); err != nil {
		rt.debugPrint("ConfigScript: command failed id=%s err=%s", id, formatStarlarkError(err))
		if ctx.ShowCommandError != nil {
			details := commandErrorDetails(err)
			deferCommandTransition(ctx, "starlark.error", func() {
				ctx.ShowCommandError(action, details)
			})
		}
	}
}

func commandErrorDetails(err error) string {
	summary := "Error: " + err.Error()
	var evalErr *starlark.EvalError
	if !errors.As(err, &evalErr) {
		return summary
	}
	// Built-ins have no user-editable source location. Find the innermost
	// script frame, which may be in a module loaded by init.star.
	for i := len(evalErr.CallStack) - 1; i >= 0; i-- {
		frame := evalErr.CallStack[i]
		if frame.Pos.Filename() != "<builtin>" && frame.Pos.Line > 0 {
			summary += fmt.Sprintf("\nLocation: %s in %s", frame.Pos, frame.Name)
			break
		}
	}
	return summary + "\n\n" + evalErr.Backtrace()
}
