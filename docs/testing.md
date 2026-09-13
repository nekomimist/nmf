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
