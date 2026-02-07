# Watcher and Jobs Lifecycle Contracts

## Directory Watcher Contract

Source: `internal/watcher/watcher.go`.

`DirectoryWatcher` lifecycle rules:

- `Start()` is no-op when already running.
- `Stop()` is idempotent and safe to call multiple times.
- Each `Start()` increments a watcher `runID` generation.
- Background loops discard stale work when generation no longer matches current run.

Concurrency model:

- Lifecycle state is guarded by watcher mutex.
- Change application is decoupled via buffered `changeChan`.
- FileManager access happens through watcher-facing interface methods:
  - `GetCurrentPath`
  - `GetFiles`
  - `UpdateFiles`
  - `RemoveFromSelections`

Polling behavior:

- Default interval is 2 seconds.
- `SetPollInterval` affects the next `Start()` run.

## Jobs Manager Contract

Source: `internal/jobs/manager.go`.

`Manager` model:

- Singleton manager (`GetManager`) with one worker goroutine.
- FIFO queue processing, one running job at a time.
- History retained up to `historyMax`.

Subscription rules:

- `Subscribe` returns an `unsubscribe` closure.
- `unsubscribe` is idempotent.
- Notifications are emitted without holding manager lock.
- UI callbacks must marshal to Fyne main thread (`fyne.Do`) when touching widgets.

## UI Integration Requirements

- Windows/dialogs that subscribe to jobs updates must always call returned `unsubscribe` on close.
  - Main window cleanup: `window_lifecycle.go`
  - Jobs dialog cleanup: `internal/ui/jobs_dialog.go`
- Directory watcher must be stopped during window shutdown before process exit.

## Failure and Cancellation Semantics

Jobs:

- Pending jobs can be canceled and removed from queue.
- Running job cancellation signals context and transitions to `StatusCanceled`.
- First failed path ends that job as `StatusFailed` with failure details recorded.

Watcher:

- Read failures during polling are skipped for that cycle.
- Full change channel drops update for that cycle (best-effort behavior).

## Regression Checklist

Before merging lifecycle changes:

- `go test ./internal/watcher ./internal/jobs`
- `go test -race ./internal/watcher ./internal/jobs`
- Verify `Start -> Stop -> Start -> Stop` watcher cycle behavior
- Verify subscribe/unsubscribe behavior for both main window and jobs dialog
