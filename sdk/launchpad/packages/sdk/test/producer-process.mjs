import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';

const repository = fileURLToPath(new URL('../../../../../', import.meta.url));

export async function startProducer(module, packagePath, source, testName, env = {}) {
  const directory = mkdtempSync(join(tmpdir(), 'sdk-producer-'));
  const cwd = resolve(repository, 'services/launchpad', module);
  const overlay = join(directory, 'overlay.json');
  writeFileSync(overlay, JSON.stringify({ Replace: {
    [resolve(cwd, packagePath, 'sdk_producer_test.go')]: fileURLToPath(new URL(source, import.meta.url)),
  } }));
  const child = spawn('go', ['test', `-overlay=${overlay}`, '-count=1', '-v', `-run=^${testName}$`, `./${packagePath}`], {
    cwd, env: { ...process.env, ...env }, stdio: ['pipe', 'pipe', 'pipe'],
  });
  let output = '';
  let readyResolve;
  let readyReject;
  const ready = new Promise((resolve, reject) => { readyResolve = resolve; readyReject = reject; });
  const lines = createInterface({ input: child.stdout });
  lines.on('line', line => {
    output += line + '\n';
    if (!line.startsWith('SDK_SERVICE ')) { process.stdout.write(line + '\n'); return; }
    const event = JSON.parse(line.slice('SDK_SERVICE '.length));
    if (event.kind === 'ready') readyResolve(event.value);

  });
  child.stderr.on('data', chunk => { output += chunk; });
  child.on('error', readyReject);
  const exited = new Promise(resolve => child.on('exit', (code, signal) => {
    const error = new Error(`Producer exited ${code ?? signal}: ${output}`);
    readyReject(error);
    resolve({ code, signal });
  }));
  const timer = setTimeout(() => { readyReject(new Error(`Producer startup timed out: ${output}`)); child.kill(); }, 120000);
  let info;
  try { info = await ready; }
  catch (error) { child.kill(); await exited; rmSync(directory, { recursive: true, force: true }); throw error; }
  finally { clearTimeout(timer); }
  return {
    ...info,
    async sign(wallet, message) {
      const response = await fetch(info.control + '/sign', { method: 'POST', body: JSON.stringify({ wallet, message }) });
      assert.equal(response.status, 200);
      return response.json();
    },
    async close() {
      const stopped = await fetch(info.control + '/stop', { method: 'POST' });
      assert.equal(stopped.status, 204);
      const timer = setTimeout(() => child.kill(), 10000);
      try {
        const { code, signal } = await exited;
        assert.equal(signal, null, output);
        assert.equal(code, 0, output);
      } finally { clearTimeout(timer); lines.close(); rmSync(directory, { recursive: true, force: true }); }
    },
  };
}

export async function readProducerCases(module, packagePath, source, testName) {
  const directory = mkdtempSync(join(tmpdir(), 'sdk-answers-'));
  const cwd = resolve(repository, 'services/launchpad', module);
  const overlay = join(directory, 'overlay.json');
  writeFileSync(overlay, JSON.stringify({ Replace: {
    [resolve(cwd, packagePath, 'sdk_answers_test.go')]: fileURLToPath(new URL(source, import.meta.url)),
  } }));
  try {
    const { execFile } = await import('node:child_process');
    const { promisify } = await import('node:util');
    const { stdout } = await promisify(execFile)('go', ['test', `-overlay=${overlay}`, '-count=1', '-v', `-run=^${testName}$`, `./${packagePath}`], { cwd, timeout: 120000, maxBuffer: 8 * 1024 * 1024 });
    const records = stdout.split('\n').filter(line => line.startsWith('SDK_SERVICE ')).map(line => JSON.parse(line.slice('SDK_SERVICE '.length)));
    assert.ok(records.length > 0);
    return records;
  } finally { rmSync(directory, { recursive: true, force: true }); }
}
