import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import test from 'node:test';

async function rankingServiceContract() {
  const cases = [
    'TestSnapshotSDKSuccessAndContinuation',
    'TestSnapshotSDKExpiry',
    'TestSnapshotSDKUnavailableAndInvalidRequests',
    'TestSnapshotSDKEmptyAndStale',
  ];
  const cwd = fileURLToPath(new URL('../../../../../services/launchpad/ranking-algo/', import.meta.url));
  let stdout;
  try {
    ({ stdout } = await promisify(execFile)('go', [
      'test', '-v', '-count=1', '-timeout=3m', './internal/httpapi',
      '-run', `^(${cases.join('|')})$`,
    ], { cwd, timeout: 240_000, maxBuffer: 1024 * 1024 }));
  } catch (error) {
    throw new Error(`Ranking service contract: ${error.message}\n${error.stdout ?? ''}${error.stderr ?? ''}`, { cause: error });
  }
  for (const name of cases) assert.match(stdout, new RegExp(`--- PASS: ${name} \\(`));
}

test(rankingServiceContract);
