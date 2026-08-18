# Watcher and Jobs Lifecycle Contracts

## Directory Watcher Contract

Source: `internal/watcher/watcher.go` and `internal/watcher/hub.go`.

`DirectoryWatcher` lifecycle rules:

- `Start()` is no-op when already running.
- `Stop()` is idempotent and safe to call multiple times.
- Each `Start()` increments a watcher `runID` generation.
- Background loops discard stale work when generation no longer matches current run.
- `RefreshSnapshot()` resets the per-window baseline from the current
  `FileManager` file list, and wins over any change set already in flight
  (see the baseline generation below).

Concurrency model:

- Lifecycle state is guarded by watcher mutex.
- Shared path monitoring is owned by `WatchHub`.
- Change application remains per-window and is decoupled via buffered `changeChan`.
- A successfully queued snapshot advances the watcher's expected baseline
  immediately, so a burst cannot derive duplicate changes while an earlier UI
  merge is pending. A full queue does not advance the baseline; the next
  snapshot therefore includes the skipped cumulative difference.
- Detection and that advance are separate critical sections with a channel send
  between them, so a `RefreshSnapshot()` can land in the gap. A `baselineGen`
  counter, incremented on every reset and captured during detection, makes the
  advance skip rather than reinstate the older directory read. The reset wins
  and the next read reconciles against it; without this, an entry the UI had
  just created would be missing from the baseline and reported as added again.
- `ApplyChanges` replaces an added path that is already in the list instead of
  appending it, since the baseline excludes entries marked deleted and a
  recreated file therefore arrives as an add.
- When a file filter is active, `browser.Model.ApplyChanges` merges against the
  unfiltered baseline and re-derives the visible list. Merging against the
  filtered list would permanently discard entries hidden at update time.
- FileManager access happens through watcher-facing interface methods:
  - `GetCurrentPath`
  - `GetFiles`
  - `UpdateFiles`
  - `RemoveFromSelections`
  - `ApplyChanges`
- Detected changes are merged via `ApplyChanges` only, and the watcher invokes
  it inside `fyne.DoAndWait`. Browsing data is synchronized by
  `internal/browser.Model`, but `FileManager.ApplyChanges` also refreshes Fyne
  widgets, so the complete operation must stay on the Fyne main goroutine.
  Watcher background work may read through snapshot-returning interface methods
  such as `GetFiles`; it must not touch widgets or retain mutable model state.
- The watcher run generation is checked again inside the UI callback. A
  callback queued by a stopped/restarted run cannot modify the new run's list.
- `ApplyChanges` skips the re-sort for modify-only change sets under
  name/extension sort (a modify event cannot change those keys); adds and
  deletes always re-sort.

Watch behavior:

- Local watchable paths use `github.com/fswatcher/fswatcher` as the primary
  event source.
- One `WatchHub` source is shared by all open windows for the same path.
- Event bursts are debounced before a complete portable directory snapshot is
  read and broadcast to subscribers.
- Each subscriber serializes snapshot delivery with channel close. A broadcast
  may retain a subscriber reference after `Unsubscribe` removes it from the
  source, but that stale delivery observes the closed subscriber and is dropped
  instead of sending to a closed channel.
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

Result recording (`Job.Result`, `internal/jobs/types.go` and `manager.go`):

- A `Result` records one top-level operation's outcome: `Source`, `Destination`,
  `SourceIsDir`, and `DestinationCreated` (source/destination display paths,
  whether the source was a directory, and whether extraction created a new
  root directory that did not already exist).
- Results are appended only for top-level items via `Job.addResult`, never for
  recursively processed children — `copyOrMovePathResolved` passes a `nil`
  result pointer on every recursive call, and permanent-delete recursion never
  calls `addResult`.
- Only items that individually succeed produce a `Result`; a skipped item
  (`errSkipped`) never reaches `addResult`, and a later item's failure aborts
  the job without discarding results already recorded for earlier top-level
  items in that same job.
- Copy/move: recorded only when the top-level source is a directory and the
  resolved destination is non-empty (a same-path no-op clears `Destination`
  and is not recorded).
- Delete: recorded only when the top-level source is a directory, in both
  `trash` and `permanent` modes; deleted files never produce a `Result`.
- Extract: recorded only when the archive's root directory did not already
  exist at the destination (`DestinationCreated: true`); extracting into an
  existing root records nothing.

Completion callback (`Job.OnFinished`, `internal/jobs/types.go`):

- `OnFinished(fn func(JobSnapshot))` registers a one-shot callback invoked
  exactly once when the job reaches a terminal status (`StatusCompleted`,
  `StatusFailed`, `StatusCanceled`).
- If the job has already finished when `OnFinished` is called, `fn` runs
  immediately on a new goroutine (`go fn(snapshot)`).
