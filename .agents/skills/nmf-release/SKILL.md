---
name: nmf-release
description: Prepare, publish, resume, or verify releases of nekomimist/nmf, including Windows checkout workflows through WSL and optional Scoop updates. Use for NMF release requests or completing a partially finished release.
---

# NMF Release

Use the repository's [release guide](../../../docs/releasing.md) as the source
of truth for versions, checks, builds, packaging, and release contents. Read it
first. Resolve links relative to this skill; run repository commands from the
nmf checkout root. Keep build rules in `Makefile` and `scripts/release.py`.

## Scope and starting state

- Distinguish procedure/skill maintenance, preparation, publication,
  verification, and Scoop-only requests. Procedure or skill maintenance does
  not authorize version changes, release preparation, tags, pushes, or workflow
  dispatch. Preparation alone ends with validated local changes. A request to
  release or publish authorizes the necessary preparation commit and branch/tag
  pushes; do not ask for those again. Update Scoop only when included in the
  request. Automatic skill selection does not expand that scope.
- For release preparation or publication, use the requested version, requested
  SemVer increment, or an unambiguous prepared version. Ask only if the intended
  target remains unclear. Use the user's release date or current local date;
  leave an empty `Unreleased` section.
- Inspect worktree/index changes, remotes, publication branch, and commits to
  be included. Preserve unrelated work. Read the actual CI/release workflows
  for branch triggers and required jobs instead of guessing their behavior.
- For publication, check GitHub authentication, remote divergence, push
  dry-run, and existing local/remote tags. Include the bucket in these checks
  when requested. A sandbox network error is not evidence that credentials
  are invalid; verify using the permitted network path before diagnosing auth.
- For release work, record the target version, source identity, relevant
  Actions run IDs, and downloaded asset directories. Before committing, identify
  the prepared source by its base HEAD plus the uncommitted diff. After committing,
  record the full release commit SHA so a later turn can resume accurately.

## Execution environment

- Keep one source checkout for the entire release. On Windows, the active
  Windows checkout is authoritative. Read
  [Releasing from Windows through WSL](../../../docs/releasing-windows.md)
  and use its PowerShell adapter for Linux checks, builds, and packaging.
  Resolve the checkout with Git, convert its path with `wslpath`, and confirm
  the Windows and WSL commands see the same root and full HEAD. Do not switch
  to WSL's separate `~/src/nmf` checkout or synchronize independent branches
  as an implicit release step.
- Use `nix develop` in that checkout for the pinned Linux toolchain and license
  collection. `scripts/dev-windows.ps1` produces a native development build,
  not the required release packages. Do not run competing Git mutations or
  packaging processes against the shared checkout.
- Choose a working Git transport before publication. Windows `gh` authentication
  and Git SSH authentication are independent. WSL Git may handle SSH operations
  against the Windows checkout while Windows `gh` handles Actions and release
  downloads; keep the checkout fixed and run Git operations sequentially.
  Convert Windows download and bucket paths before passing them to Linux
  commands. Pass paths, tags, and SHAs as arguments rather than interpolated
  shell code. Do not change persistent credentials or remotes without a need.
- If a sandbox blocks WSL or credential access, use the permitted host execution
  path; do not diagnose installation/authentication failure from that result
  alone. Native Linux operation can use the release guide's shell commands
  directly from its chosen checkout.

## Execute the requested stages

1. Follow the release guide's metadata updates, checks, and local Windows
   packaging. Inspect both archives. Windows builds share packaging metadata,
   so build the architectures sequentially. Verify local ZIPs with the helper
   and compare packaging inputs with their pre-build contents to confirm
   `FyneApp.toml` was restored and the Flake lock did not change unexpectedly.
   Preserve intentional pre-existing changes. Reuse successful checks for
   identical inputs when appropriate; do not skip required release checks.
   Preparation-only work
   stops here with validated uncommitted changes and local artifacts, before
   publication steps 2-5.
2. Review and commit only the intended release changes. Publish the preparation
   commit through the repository's branch workflow. Wait for `ci.yml` on that
   exact full SHA to complete successfully before tagging. Consult the available
   git commit/PR skills when performing those operations.
3. Create the matching annotated tag explicitly on the checked full SHA, then
   push that specific tag. Wait for `release.yml` with both the expected tag
   and SHA. Run creation or a started build is not completion. Use a background
   `gh run watch --exit-status --compact` session and report meaningful progress
   while waiting; inspect failed jobs before deciding on a retry.
4. Confirm the GitHub Release is published and its prerelease flag matches the
   target version. Download its two ZIPs and checksum files into a fresh
   temporary directory. Run `scripts/release.py verify` as documented in the
   guide. This validates bytes and archive contents; separately establish their
   provenance from the expected GitHub Release and successful run. Never use
   locally built ZIP hashes for a published Scoop update.
5. If requested for a stable release, update `nekomimist/scoop-bucket`. Use the
   provided checkout, otherwise look for `~/src/scoop-bucket` on the operating
   host (the Windows home when operating from Windows); read its local
   instructions and README. Do not silently substitute a WSL-home checkout.
   Fetch and inspect its current branch before editing. Generate a candidate
   with `verify --scoop-manifest`, review the diff, and apply only the intended
   version, URLs, and hashes. Commit and push the manifest, then wait for the
   bucket CI on that commit. If the user instead requests the bucket's updater
   workflow, use that route and verify its result.

## Resume and failure handling

Inspect remote state before repeating a mutation. Use the requested stage,
not the current checkout's `VERSION`, to identify a previously published release.

| Observed state | Next action |
| --- | --- |
| Preparation commit already exists | Verify its metadata, inclusion in the publication branch, and CI; continue from the first unfinished stage. |
| Local tag exists but remote tag does not | Verify the annotation and peeled commit SHA before pushing it. |
| Remote tag already exists | Compare its peeled SHA with the intended release commit. Reuse a matching tag; stop on a mismatch. Never move or delete it. |
| Matching release run is pending or running | Watch that run; do not start a duplicate publication. |
| Matching run failed | Inspect the failure. Retry a transient failure on the same SHA only when publication state is understood; do not loop blindly. Before tagging, fixes can use a new commit and CI run for the same version. After a remote tag exists, code changes need a new version; never retag. |
| Release is already published | Verify its tag, assets, and checksums; proceed only with unfinished requested work. Never overwrite published assets. |
| Scoop already has the target version | Compare both URLs and hashes. If correct, avoid a duplicate commit and verify the existing publication/CI. If newer, do not downgrade it. |

When a prerequisite fails, pause dependent publication steps while investigating.
Do not expand a release request into unrelated fixes. Report the exact failed
stage and completed remote mutations if progress requires a user decision.

## Completion

Report the release URL and tag/commit, relevant CI results, artifact verification,
and the bucket commit/CI when requested. State any unfinished stage or remaining
local changes. For preparation-only requests, report the prepared diff and
checks instead of publishing. For procedure/skill maintenance, report the
documentation changes and their validation without starting a release.
