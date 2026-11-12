import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync, existsSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import { collectArtifacts, overlayMain, runSession } from './runner.mjs';

test('overlay refuses ambiguous or changed startup code', () => {
  assert.throws(() => overlayMain('package main'), /exactly one/);
  assert.throws(() => overlayMain('\tfyneApp.Run()\n\tfyneApp.Run()'), /exactly one/);
  assert.match(overlayMain('\tfyneApp.Run()'), /startGUICapture\(fm\)/);
});

function fixture(failAt) {
  const calls = [], journal = {}, stages = [];
  const sdk = {};
  for (const name of ['provisionContainer', 'startContainer', 'runInContainer', 'stopContainer', 'deprovisionContainer']) {
    sdk[name] = async () => {
      calls.push(name);
      if (name === failAt) throw new Error('injected ' + name);
      if (name === 'provisionContainer') return { containerId: 'test-id', metadata: { agentUserName: 'test-user', ephemeralWorkspacePath: 'C:\\Users\\test-user\\Shared' } };
      if (name === 'runInContainer') return { exitCode: 0, timedOut: false };
      return { warnings: [] };
    };
  }
  return { calls, journal, stages, sdk, record: (name, value) => stages.push({ name, value }) };
}

for (const failure of ['provisionContainer', 'stage', 'startContainer', 'runInContainer', 'collect', 'stopContainer', 'deprovisionContainer']) {
  test('fails and cleans up after ' + failure, async () => {
    const f = fixture(failure);
    const passed = await runSession(f.sdk, f.journal, f.record,
      () => { if (failure === 'stage') throw new Error('fixture failed'); },
      () => { if (failure === 'collect') throw new Error('copy failed'); return { passed: true }; }, 1);
    assert.equal(passed, false);
    if (failure === 'provisionContainer') assert.deepEqual(f.calls, ['provisionContainer']);
    else assert.equal(f.calls.at(-1), 'deprovisionContainer');
    if (['startContainer', 'runInContainer', 'collect', 'stopContainer', 'deprovisionContainer'].includes(failure)) assert.ok(f.calls.includes('stopContainer'));
    assert.equal(f.journal.deprovisioned === true, !['provisionContainer', 'deprovisionContainer'].includes(failure));
  });
}

for (const result of [{ exitCode: 1, timedOut: false }, { exitCode: 0, timedOut: true }]) {
  test('propagates SDK workload failure ' + JSON.stringify(result), async () => {
    const f = fixture(); f.sdk.runInContainer = async () => result;
    assert.equal(await runSession(f.sdk, f.journal, f.record, () => {}, () => ({}), 1), false);
    assert.equal(f.journal.deprovisioned, true);
  });
}

test('cancellation never launches the GUI and still deprovisions', async () => {
  const f = fixture();
  assert.equal(await runSession(f.sdk, f.journal, f.record, () => {}, () => ({}), 1, () => true), false);
  assert.deepEqual(f.calls, ['provisionContainer', 'deprovisionContainer']);
});

test('successful lifecycle includes artifact collection and cleanup', async () => {
  const f = fixture(); let collected = false;
  assert.equal(await runSession(f.sdk, f.journal, f.record, () => {}, () => { collected = true; }, 1), true);
  assert.ok(collected); assert.equal(f.journal.deprovisioned, true);
});

test('artifact collection preserves failed evidence and rejects false success', () => {
  const temp = mkdtempSync(path.join(os.tmpdir(), 'nmf-gui-runner-'));
  try {
    const workspace = path.join(temp, 'workspace'), runDir = path.join(temp, 'run');
    mkdirSync(path.join(workspace, 'out'), { recursive: true }); mkdirSync(runDir);
    writeFileSync(path.join(workspace, 'out/result.json'), JSON.stringify({ passed: false, steps: [] }));
    writeFileSync(path.join(workspace, 'out/evidence.txt'), 'failed frame');
    assert.throws(() => collectArtifacts(workspace, runDir), /missing successful assertions/);
    assert.ok(existsSync(path.join(runDir, 'artifacts/evidence.txt')));
    writeFileSync(path.join(workspace, 'out/result.json'), JSON.stringify({ passed: true, steps: Array.from({ length: 6 }, () => ({ passed: true })) }));
    assert.deepEqual(collectArtifacts(workspace, runDir), { passed: true, steps: 6 });
  } finally {
    // mkdtemp owns this exact absolute directory; never remove a caller path.
    assert.equal(path.dirname(path.resolve(temp)), path.resolve(os.tmpdir()));
    rmSync(temp, { recursive: true, force: true });
  }
});
