import { expect, test } from 'vitest';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { closeSync, mkdtempSync, openSync, readFileSync, rmSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { setTimeout } from 'node:timers/promises';
import { pathToFileURL } from 'node:url';
import { observeOraclePublication } from './oracle-publication.mjs';

async function oraclePublicationsRecoverAfterCanonicalReorganization() {
  const file = process.env.L27_PHASE2_FIXTURE;
  if (!file) throw new Error('Owned creation authority is required');
  const core = JSON.parse(readFileSync(file, 'utf8')).phase2;
  const rpc = new URL(core.rpcUrl);
  if (!core.ownedFixture || rpc.hostname !== '127.0.0.1') {
    throw new Error('An isolated loopback chain is required');
  }
  const ethers = await import(pathToFileURL(resolve(
    process.cwd(),
    '../../../contracts/launchpad/node_modules/ethers/lib.esm/index.js'
  )).href);
  const reservation = createServer();
  reservation.listen(0, rpc.hostname);
  await once(reservation, 'listening');
  const address = reservation.address();
  if (!address || typeof address === 'string') throw new Error('TCP port is required');
  rpc.port = String(address.port);
  await new Promise<void>((resolve, reject) => {
    reservation.close(error => error ? reject(error) : resolve());
  });
  const output = mkdtempSync(join(tmpdir(), 'oracle-publication-'));
  const log = openSync(join(output, 'chain.log'), 'w', 0o600);
  const node = spawn(process.env.ANVIL_BIN ?? 'anvil', [
    '--host', rpc.hostname, '--port', rpc.port, '--chain-id', '125', '--silent',
  ], { stdio: ['ignore', log, log] });
  let launchError: Error | undefined;
  node.on('error', error => { launchError = error; });
  const provider = new ethers.JsonRpcProvider(rpc.href, 125, { cacheTimeout: -1 });
  try {
    const deadline = Date.now() + 5000;
    for (;;) {
      if (launchError) throw launchError;
      if (node.exitCode !== null) throw new Error('Owned chain exited before readiness');
      try {
        const response = await fetch(rpc, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'eth_chainId', params: [] }),
          signal: AbortSignal.timeout(500),
        });
        const result = await response.json();
        if (result.result !== '0x7d') throw new Error('Owned chain identity differs');
        break;
      } catch (error) {
        if (Date.now() >= deadline) throw error;
        await setTimeout(50);
      }
    }
    const signer = await provider.getSigner(0);
    const recipient = await (await provider.getSigner(1)).getAddress();
    const balance = await provider.getBalance(recipient);
    const snapshot = await provider.send('evm_snapshot', []);
    const removed = await signer.sendTransaction({ to: recipient, value: 1n, gasLimit: 21000n });
    expect((await provider.send('eth_getTransactionReceipt', [removed.hash])).status).toBe('0x1');
    expect(await provider.send('evm_revert', [snapshot])).toBe(true);
    expect(await observeOraclePublication(provider, removed)).toEqual({
      state: 'reorganized', transactionHash: removed.hash,
    });
    expect(await provider.send('eth_getTransactionByHash', [removed.hash])).toBeNull();
    const replacement = await signer.sendTransaction({ to: recipient, value: 2n, gasLimit: 21000n });
    const confirmed = await observeOraclePublication(provider, replacement);
    expect(confirmed.state).toBe('confirmed');
    if (confirmed.state !== 'confirmed') {
      throw new Error('Canonical publication receipt is required');
    }
    expect(confirmed.receipt.status).toBe('0x1');
    expect(confirmed.receipt.transactionHash).toBe(replacement.hash);
    const canonical = await provider.send('eth_getBlockByNumber', [
      confirmed.receipt.blockNumber, false,
    ]);
    expect(canonical.hash).toBe(confirmed.receipt.blockHash);
    expect(await provider.getBalance(recipient)).toBe(balance + 2n);
  } finally {
    provider.destroy();
    if (node.pid !== undefined && node.exitCode === null) {
      node.kill('SIGTERM');
      await once(node, 'exit');
    }
    closeSync(log);
    rmSync(output, { recursive: true, force: true });
  }
}

test(
  oraclePublicationsRecoverAfterCanonicalReorganization.name,
  oraclePublicationsRecoverAfterCanonicalReorganization,
  10000
);
