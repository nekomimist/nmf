# Versioning and releases

## Version policy

NMF uses [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html), starting
at **0.1.0**. The root `VERSION` file is the source of truth, without a `v` prefix.
It is embedded in every Go build, including `go run`, and supplies the Windows
package version. Git tags use `v` followed by that exact version, such as `v0.1.0`.

The compatibility surface consists of documented command-line options,
`config.json` settings, Starlark configuration and command APIs, persisted state,
and user-facing keyboard and file-operation behavior. Internal Go packages are
implementation details.

- While the major version is zero, bump the minor version for new features or
  breaking changes, and the patch version for compatible fixes and improvements.
  Compatibility is still evolving; always document breaking changes explicitly.
- From 1.0.0, bump the major version for incompatible changes, the minor version
  for compatible features, and the patch version for compatible fixes.
- Prereleases may use suffixes such as `0.2.0-rc.1`. GitHub marks these releases
  as prereleases. SemVer build metadata is also accepted.
- Published versions and tags are immutable. Corrections require a new version.

The version dialog and debug log show the full `VERSION` value. Windows PE
version resources use its numeric `MAJOR.MINOR.PATCH` core, since the resource
format cannot represent prerelease or build suffixes. The separate Fyne
`Details.Build` value is a packaging build number, not the application version;
the Makefile passes it explicitly and restores `FyneApp.toml` after packaging to
undo Fyne's automatic increment, keeping both architectures on the same number.
Go's VCS build metadata remains available through `go version -m`.

## Changelog

Record user-visible improvements, features, fixes, behavior changes, and breaking
changes in the root [CHANGELOG.md](../CHANGELOG.md) as part of the change that
introduces them. Internal refactors, tests, and documentation housekeeping do
not need entries unless they affect users.

Follow the compact style used by `nekomimist/neft`: an `## Unreleased` section at
the top, then `## VERSION - YYYY-MM-DD` releases in reverse chronological order,
with plain English bullet points. Describe the effect on users rather than a
commit list. Mark incompatible changes with **Breaking:** and explain any action
users must take. The 0.1.0 entry establishes the baseline; earlier development
history remains in Git.

## Preparing a release

1. Update `VERSION` and move the `Unreleased` entries into a new dated release
   section, leaving an empty `Unreleased` heading at the top. Use the release date.
2. Validate the release metadata and run the checks:

   ```sh
   python3 scripts/release.py check v0.1.0
   python3 -m unittest discover -s scripts -p '*_test.py'
   make test-all
   go vet -tags migrated_fynedo ./...
   git diff --check
   ```

3. Build both Windows architectures and inspect their ZIPs locally:

   ```sh
   make build-windows
   nix develop --command python3 scripts/release.py package amd64
   make build-windows-arm64
   nix develop --command python3 scripts/release.py package arm64
   ```

   Release tooling requires Python 3.11 or newer; the Flake includes a pinned
   Python alongside Go, Zig, and Fyne. Packaging uses that environment to collect
   the licenses for each architecture's dependencies.
4. Commit and merge the release preparation. After CI passes on that commit,
   create and push the matching annotated tag (replace the example version for
   later releases):

   ```sh
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

Pushing a `v*` tag starts [.github/workflows/release.yml](../.github/workflows/release.yml).
It rejects invalid versions, mismatched tags, missing or empty release notes,
and entries still left in `Unreleased`. It runs the tests and vet checks, then
builds both Windows targets with the existing pinned Nix/Go/Zig/Fyne toolchain.
After both ZIPs and their checksums are ready, it creates the GitHub Release
using the matching CHANGELOG section as its release notes. No manual release
creation or additional signing secret is required; publishing uses the workflow's
`GITHUB_TOKEN` with `contents: write` permission.

## Release contents and licenses

Each release publishes these assets (with its own version in the filenames):

- `nmf-v0.1.0-windows-amd64.zip` for Windows x64.
- `nmf-v0.1.0-windows-arm64.zip` for Windows ARM64.
- A `.zip.sha256` checksum file beside each ZIP.

Both ZIPs contain `nmf.exe`, `README.md`, `CHANGELOG.md`, `LICENSE`,
`THIRD_PARTY_LICENSES.txt`, and a `licenses/` directory. Extract the entire ZIP
into a directory and run `nmf.exe`; keep the license documents with redistributed
copies. The ZIP filename identifies the architecture, and packaging verifies
the executable's PE machine type before writing the archive.

`scripts/release.py` collects license, notice, copyright, authors, and patent
files from the Go modules selected by the target's dependency graph, preserving
module names, versions, and relative paths. This includes nested notices such
as Fyne's font licenses and GLFW's bundled C code. It also includes the Go runtime
and MinGW-w64 license documents. The Flake extracts Go's notices from the pinned
Go source archive because nixpkgs omits them from the installed `GOROOT`.
`THIRD_PARTY_LICENSES.txt` holds supplementary
notices, including those for the embedded JPEG XL implementation.

When updating dependencies, review their bundled assets and native code for
additional notices and update the supplementary file if needed. The collector
fails if a module has no recognized license file; the pinned `koron/gelatin`
module is an explicit exception because it declares its MIT license in
`README.mkd`, which is included instead. Missing or empty required documents
also fail packaging.
