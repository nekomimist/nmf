import { copyFileSync, cpSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '../..');
const kind = 'nmf-gui-windows';
const json = file => JSON.parse(readFileSync(file, 'utf8'));
const save = (file, value) => writeFileSync(file, JSON.stringify(value, null, 2) + '\n');
const errorInfo = error => ({ message: String(error.message ?? error), code: error.code, operation: error.operation });

export function overlayMain(source) {
  const anchor = '\tfyneApp.Run()';
  if (source.split(anchor).length !== 2) throw new Error('Expected exactly one fyneApp.Run() anchor; review the GUI overlay after startup changes.');
  return source.replace(anchor, '\tstartGUICapture(fm)\n' + anchor);
}

export function prepare(runDir) {
  mkdirSync(runDir); // A fresh run never reuses stale captures or binaries.
  const virtualSource = path.join(repo, 'zz_gui_capture_windows.go');
  if (existsSync(virtualSource)) throw new Error('GUI overlay virtual filename already exists');
  const main = path.join(repo, 'main.go');
  writeFileSync(path.join(runDir, 'main.go'), overlayMain(readFileSync(main, 'utf8')));
  copyFileSync(path.join(here, 'capture.go.txt'), path.join(runDir, 'capture_windows.go'));
  save(path.join(runDir, 'overlay.json'), { Replace: {
    [main]: path.join(runDir, 'main.go'),
    [virtualSource]: path.join(runDir, 'capture_windows.go'),
  } });
}

async function loadSDK(sdkDir) {
  if (process.platform !== 'win32' || process.arch !== 'x64') throw new Error('This GUI runner currently requires Windows x64 and x64 Node.js.');
  const [major, minor] = process.versions.node.split('.').map(Number);
  if (!((major === 24 && minor >= 21) || (major === 26 && minor >= 8) || major > 26)) {
    throw new Error('MXC stdio requires Node.js 24.21+ or 26.8+ (supported release lines).');
  }
  const require = createRequire(path.join(sdkDir, 'package.json'));
  return import(pathToFileURL(require.resolve('@microsoft/mxc-sdk/v1')).href);
}

function stageFixture(workspace, runDir) {
  copyFileSync(path.join(runDir, 'nmf-gui.exe'), path.join(workspace, 'nmf-gui.exe'));
  copyFileSync(path.join(runDir, 'GuiSmoke.exe'), path.join(workspace, 'GuiSmoke.exe'));
  mkdirSync(path.join(workspace, 'profile'));
  mkdirSync(path.join(workspace, 'files/folder-a'), { recursive: true });
  writeFileSync(path.join(workspace, 'files/alpha.txt'), 'Isolated GUI fixture\n');
  writeFileSync(path.join(workspace, 'files/日本語.txt'), 'Japanese filename fixture\n');
  writeFileSync(path.join(workspace, 'files/folder-a/nested.txt'), 'Nested fixture\n');
  save(path.join(workspace, 'profile/config.json'), {
    window: { width: 900, height: 650 }, theme: { dark: true, fontSize: 14 },
  });
}

// Dependencies are passed explicitly so failure/cleanup paths can be checked
// without creating Windows accounts or injecting any input.
export async function runSession(sdk, journal, record, stage, collect, hostSession, cancelled = () => false) {
  let failed = false;
  try {
    const provision = await sdk.provisionContainer({
      containment: 'isolation_session',
      // Required by this SDK backend. The fixture performs no network I/O.
      network: { egress: { default: 'allow' }, ingress: { default: 'allow', hostLoopback: 'allow' } },
    });
    journal.containerId = provision.containerId;
    journal.metadata = provision.metadata;
    record('provision', provision);
    const { ephemeralWorkspacePath: workspace, agentUserName } = provision.metadata;
    if (!path.win32.isAbsolute(workspace) || !/^[A-Za-z0-9_-]+$/.test(agentUserName)) throw new Error('Unexpected IsolationSession metadata');
    stage(workspace);
    if (cancelled()) throw new Error('Cancelled before session start');
    journal.startAttempted = true;
    record('starting', {});
    record('start', await sdk.startContainer(journal.containerId));
    if (cancelled()) throw new Error('Cancelled before GUI launch');
    const result = await sdk.runInContainer(journal.containerId, {
      // Fixed filenames under the SDK workspace avoid cmd.exe path quoting.
      command: `GuiSmoke.exe ${hostSession} ${agentUserName}`,
      workingDirectory: workspace, timeoutMs: 120000,
    });
    record('gui', result);
    if (result.exitCode !== 0 || result.timedOut || cancelled()) throw new Error('GUI execution failed, timed out, or was cancelled');
  } catch (error) {
    failed = true; record('error', errorInfo(error));
  } finally {
    // Preserve evidence before deleting the SDK-owned workspace, including on
    // launch/assertion failure. Copy failure must not prevent account cleanup.
    if (journal.metadata) {
      try { record('artifacts', collect(journal.metadata.ephemeralWorkspacePath)); }
      catch (error) { failed = true; record('artifacts-error', errorInfo(error)); }
    }
    if (journal.containerId) {
      if (journal.startAttempted) {
        try { record('stop', await sdk.stopContainer(journal.containerId)); }
        catch (error) { failed = true; record('stop-error', errorInfo(error)); }
      }
      try {
        const result = await sdk.deprovisionContainer(journal.containerId);
        journal.deprovisioned = true; record('deprovision', result);
      } catch (error) { failed = true; record('deprovision-error', errorInfo(error)); }
    }
  }
  return !failed;
}

