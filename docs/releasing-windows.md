# Releasing from Windows through WSL

Use the Windows checkout as the source for the entire release. WSL supplies the
Linux tools used by the existing [release procedure](releasing.md); it does not
supply another working copy. Release versions, required checks, packaging rules,
and publication stages remain defined in that procedure.

Updating these instructions or investigating the environment does not authorize
a release. Preparation-only work stops with validated uncommitted changes and
local artifacts. Do not push a release preparation commit or create or push a
release tag unless publication is requested.

## Connect PowerShell to the same checkout

Use PowerShell 7 from the Windows checkout. Select an installed WSL distribution
with Nix and the release toolchain available; `Ubuntu` is an example. Resolve the
checkout from Git and translate its path instead of hardcoding a Linux home path:

```powershell
$nmfDistro = 'Ubuntu'
$nmfCheckout = git rev-parse --show-toplevel
if ($LASTEXITCODE -ne 0) { throw 'Run this from the intended Windows checkout.' }
$nmfWslCheckout = wsl.exe -d $nmfDistro --exec wslpath -a -u $nmfCheckout
if ($LASTEXITCODE -ne 0) { throw 'Could not translate the checkout path through WSL.' }
$nmfWslCheckout = $nmfWslCheckout.Trim()

function Invoke-NmfWsl {
    param(
        [Parameter(Mandatory)][string]$Script,
        [string[]]$ScriptArguments = @()
    )
    $nmfBash = @'
set -eu
cd -- "$1"
shift
'@ + "`n" + $Script
    wsl.exe -d $nmfDistro --exec bash -lc $nmfBash nmf-release $nmfWslCheckout @ScriptArguments
    if ($LASTEXITCODE -ne 0) { throw "WSL command failed with exit code $LASTEXITCODE." }
}

$nmfWindowsHead = git -C $nmfCheckout rev-parse HEAD
if ($LASTEXITCODE -ne 0) { throw 'Could not read the Windows checkout HEAD.' }
$nmfLinuxHead = Invoke-NmfWsl 'git rev-parse HEAD'
if ($nmfWindowsHead.Trim() -ne $nmfLinuxHead.Trim()) {
    throw 'Windows and WSL must use the same checkout and HEAD.'
}
Invoke-NmfWsl 'git rev-parse --show-toplevel; git status --short; nix --version'
```

The function changes directory after the login shell has initialized, passes
paths and other arguments separately from shell code, and propagates failures to
PowerShell. Use single-quoted PowerShell strings or here-strings for Bash code so
PowerShell does not expand Bash variables. Pass changing values with
`-ScriptArguments`; do not interpolate paths, tags, or refspecs into Bash code.

Inspect the worktree, index, branch, and remotes before proceeding. Keep metadata
edits and Git operations in this checkout even when a separate `~/src/nmf` exists
inside WSL. Do not run Git mutations from both environments concurrently.

When an agent sandbox reports `Wsl/E_ACCESSDENIED`, use the permitted host
execution path for `wsl.exe`. This is separate from WSL installation or GitHub
authentication; do not change host security settings to work around it.

## Check tools and authentication

Run Linux checks and packaging inside `nix develop` from this checkout. The Flake
pins Go, Zig, Fyne, Python, and the license documents required by packaging. The
native `scripts/dev-windows.ps1` remains for Windows development and tests; its
host-architecture executable is not a substitute for the two release packages.
Keep Linux Go/Zig caches separate from the native Windows caches.

```powershell
Invoke-NmfWsl @'
nix develop --command go version
nix develop --command zig version
nix develop --command python3 --version
'@
```

Check GitHub authentication when publication or remote verification is in scope.
Local preparation can proceed with the tool checks alone:

```powershell
gh auth status
if ($LASTEXITCODE -ne 0) { throw 'Check GitHub CLI authentication in this environment.' }
Invoke-NmfWsl 'git ls-remote --exit-code origin refs/heads/master'
```

Windows `gh` can inspect Actions and releases independently of the Git client's
authentication. A working `gh auth status` does not prove that Windows Git can
push an SSH remote. If Windows SSH fails but WSL SSH works, run Git network
operations with `Invoke-NmfWsl` against this same checkout. For publication,
check the selected transport with a push dry-run and inspect remote branches
and tags as required by the release skill. Do not rewrite global credentials or
remote URLs just to switch environments. A sandbox credential or network error
must be rechecked through the permitted host execution path before diagnosing
invalid credentials.

## Run preparation and packaging

Edit `VERSION` and `CHANGELOG.md` in the Windows checkout as described in the
release procedure. Choose the intended tag, then run the checks through WSL:

```powershell
$nmfTag = 'v0.2.0' # Example only; use the requested release version.
Invoke-NmfWsl -ScriptArguments @($nmfTag) -Script @'
nix develop --command python3 scripts/release.py check "$1"
nix develop --command python3 -m unittest discover -s scripts -p '*_test.py'
nix develop --command make test-all
nix develop --command go vet -tags migrated_fynedo ./...
git diff --check
'@
```

Build and package the architectures sequentially. They temporarily share
`FyneApp.toml` and the intermediate Fyne executable. Do not edit the checkout or
run another packaging process while this block is running:

```powershell
Invoke-NmfWsl -ScriptArguments @($nmfTag) -Script @'
nmf_packaging_inputs=$(sha256sum FyneApp.toml flake.lock)
make build-windows
nix develop --command python3 scripts/release.py package amd64
make build-windows-arm64
nix develop --command python3 scripts/release.py package arm64
nix develop --command python3 scripts/release.py verify "$1" dist
if [ "$nmf_packaging_inputs" != "$(sha256sum FyneApp.toml flake.lock)" ]; then
    echo "Packaging changed FyneApp.toml or flake.lock; inspect the diff." >&2
    exit 1
