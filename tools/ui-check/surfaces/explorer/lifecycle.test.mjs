import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { test } from 'node:test';

import { watchSurfaceOwner } from './lifecycle.mjs';

async function explorerServerClosesWhenItsOwnerExits() {
  const owner = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' });
  await once(owner, 'spawn');
  let closeCount = 0;
  let stop;
  let deadline;
  try {
    const closed = new Promise((resolve) => {
      stop = watchSurfaceOwner(owner.pid, () => {
        closeCount++;
        resolve();
      });
    });
    assert.equal(closeCount, 0);
    const exited = once(owner, 'exit');
    owner.kill('SIGTERM');
    await exited;
    await Promise.race([
      closed,
      new Promise((resolve, reject) => {
        deadline = setTimeout(() => reject(new Error('Explorer owner closure must finish')), 4000);
      }),
    ]);
    assert.equal(closeCount, 1);
  } finally {
    stop?.();
    clearTimeout(deadline);
    if (owner.exitCode === null && owner.signalCode === null) owner.kill('SIGTERM');
  }
}

test(explorerServerClosesWhenItsOwnerExits, { timeout: 5000 });
