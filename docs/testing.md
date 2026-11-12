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

## Isolated Windows GUI smoke test

On a Windows x64 host with MXC IsolationSession support, run:

```powershell
.\scripts\test-gui-windows.ps1
# If Zig is not on PATH:
.\scripts\test-gui-windows.ps1 -Zig C:\tools\zig\zig.exe
```

This also requires the native Go/Zig prerequisites above, x64 Node.js 24.21+
or 26.8+ on a supported release line, npm, and the Windows .NET Framework C#
compiler. The runner checks MXC availability and fails if it is unavailable;
it does not enable Windows features or fall back to the user's desktop.
Run it from the interactive host session with permission to provision MXC
sessions. An agent's command sandbox may require approval for the outer
command. The first run downloads the pinned `@microsoft/mxc-sdk` 1.0.1 and
its locked dependencies into `.cache/gui-windows/sdk`, with npm lifecycle
scripts disabled. Later runs reuse this installation. See Microsoft's
[SDK prerequisites and lifecycle API](https://github.com/microsoft/mxc/blob/main/sdk/node/README.md).

Each invocation creates a fresh SDK-owned user/session, profile, and fixture
containing a directory, an ASCII filename, and a Japanese filename. A helper
checks its user and session before launching NMF or calling any desktop API;
it checks window ownership and foreground state before each `SendInput` call.
The smoke test asserts six states and captures a PNG at each one:

1. Startup with the parent entry selected.
2. Down selects `folder-a`.
3. Ctrl+F opens the filter dialog.
4. Escape closes it and restores the main key handler.
5. Enter opens `folder-a`.
6. Backspace returns to the fixture and restores its cursor.

Assertions poll application state with deadlines. Missing or nearly uniform
images, and an unchanged canvas after an input step, fail the run. These are
behavior and rendering smoke checks, not pixel-perfect visual comparisons;
inspect the saved images when reviewing UI
changes. Exit code zero requires successful assertions, artifact collection,
and session cleanup. Failures and SDK timeouts return nonzero.

Artifacts are retained in a unique `.cache/gui-windows/runs/<timestamp>-<id>`:

- `artifacts/01-startup.png` through `06-parent-directory.png`, with matching
  state JSON; intermediate numbered captures also remain for diagnosis.
- `artifacts/result.json`, the NMF debug log, stdout, and stderr.
- `lifecycle.json`, including SDK results and whether deprovisioning completed.
- The generated overlay, test executable, and input helper.

The test reads the native Fyne/OpenGL framebuffer through `Canvas().Capture()`.
Ordinary desktop capture and `PrintWindow` returned black client areas in the
initial IsolationSession experiment. The capture bridge is inserted using Go's
`-overlay` only for this test build. No application source files are rewritten,
and normal Windows, Linux, and cross builds do not include the bridge or require
Node.js/MXC. The generated executable is a test artifact, not a release binary.
The overlay preparation fails explicitly if the expected startup anchor changes.

The runner terminates its disposable NMF process, saves evidence, then stops
and deprovisions the session, including after test failure. Ctrl+C requests
cleanup after the bounded SDK command finishes; allow it to complete. If the
outer process is forcibly terminated or cleanup fails, recover using that run's
recorded container identity:

```powershell
.\scripts\test-gui-windows.ps1 -CleanupRun .cache\gui-windows\runs\<timestamp>-<id>
```

This recovery uses the SDK lifecycle API and leaves local artifacts intact.
It does not enumerate or delete unrelated users. IsolationSession currently
requires unrestricted directional networking in the SDK provision request;
the local fixture does not use the network. This workflow covers keyboard
input and the NMF canvas. Mouse injection, IME, native Shell dialogs, desktop
composition, and real multi-monitor/DPI behavior are outside its coverage.
It is an opt-in local test, separate from the headless Windows CI job.

The driver and scenario live under `scripts/gui-windows`. To check the overlay
guard and lifecycle failure paths without creating sessions or sending input:

```powershell
node --test scripts/gui-windows/runner.test.mjs
```

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