fi
'@
```

The binaries, ZIPs, and checksums appear in the Windows checkout's `dist/`.
Inspect both ZIPs and the intended release diff before committing. For a
preparation-only request, stop with the validated uncommitted changes and local
artifacts. Record the base HEAD plus the prepared diff until a release commit
exists. The packaging check compares pre-build contents so preserved earlier
changes are not mistaken for packaging failures; do not reset them.

Access through `/mnt/c` may be slower than a Linux-native filesystem. Keep this
single-checkout workflow unless measurements justify another approach. Any
future Linux build staging must be tied to the recorded source commit rather
than continuing a release in an independently advancing checkout.

## Publish and verify only when requested

Follow the publication and resume rules in the release skill: publish the
preparation commit through the branch workflow, wait for successful CI on its
exact full SHA, and tag that checked SHA. Use the same Git transport and
checkout for those operations. `Invoke-NmfWsl` accepts Git commands and separate
tag/SHA arguments just like the preparation blocks; it does not itself publish.
Windows `gh run watch` can wait for CI and release runs. The tag workflow still
builds and publishes on GitHub's Ubuntu runner.

For published-asset verification, Windows `gh` can download into a fresh Windows
directory. Translate that directory before passing it to the Linux verifier:

```powershell
# Use the tag being verified, which need not match the checkout's VERSION.
$nmfDownloadDir = Join-Path ([IO.Path]::GetTempPath()) ('nmf-release-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $nmfDownloadDir | Out-Null
gh release download $nmfTag --repo nekomimist/nmf --dir $nmfDownloadDir
if ($LASTEXITCODE -ne 0) { throw 'Could not download the published release assets.' }
$nmfWslDownloadDir = wsl.exe -d $nmfDistro --exec wslpath -a -u $nmfDownloadDir
if ($LASTEXITCODE -ne 0) { throw 'Could not translate the release asset directory.' }
Invoke-NmfWsl -ScriptArguments @($nmfTag, $nmfWslDownloadDir.Trim()) -Script @'
nix develop --command python3 scripts/release.py verify "$1" "$2"
'@
```

Use only hashes from these verified published downloads for an authorized Scoop
update. If editing the bucket locally, choose one explicit bucket checkout,
read its instructions, and translate its Windows path for WSL commands. Do not
silently switch to a different bucket checkout under the WSL home directory.
