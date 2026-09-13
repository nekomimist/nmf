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

## Validation and limits

- Linux: `make test-all`, `make test-race`, and
  `go vet -tags migrated_fynedo ./...` passed.
- macOS amd64/arm64: `make test-darwin-compile` passed for `fileinfo`, `jobs`, and
  `watcher`.
- Windows amd64/arm64: `make test-windows-compile` and
  `make test-windows-compile-arm64` passed for all packages using the pinned
  Nix environment. These targets compile tests with `-exec=true` and do not
  execute them on Windows.
- No native Windows/macOS execution, live SMB-server test, or interactive GUI
  performance measurement was performed. Cross-compilation does not establish
  runtime correctness on those platforms.

The [transfer destination safety contract](architecture/vfs-smb.md#transfer-destination-safety)
records remaining backend limits. Direct SMB extraction uses path operations
and cannot atomically prevent a remote parent being replaced between calls.
SMB replacement is a backup/publish/restore sequence and can leave a named
backup after interruption. Filesystems without no-replace rename can use an
exclusive-copy fallback, during which the final file is visible before all
bytes are written. Failed non-overwrite publication reports an error and
preserves the conflicting file.

## Follow-up work

These remain investigation candidates, not measured bottlenecks or reproduced
defects from this review:

1. Exercise the new path guarantees on native Windows, including junctions,
   mounted filesystems, and SMB disconnects during replacement. Verify compare
   cancellation against a stalled server.
2. Propagate cancellation through remote archive staging. It currently copies
   the remote archive under `archiveTempMu`; compare cancellation releases the
   UI immediately, but this backend work can outlive it.
3. Measure `Manager.List()` allocation for status indicators with many sources
   and multiple windows. Indicators only need counts and flags, while snapshots
   also copy source/failure/result slices.
4. Measure retained directory-cache memory across several large directories
   before replacing its existing eight-path bound with an entry/byte budget.

Keep the current stale-generation checks, defensive public snapshots, and
explicit dialog close paths. They protect concrete lifecycle boundaries and
were not simplification targets.
