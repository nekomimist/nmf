# Testing

On Linux/WSL, run `make test-all` for the native suite and `make test-race` for
concurrency checks. Use `go vet -tags migrated_fynedo ./...` for repository-wide
vetting.

## Native Windows development

With Windows Go (at least the version in `go.mod`) and Zig on `PATH`, run:

```powershell
.\scripts\dev-windows.ps1 test
.\scripts\dev-windows.ps1 test -Arguments @('-v', './...')
.\scripts\dev-windows.ps1 test -Arguments @('-run', 'TestResolve', './internal/fileinfo')
.\scripts\dev-windows.ps1 vet
.\scripts\dev-windows.ps1 build
```

The script enables cgo and configures Zig for the Windows Go host architecture.
Without arguments, `test` and `vet` cover `./...`. When supplying `-Arguments`,
include the package paths as well as any Go flags. `run` forwards these
arguments to NMF; `build` forwards them as Go build flags. Commands run from
the repository root, and native command failures propagate as nonzero exit
codes. Use `-Zig` to select an executable outside `PATH`.

Go and Zig caches live under `.cache/go-build-windows`,
`.cache/zig-local-windows`, and `.cache/zig-global-windows`. These absolute
paths are computed from the checkout location and used only while the script
runs; inherited cross-compilation settings and cache paths are restored on exit.
Go modules still use the normal module cache. Windows CI uses the same script
to execute all tests, vet all packages, and build `dist/nmf-dev.exe`. Its Go
version comes from `go.mod`; Zig 0.16.0 is downloaded with a pinned SHA-256
checksum, matching the Zig release in the current Linux Flake. Review both pins
when upgrading.

The `vet` command runs the full analyzer set and propagates failures. Both
Windows and Linux CI require repository-wide vetting; no analyzer is disabled
by the script. Windows Shell tests invoke the COM callback ABI and exercise
hidden native window message forwarding and cleanup without opening a menu.

If Windows PowerShell 5.1 blocks local scripts under its execution policy, invoke
the script in a child process without changing the machine's policy:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-windows.ps1 test
```

The existing Fyne tests use a headless test driver. Passing them does not
validate native rendering, IME, DPI, focus, or Explorer interactions. Review
skips as well: junction and symlink tests need filesystem support and suitable
privileges. The Windows CI runner is x64; native ARM64 execution is not covered
by this job.

Keep race checks on Linux for now. Windows Go 1.27.3 with Zig 0.18.0-dev failed
to link the race runtime's synchronization APIs; explicitly linking those APIs
then exposed a ThreadSanitizer address-allocation failure before tests ran.
Windows `-race` is not a supported part of this script's workflow yet. This
does not affect the ordinary Windows suite. See the
[Go race detector requirements](https://go.dev/doc/articles/race_detector#Requirements).

## Windows execution from WSL

```sh
make test-windows-wsl
make test-windows-wsl WINDOWS_TEST_PACKAGES='./internal/fileinfo ./internal/jobs'
```

This target requires WSL Windows interop (`powershell.exe` and `wslpath`) and
an x64 Windows host. It cross-compiles using the pinned Nix toolchain, then
runs the generated tests on Windows. A Windows Go installation is not required.
Each test executable runs from its own Windows temporary directory, with
`TEMP` and `TMP` set there so filesystem tests exercise the native filesystem.
The package's source directory remains the working directory for test fixtures.
The runner waits for process exit, propagates its status, and removes its
temporary directory.

Review skipped tests as well as failures. In particular, junction and symlink
regressions should run on a host with suitable filesystem support and privileges.
`make test-windows-compile` and `make test-windows-compile-arm64` only compile;
they do not execute tests. CI additionally runs the full suite with the native
Windows Go/Zig toolchain described above.

## Disposable SMB integration tests

```sh
bash scripts/test-smb.sh
```

This Linux/WSL command requires Docker with a bridge reachable from the test
host and network access to build an Ubuntu/Samba fixture. It creates no host
directory shares or published ports. The container, its image tag, and build
directory are removed on exit. Package-layer caches may remain in Docker.

The tests exercise real SMB reads, writes, rename, and backup verification.
Replacement-error cases inject publication or transport errors after a real
backup rename; they do not simulate every possible TCP failure. The cancellation
case actually pauses the disposable server and requires the read and cleanup
to finish while it remains paused. The script runs these tests sequentially.

An existing **dedicated test share** can also be supplied via
`NMF_SMB_TEST_DIR=smb://guest@host/share/path`. Tests create unique child paths
and remove them on completion. `NMF_SMB_TEST_CONTAINER` enables the server-pause
test only when the share host matches that explicitly selected container's IP.
