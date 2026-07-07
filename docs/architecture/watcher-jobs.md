# Watcher and Jobs Lifecycle Contracts

## Directory Watcher Contract

Source: `internal/watcher/watcher.go` and `internal/watcher/hub.go`.

`DirectoryWatcher` lifecycle rules:

- `Start()` is no-op when already running.
- `Stop()` is idempotent and safe to call multiple times.
- Each `Start()` increments a watcher `runID` generation.
- Background loops discard stale work when generation no longer matches current run.
- `RefreshSnapshot()` resets the per-window baseline from the current
  `FileManager` file list.

Concurrency model:

- Lifecycle state is guarded by watcher mutex.
- Shared path monitoring is owned by `WatchHub`.
- Change application remains per-window and is decoupled via buffered `changeChan`.
- FileManager access happens through watcher-facing interface methods:
  - `GetCurrentPath`
  - `GetFiles`
  - `UpdateFiles`
  - `RemoveFromSelections`
  - `ApplyChanges`
- Detected changes are merged via `ApplyChanges` only, and the watcher invokes
  it inside `fyne.Do`: `fm.files`/`fm.selectedFiles` are otherwise accessed
  without locks by UI-thread code, so the merge must stay confined to the Fyne
  main goroutine. Do not call `GetFiles`/`RemoveFromSelections` from watcher
  background goroutines.
- `ApplyChanges` skips the re-sort for modify-only change sets under
  name/extension sort (a modify event cannot change those keys); adds and
  deletes always re-sort.

Watch behavior:

- Local watchable paths use `github.com/fswatcher/fswatcher` as the primary
  event source.
- One `WatchHub` source is shared by all open windows for the same path.
- Event bursts are debounced before a complete portable directory snapshot is
  read and broadcast to subscribers.
- If watcher creation, path registration, or runtime watcher delivery fails,
  that source falls back to polling.
- Default fallback interval is 2 seconds. `SetPollInterval` affects the next
  `Start()` run.
- `Subscription.Unsubscribe()` detaches its caller from the shared source
  immediately and never blocks. When the unsubscribing caller was the last
  subscriber for that path, `WatchHub` removes the source from its map
  synchronously but tears it down (backend `Remove`/`Close`, or an in-flight
  poll read) on a detached goroutine, since `DirectoryWatcher.Stop()` runs on
  the Fyne main thread during window close and must not block on a slow or
  hung backend. Process exit does not wait for that teardown goroutine; any
  still in flight are abandoned when the process exits.

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
- Failed jobs remain visible in history; selecting a failed job in the Jobs
  window marks that failure as acknowledged so main-window Jobs indicators stop
  error blinking for that job.
- Delete jobs support two modes:
  - `trash`: move each top-level source to the OS trash/recycle bin.
  - `permanent`: recursively remove each top-level source after UI confirmation.
- Permanent delete refuses filesystem roots and SMB share roots. Symlinks are
  deleted as links and are not followed.
- Directory symlinks and Windows junction-like reparse points are navigable in
  the UI when their targets are directories, but copy/move/delete still operate
  on the link itself rather than the target tree.
- Trash failures are reported as job failures and never fall back to permanent
  deletion automatically.
- Copy/move name collisions are resolved at execution time, immediately before
  writing the destination path.
- Existing files and symlinks can be skipped, renamed, auto-suffixed as
  `name (1).ext`, overwritten only when the source is clearly newer, overwritten
  unconditionally, or used to cancel the running job. The interactive default is
  "overwrite if newer"; non-interactive copy/move still auto-suffixes.
- Same-name destination directories are merged recursively. File collisions
  inside the merge still use the collision resolver.
- Copying an item to its own directory is allowed; the exact same destination
  path is treated as a collision and can become an auto-suffixed duplicate.
- Moving an item to its exact current path remains a no-op.
- Move jobs first try a provider rename within the resolved backend/share, then
  fall back to copy plus source deletion when rename is unavailable.

Watcher:

- Read failures during snapshot refresh or polling are skipped for that cycle.
- Failing fswatcher sources fall back to polling for that path source.
- Full change channel drops update for that cycle (best-effort behavior).

## Regression Checklist

Before merging lifecycle changes:

- `go test ./internal/watcher ./internal/jobs`
- `go test -race ./internal/watcher ./internal/jobs`
- Verify `Start -> Stop -> Start -> Stop` watcher cycle behavior
- Verify shared source subscribe/unsubscribe behavior for multiple windows on
  the same path
