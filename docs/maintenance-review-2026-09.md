# Maintenance Review — September 2026

The review started at `f350006` on 2026-09-13. The existing runtime, browser,
watcher, and UI ownership boundaries remain appropriate for maintenance.
Changes focused on file-operation guarantees, measured allocation costs, and
specific duplication. No additional service layer or dependency was required.

## Completed changes

Priority 1 covers data integrity, priority 2 covers reliability and measured
performance, and priority 3 covers maintenance cost.

| Priority | Finding | Resolution | Commit |
| --- | --- | --- | --- |
| 1 | Fixed `.part` names could truncate unrelated files | Create unique temporary output exclusively | `7001993` |
| 1 | Copying into a descendant could recursively copy generated output | Reject descendant transfers, including resolved local aliases | `fe51686` |
| 1 | Ensuring an existing directory changed its permissions | Preserve existing directory modes | `fe51686` |
| 1 | Late destination creation could bypass a non-overwrite decision | Refuse replacement during publication and fast move | `2150331` |
| 1 | Extraction could follow destination symlinks outside its root | Confine local extraction with `os.Root`; validate direct SMB parents | `aef0fa3` |
| 1 | Replacement failure could lose the previous SMB or symlink destination | Preserve old SMB output until publication succeeds; stage replacement links first | `39e0df0` |
| 2 | Every file allocated a 1 MiB transfer buffer | Reuse one buffer within a serial job | `5ca955f` |
| 2 | Watcher updates copied the whole list repeatedly and scanned once per change | Merge in one pass, retain owned public snapshots, use compact sort keys | `9fee351` |
| 2 | Directory comparison could not be canceled | Propagate context, connect Escape/close/navigation, discard stale completions | `f7b22eb` |
| 3 | Unsupported platforms ran native-icon workers without useful results | Skip native-icon service construction outside Windows | `6644873` |
| 3 | Unused interfaces/widget code and low-value tests remained | Remove unused code and its tests; move useful loader tests to `browser` | `c1d0735` |
| 3 | Job queueing, paths, and I/O shared one large source file | Separate responsibilities within the existing package | `abd2307` |
| 3 | Transfer and compare dialogs duplicated destination selection | Share the picker and preserve dialog-owned lifecycle handling | `74166e4` |

The new transfer regression tests cover existing temporary files, late conflicts,
copy-to-descendant, existing permissions, extraction-root replacement, and
failed replacement/rollback. Loader cancellation, input-handler ownership,
watcher consistency, and window-close tests were retained. Mechanical job-file
separation also preserved all 115 declarations in an AST comparison.

## Allocation measurements

Measurements used Go 1.26.7 on Linux amd64 (Ryzen 9 7950X3D), with five iterations
per case. Bytes are cumulative allocation per operation, not retained memory.
Timings are indicative local microbenchmarks, not GUI frame-time measurements.

| Operation | Before B/op | After B/op | Before time | After time |
| --- | ---: | ---: | ---: | ---: |
| Copy 100 tiny files in one job | 105,191,465 | 1,298,292 | 21.16 ms | 12.68 ms |
| Sort 100,000 entries | 64,185,368 | 12,812,310 | 22.31 ms | 11.47 ms |
| Modify 1 of 100,000 entries | 26,420,315 | 0 | 10.34 ms | 1.14 ms |
| Modify all 10,000 entries | 2,654,211 | 1,835,673 | 151.95 ms | 1.32 ms |

The watcher cases use name sorting without a filter. The zero-allocation result
applies to that modify-only model update, not the complete UI refresh. Each copy
job still lazily allocates one 1 MiB buffer. Sorting measurements exclude fixture
creation. Benchmarks live beside the implementation:

```sh
go test -tags migrated_fynedo ./internal/jobs -run '^$' -bench '^BenchmarkCopySmallFiles$' -benchmem -benchtime=5x
go test -tags migrated_fynedo ./internal/browser -run '^$' -bench '^(BenchmarkSortFiles|BenchmarkApplyChanges)$' -benchmem -benchtime=5x
```

### Follow-up measurements

The same Go/Linux host was used for the follow-up work. The job-indicator
benchmark creates 100 completed jobs, one running job, and one pending job,
each with the indicated source count. One operation queries all indicated
windows. Ten iterations per case exclude fixture construction and GUI rendering.

| Sources per job | Windows | Before B/op (`List`) | After B/op (`Summary`) |
| ---: | ---: | ---: | ---: |
| 100 | 1 | 223,744 | 0 |
| 100 | 4 | 894,988 | 0 |
| 10,000 | 1 | 16,754,340 | 0 |
| 10,000 | 4 | 67,011,804 | 0 |

`Manager.Summary()` reads status under the existing locks without copying
source, result, or failure slices. The jobs window still uses detailed
snapshots; the zero-allocation claim applies only to indicator state retrieval.

Directory-cache measurements instead report **retained heap bytes after GC**,
using one iteration with eight distinct directories per window. Entries have
synthetic names and paths, with the original listing slices collected before
measurement. They exclude active browser/UI listings and measure neither peak
RSS nor total allocations.

