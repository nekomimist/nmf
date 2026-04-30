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

- Windows that subscribe to jobs updates must always call returned `unsubscribe` on close.
  - Main window cleanup: `window_lifecycle.go`
- The Jobs view is an app-global singleton window.
  - FileManager Jobs buttons show or focus the same Jobs window.
  - Jobs window cleanup: `internal/ui/jobs_window.go`
  - Last FileManager window cleanup also closes the Jobs window before app quit.
- Directory watcher must be stopped during window shutdown before process exit.

## Failure and Cancellation Semantics

Jobs:

- Pending jobs can be canceled and removed from queue.
- Running job cancellation signals context and transitions to `StatusCanceled`.
- First failed path ends that job as `StatusFailed` with failure details recorded.
- Delete jobs support two modes:
  - `trash`: move each top-level source to the OS trash/recycle bin.
  - `permanent`: recursively remove each top-level source after UI confirmation.
- Permanent delete refuses filesystem roots and SMB share roots. Symlinks are
  deleted as links and are not followed.
- Trash failures are reported as job failures and never fall back to permanent
  deletion automatically.
- Copy/move name collisions are resolved at execution time, immediately before
  writing the destination path.
- Existing files and symlinks are never overwritten by default. A collision can
  be skipped, renamed, auto-suffixed as `name (1).ext`, or used to cancel the
  running job.
- Same-name destination directories are merged recursively. File collisions
  inside the merge still use the collision resolver.
- Copying an item to its own directory is allowed; the exact same destination
  path is treated as a collision and can become an auto-suffixed duplicate.
- Moving an item to its exact current path remains a no-op.

Watcher:

- Read failures during polling are skipped for that cycle.
- Full change channel drops update for that cycle (best-effort behavior).

## Regression Checklist

Before merging lifecycle changes:

- `go test ./internal/watcher ./internal/jobs`
- `go test -race ./internal/watcher ./internal/jobs`
- Verify `Start -> Stop -> Start -> Stop` watcher cycle behavior
- Verify subscribe/unsubscribe behavior for both main window and jobs dialog
