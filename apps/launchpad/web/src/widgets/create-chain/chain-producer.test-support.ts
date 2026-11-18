import { spawn } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { createInterface } from 'node:readline';
import type { Address } from 'viem';

export async function openChainProducer() {
  const repository = resolve(__dirname, '../../../../../..');
  const directory = mkdtempSync(resolve(tmpdir(), 'chain-launch-'));
  const overlay = resolve(directory, 'overlay.json');
  writeFileSync(overlay, JSON.stringify({ Replace: {
    [resolve(repository, 'precompiles/launchpad/web_chain_producer_test.go')]: resolve(__dirname, 'chain-producer_test.go.txt'),
  } }));
  const child = spawn('go', ['test', `-overlay=${overlay}`, '-p=2', '-count=1', '-v',
    '-run=^TestChainLaunchWebProducer$', './precompiles/launchpad'], {
    cwd: repository, stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '';
  const stopped = new Promise<number | null>((resolve, reject) => {
    child.once('error', reject);
    child.once('exit', resolve);
  });
  child.stderr.on('data', chunk => { output += String(chunk); });
  const lines = createInterface({ input: child.stdout });
  try {
    const info = await new Promise<{ origin: string; creator: Address; token: Address }>((resolve, reject) => {
      lines.on('line', line => {
        output += `${line}\n`;
        if (line.startsWith('CHAIN_PRODUCER ')) resolve(JSON.parse(line.slice('CHAIN_PRODUCER '.length)));
      });
      void stopped.then(code => reject(new Error(`Chain producer exited ${code}:\n${output}`)), reject);
    });
    return { ...info, async close() {
      try {
        const response = await fetch(`${info.origin}/stop`, { method: 'POST' });
        if (response.status !== 204 || await stopped !== 0) throw new Error(output);
      } finally {
        child.kill('SIGTERM');
        lines.close();
        rmSync(directory, { recursive: true });
      }
    } };
  } catch (error) {
    child.kill('SIGTERM');
    lines.close();
    rmSync(directory, { recursive: true });
    throw error;
  }
}