| Files per directory | Windows | Before retained bytes | After retained bytes |
| ---: | ---: | ---: | ---: |
| 10,000 | 1 | 12,850,304 | 12,844,728 |
| 10,000 | 4 | 51,363,560 | 51,369,232 |
| 100,000 | 1 | 128,075,040 | 16,012,056 |
| 100,000 | 4 | 512,272,824 | 64,033,160 |

The cache now retains at most 100,000 file records per window as well as at
most eight paths. It evicts the oldest snapshots before cloning a new listing.
A listing above the record limit is displayed normally but not cached; it also
invalidates any previously cached version of that path. TTL remains two minutes
with lazy expiration. This is a record-count bound, not a strict byte limit;
longer paths consume more memory. Minor differences in the 10,000-file cases
are measurement noise, since all eight listings fit both versions.

```sh
go test -tags migrated_fynedo ./internal/jobs -run '^$' -bench '^BenchmarkJobIndicator$' -benchmem -benchtime=10x
go test -tags migrated_fynedo ./internal/browser -run '^$' -bench '^BenchmarkDirectoryCacheRetention$' -benchtime=1x
```

## Validation and limits

- Linux: `make test-all`, `make test-race`, and
  `go vet -tags migrated_fynedo ./...` passed.
- macOS amd64/arm64: `make test-darwin-compile` passed for `fileinfo`, `jobs`, and
  `watcher`.
- Windows amd64: `make test-windows-wsl` passed for all packages on the Windows
  x64 host, including the root/UI suites and actual junction/symlink tests.
  WSL builds the executables with the pinned Nix toolchain; the adapter runs
  them on Windows with Windows temporary directories. This found two defects
  that compile-only checks missed: deleting an open temporary input during
  exclusive-copy publication, and treating a regular-file directory blocker
  as a missing path. Both were fixed and their existing regressions passed.
- Windows arm64: `make test-windows-compile-arm64` passed for all packages.
  This remains compile-only validation. The new Windows CI job is configured
  to execute filesystem, jobs, comparison, and browser tests; it has not been
  run remotely as part of this local follow-up.
- Live SMB: `bash scripts/test-smb.sh` passed with race detection against
  Samba 4.15.13 in a disposable Docker container. It covers copy roundtrip,
  no-overwrite, replacement, restoration, and retained-backup verification.
  Replacement-failure cases inject errors after real SMB backup operations;
  they are not actual network disconnections. The read-cancellation test
  actually pauses the server and requires read and cleanup to finish while
  it is still paused. Binding the session and share contexts explicitly fixed
  cancellation lost by the SMB library's context defaults.
- Native macOS/Windows arm64 execution, Windows UNC against a live server,
  mounted-filesystem variations, actual disconnects during replacement, and
  interactive GUI performance remain unverified. Cross-compilation and
  injected errors do not establish those runtime guarantees.

See [Testing](testing.md) for repeatable WSL and disposable SMB commands.

The [transfer destination safety contract](architecture/vfs-smb.md#transfer-destination-safety)
records remaining backend limits. Direct SMB extraction uses path operations
and cannot atomically prevent a remote parent being replaced between calls.
SMB replacement is a backup/publish/restore sequence and can leave a named
backup after interruption. Filesystems without no-replace rename can use an
exclusive-copy fallback, during which the final file is visible before all
bytes are written. Failed non-overwrite publication reports an error and
preserves the conflicting file.

## Follow-up results

The four original follow-up items were investigated on 2026-09-13. Concrete
fixes and repeatable validation are complete; the platform/backend limits above
remain explicit rather than being treated as verified.

| Original item | Result | Commits |
| --- | --- | --- |
| Native Windows and live SMB validation | Add WSL execution and Windows CI; fix the two Windows failures; retain SMB session/share cancellation and add disposable server tests | `b587173`, `f72a980`, `1395b19`, `5205fd3`, `f878d60` |
| Remote archive staging cancellation | Propagate context through resolve/open/copy, clean partial downloads on cancellation, and remove the global download mutex | `f742b5f` |
| Jobs indicator allocation | Replace detailed snapshot retrieval with counts and flags after measuring its cost | `90943b3` |
| Directory-cache memory | Add a total file-record budget after measuring retained heap across multiple windows | `ebf0791` |

Archive staging tests verify cancellation, partial-file cleanup, and concurrent
independent downloads. Providers must support context-aware I/O to interrupt
an already blocked read; checks between reads alone cannot force that. Existing
non-context archive entry points continue to use a background context.

Further testing should focus on Windows UNC and actual network interruptions
when that environment is available, then native arm64/macOS when suitable hosts
are available. Interactive performance measurements can follow observed UI delays.
No further structural refactoring is justified by the measurements here.

Keep the current stale-generation checks, defensive public snapshots, and
explicit dialog close paths. They protect concrete lifecycle boundaries and
were not simplification targets.
