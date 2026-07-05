---
name: verify
description: Build, launch, and drive the nmf GUI end-to-end on WSLg (Linux) and Windows via WSL interop — keyboard injection + screenshots.
---

# Verifying nmf changes end-to-end

nmf is a keyboard-driven Fyne GUI app. This skill bundles working drivers for
both surfaces; `scripts/` lives next to this file.

## Linux (WSLg, DISPLAY=:0)

XTEST/xdotool input is silently dropped on WSLg (the Wayland compositor owns
focus; Xvfb is not installed, no sudo). What works: synthetic
KeyPress/KeyRelease via XSendEvent straight to the window — GLFW processes
send_event events. `scripts/inject_keys.py` does exactly that; run it with
`uv run` (uv is always installed here; the script declares its `python-xlib`
dependency inline, PEP 723 — never pip-install into the system Python).

```bash
SKILL=.claude/skills/verify            # repo-relative
go build -o "$SCRATCH/nmf-bench" .     # normal build; add -race for concurrency checks

# Big test directory (20k files + 50 subdirs)
mkdir -p /tmp/nmf-bench/big && cd /tmp/nmf-bench/big
for i in $(seq 1 20000); do : > "file_$(printf %05d $i).txt"; done
mkdir -p subdir_{01..50}

# Launch with debug logs (use the harness's run_in_background, not shell '&')
"$SCRATCH/nmf-bench" -d -debug-log "$SCRATCH/run.log" -path /tmp/nmf-bench/big

# Drive (window title is "File Manager", not "nmf")
uv run "$SKILL/scripts/inject_keys.py" "File Manager" Down 200 5     # hold-down simulation
uv run "$SKILL/scripts/inject_keys.py" "File Manager" S-Period 1 50  # cursor to last entry
uv run "$SKILL/scripts/inject_keys.py" "File Manager" Return 1 50

# Observe
import -window "File Manager" shot.png          # ImageMagick, works on WSLg
grep -E "LoadDirectory (start|done)" "$SCRATCH/run.log"
grep -c "KeyManager: KeyDown recorded key=Down" "$SCRATCH/run.log"
```

## Windows build (WSL interop)

WSL2 runs the Windows build directly on the user's desktop — windows pop up
on their screen, so keep sessions short and kill the process when done.

```bash
make build-windows && cp dist/nmf.exe /mnt/c/Temp/nmf-verify.exe
cp .claude/skills/verify/scripts/drive.ps1 /mnt/c/Temp/drive.ps1
cmd.exe /c "start /D C:\Temp C:\Temp\nmf-verify.exe -d -debug-log C:\Temp\run.log -path C:\Temp\testdir"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\\Temp\\drive.ps1" -Proc nmf-verify -Keys "+." -Shot "C:\\Temp\\out.png"
powershell.exe -NoProfile -Command "Stop-Process -Name nmf-verify -Force"
```

- FindWindow-by-title fails from interop; drive.ps1 resolves the window via
  `(Get-Process X).MainWindowHandle` and activates with WScript.Shell.
- SendKeys syntax: `{DOWN}` `{ENTER}` `{BACKSPACE}`, `+.` = Shift+Period, `q`.
- Read logs/screenshots from WSL via `/mnt/c/Temp/...`.

## Flows worth driving

- Enter a big dir → `LoadDirectory start/done` timestamps; no
  "spinner gone but list blank" gap.
- Hold-down simulation: 200x Down @5ms; the settled screenshot shows the
  cursor at exactly the expected index (rows: 0="..", then dirs, then files).
- Enter a subdir (Return) and Return on ".." → cursor restores to that subdir.
- space x3 → status bar "Mark: 3", selection highlight, cursor advances.
- Watcher: `touch`/`rm` files in the open dir → expect
  `DirectoryWatcher: Applying changes: N added, ...` within ~2s; modify-only
  churn under name sort must NOT log `FileManager: Sorting files`
  (re-sort skip), adds/deletes must.
- Race check: run a `-race` build, churn files while injecting Down keys
  concurrently; `grep -c "WARNING: DATA RACE" run.log` must be 0.
- **Deep-cursor restore** (driver fact 8 in docs/architecture/ui-input.md,
  regression seen on Windows): needs a saved cursor beyond the first viewport
  in a large dir — S-Period (cursor to bottom), BackSpace to parent (saves
  the position), q to quit, relaunch into that dir and screenshot **without
  any keypress**: the list must show the scrolled-to region, not stay blank.

## Gotchas

- The debug log's `KeyManager: KeyDown recorded` count is exact evidence of
  delivered keys; log timestamps are second-granularity only.
- Screenshots right after a key burst catch the UI mid-catch-up; sleep ~2s
  for the settled frame.
- `pkill -f <binary>` between runs; a stale instance makes window lookup
  ambiguous. On Windows: `Stop-Process -Name <proc> -Force`.
- Cursor positions persist per directory in the config — saved on
  navigation away, not on quit. Great for building repros; clean up test
  dirs when done so stale entries don't confuse later sessions.
- On Windows at 150% scale, ScrollTo's first jump can land the cursor row
  just below the viewport (Fyne itemMin estimate; pre-existing) — one cursor
  key corrects it; don't mistake it for the blank-list bug.
