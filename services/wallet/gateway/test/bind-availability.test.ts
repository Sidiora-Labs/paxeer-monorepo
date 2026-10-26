import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { encodeAbiParameters, toFunctionSelector, type Hex } from 'viem';
import { readKernelAvailability } from '../src/provision/bind.js';
import { RpcPool } from '../src/rpc/pool.js';

interface RpcRequest {
  jsonrpc: string;
  id: number;
  method: string;
  params: unknown[];
}

type RpcReply = { result: Hex } | { error: { code: number; message: string; data: Hex } };

async function startBindRpc(reply: RpcReply) {
  const calls: RpcRequest[] = [];
  const server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      const call = JSON.parse(Buffer.concat(chunks).toString('utf8')) as RpcRequest;
      calls.push(call);
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify({ jsonrpc: '2.0', id: call.id, ...reply }));
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const rpc = new RpcPool({
    urls: [`http://127.0.0.1:${(server.address() as AddressInfo).port}`],
    chainId: 125,
    lagThresholdBlocks: 20,
    timeoutMs: 3_000,
    healthIntervalMs: 60_000,
  });
  return {
    rpc,
    calls,
    close: () => new Promise<void>((resolve, reject) => {
      server.close((err) => err ? reject(err) : resolve());
    }),
  };
}

function expectBindNonceCall(calls: RpcRequest[]): void {
  expect(calls).toEqual([{
    jsonrpc: '2.0',
    id: expect.any(Number),
    method: 'eth_call',
    params: [{
      to: '0x0000000000000000000000000000000000001004',
      data: `${toFunctionSelector('layerXBindNonce(address)')}${'0'.repeat(64)}`,
    }, 'latest'],
  }]);
}

async function kernelAvailabilityReadsBindNonceView(): Promise<void> {
  for (const nonce of [0n, 42n, (1n << 64n) - 1n]) {
    const node = await startBindRpc({ result: encodeAbiParameters([{ type: 'uint64' }], [nonce]) });
    try {
      expect(await readKernelAvailability(node.rpc)).toEqual({ state: 'available' });
      expectBindNonceCall(node.calls);
    } finally {
      await node.close();
    }
  }
}

async function kernelAvailabilityUnavailableWhenAddrCallFails(): Promise<void> {
  const node = await startBindRpc({ error: { code: 3, message: 'execution reverted', data: '0x' } });
  try {
    expect(await readKernelAvailability(node.rpc)).toEqual({
      state: 'unavailable',
      reason: 'kernel bind nonce could not be read: execution reverted',
    });
    expectBindNonceCall(node.calls);
  } finally {
    await node.close();
  }
}

test(kernelAvailabilityReadsBindNonceView);
test(kernelAvailabilityUnavailableWhenAddrCallFails);