- Otherwise `fn` is queued on the job and drained by whichever path drives
  that job to a terminal state: `Manager.worker` after a job finishes
  running, or `Manager.Cancel` for a job still pending in the queue.
  Canceling a currently *running* job only cancels its context — the
  worker's own drain still delivers the callback once that job actually
  stops.
- Callbacks are always invoked outside `j.mu` and `m.mu`, but not on any
  single guaranteed goroutine: depending on the path above they run on the
  worker goroutine, on whichever goroutine called `Cancel`, or on an ad-hoc
  goroutine. Consumers must not assume a particular thread and must marshal
  to Fyne's main goroutine themselves (`fyne.Do`) before touching UI state.

## UI Integration Requirements

- Windows that subscribe to jobs updates must always call returned `unsubscribe` on close.
  - Main window cleanup: `window_lifecycle.go`
- The Jobs view is an app-global singleton window.
  - FileManager Jobs buttons show or focus the same Jobs window.
  - Jobs window cleanup: `internal/ui/jobs_window.go`
- In the Jobs window, `Ctrl+C` copies the selected row's summary, a blank line,
  and the exact details text shown for that job.
- Last FileManager window cleanup also closes the Jobs window before app quit.
- Directory watcher must be stopped during window shutdown before process exit.
- Last-window close requests from both the keyboard command and native title
  bar use the same confirmation dialog. Active jobs change the affirmative
  action to explicit `Quit Anyway`; dismissing the dialog keeps the window and
  its jobs subscription alive.
- Confirmed window destruction first calls
  `internal/browser.DirectoryLoader.CancelActive` and ends the window's
  `internal/ui.BusyController`. Queued load completions must fail `Finish`'s
  generation check and must not restart the watcher or restore focus after
  close.
- `FileManager.trackNavigationHistoryJob` (`navigation_history_mutation.go`)
  registers an `OnFinished` callback on every enqueued copy/move/delete/extract
  job to update persisted navigation history once the job's outcome is known,
  driven entirely off `JobSnapshot.Results`: copy and extract add the new
  directory to history, move rebases entries under the old root to the new
  root, and delete removes entries under the deleted root. Enqueue sites are
  `delete_ui.go`, two sites in `jobs_ui.go`, and `drop_ui.go`.
- That callback is registered once per enqueue call and is never unregistered,
  unlike `Manager.Subscribe`'s `unsubscribe` closure that window lifecycle
  code must call on close — `OnFinished` has no unregister path by design,
  since it fires once and is done. This is safe only because `fm.state` is a
  shared `*config.State` pointer handed to every window opened from the same
  process (see the `NewFileManager` call in `navigation_ui.go`), so a
  lingering per-job callback still mutates the one shared state rather than a
  stale copy. Do not assume `OnFinished` is symmetrical with `Subscribe`.

## Failure and Cancellation Semantics

Jobs:

- Pending jobs can be canceled and removed from queue.
- Running job cancellation signals context and transitions to `StatusCanceled`.
- First failed path ends that job as `StatusFailed` with failure details recorded.
- With debug logging enabled, a terminal job failure emits one structured
  `jobs:` line containing the job ID/type, progress, current source,
  destination, delete mode, failing path, and error. A wrapped OS error also
  includes its numeric `errno` (for example, `errno=395`).
- The destination is resolved and validated once, before any source is
  processed (`openTransferDestination`): it must already exist and be a
  directory, and archive paths are rejected outright since archive mutation is
  out of scope. A mistyped destination tree is never created implicitly. The
  execution context that validation opens is shared by every source of the job,
  so a remote backend is dialed once per job rather than once per source.
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
- Watch source path resolution and backend startup run outside the hub-wide
  mutex. Initialization is coordinated per display path, so one slow path does
  not block subscriptions or unsubscriptions for unrelated windows.
- Snapshot delivery clones once per subscriber; ownership transfers to that
  subscriber's watcher when received, avoiding a second baseline map clone.

## Regression Checklist

Before merging lifecycle changes:

- `go vet -tags migrated_fynedo ./...`
- `make test-race`
- `make test-windows-compile`
- `make test-darwin-compile`
- CI additionally runs the full test suite natively on pinned `macos-15` as a
  non-blocking smoke job. macOS is not a supported release target yet, so this
  job remains diagnostic until it is consistently green and the GUI has been
  checked manually. Fyne's Darwin tests cannot be cross-compiled correctly from
  Linux without the macOS SDK; the local Make target only compile-checks the
  production core packages for both `amd64` and `arm64` with CGO disabled.
- Verify `Start -> Stop -> Start -> Stop` watcher cycle behavior
- Verify shared source subscribe/unsubscribe behavior for multiple windows on
  the same path
