# Testing

Run `make test-all` for the native suite and `make test-race` for concurrency
checks. Use `go vet -tags migrated_fynedo ./...` for repository-wide vetting.

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
they do not execute tests. CI additionally runs filesystem, job, comparison,
and browser tests on a Windows runner without requiring the GUI C toolchain.

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
