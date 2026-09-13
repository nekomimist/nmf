#!/usr/bin/env bash
# go test -exec adapter: build in WSL, execute tests on Windows/NTFS.
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "$0")" && pwd)
executable=$(wslpath -w "$1")
shift
workdir=$(wslpath -w "$PWD")
test_args=()
for arg in "$@"; do
    case "$arg" in
        -test.testlogfile=*|-test.coverprofile=*|-test.gocoverdir=*)
            test_args+=("${arg%%=*}=$(wslpath -w "${arg#*=}")") ;;
        *) test_args+=("$arg") ;;
    esac
done

exec powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass \
    -File "$(wslpath -w "$script_dir/windows-test-exec.ps1")" \
    "$executable" "$workdir" "${test_args[@]}"