export function collectArtifacts(workspace, runDir) {
  const output = path.join(workspace, 'out');
  if (existsSync(output)) cpSync(output, path.join(runDir, 'artifacts'), { recursive: true });
  const report = json(path.join(runDir, 'artifacts/result.json'));
  if (report.passed !== true || !Array.isArray(report.steps) || report.steps.length !== 6 || report.steps.some(step => step.passed !== true)) {
    throw new Error('GUI result is missing successful assertions; inspect artifacts/result.json');
  }
  return { passed: true, steps: report.steps.length };
}

async function main() {
  const [task, runArg, sdkArg, sessionArg] = process.argv.slice(2);
  if (!runArg) throw new Error('Expected task and run directory');
  const runDir = path.resolve(runArg);
  if (task === 'prepare') { prepare(runDir); return; }
  const sdk = await loadSDK(path.resolve(sdkArg));
  if (task === 'check') {
    const support = sdk.getPlatformSupport();
    console.log(JSON.stringify(support, null, 2));
    if (!support.availableMethods?.includes('isolation_session')) throw new Error('IsolationSession is unavailable on this Windows host');
    return;
  }
  const journalPath = path.join(runDir, 'lifecycle.json');
  if (task === 'cleanup') {
    const journal = json(journalPath);
    if (journal.kind !== kind || !journal.containerId) throw new Error('Not an NMF GUI session journal');
    if (journal.deprovisioned) { console.log('Session was already deprovisioned.'); return; }
    if (journal.startAttempted) {
      try { await sdk.stopContainer(journal.containerId); }
      catch (error) { console.error('Stop: ' + error.message); }
    }
    await sdk.deprovisionContainer(journal.containerId);
    journal.deprovisioned = true; journal.recoveredAt = new Date().toISOString(); save(journalPath, journal);
    console.log('Session deprovisioned.'); return;
  }
  if (task !== 'run') throw new Error('Unknown task: ' + task);
  const hostSession = Number(sessionArg);
  if (!Number.isInteger(hostSession) || hostSession <= 0) throw new Error('Expected the host interactive session ID');
  if (existsSync(journalPath)) throw new Error('Run directory already has a session journal; create a fresh run or use cleanup');
  const journal = { kind, startedAt: new Date().toISOString(), node: process.version, hostSession, stages: [] };
  const record = (stage, value) => {
    journal.stages.push({ stage, time: new Date().toISOString(), value });
    // A log write error must not abort the finally block's resource cleanup.
    try { save(journalPath, journal); }
    catch (error) { journal.logError = String(error); console.error('Journal write failed: ' + error.message); }
    if (stage === 'error' || stage.endsWith('-error')) console.error(stage + ': ' + value.message);
    else console.log(stage + (stage === 'gui' ? '\n' + value.stdout + value.stderr : ''));
  };
  let cancelled = false;
  const cancel = () => { cancelled = true; console.error('Cancellation requested; waiting for bounded execution and cleanup.'); };
  process.on('SIGINT', cancel); process.on('SIGTERM', cancel);
  try {
    const passed = await runSession(sdk, journal, record, workspace => stageFixture(workspace, runDir),
      workspace => collectArtifacts(workspace, runDir), hostSession, () => cancelled);
    journal.passed = passed && !journal.logError;
    record('finished', { passed: journal.passed });
    journal.passed = journal.passed && !journal.logError;
    if (!journal.passed) process.exitCode = 1;
    console.log(`${journal.passed ? 'PASS' : 'FAIL'}: ${runDir}`);
    if (journal.containerId && !journal.deprovisioned) console.error(`Cleanup is incomplete. Run test-gui-windows.ps1 -CleanupRun "${runDir}"`);
  } finally { process.off('SIGINT', cancel); process.off('SIGTERM', cancel); }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch(error => { console.error(error); process.exitCode = 1; });
}
