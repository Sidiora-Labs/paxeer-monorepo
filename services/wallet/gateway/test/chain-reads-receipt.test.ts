import { randomBytes } from 'node:crypto';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import type { Hex } from 'viem';
import { EnvSchema, env } from '../src/env.js';
import { waitForReceipt } from '../src/chainReads.js';
import { sharedRpcPool, type SequencerReceiptResult } from '../src/rpc/pool.js';

async function actionReceiptWaitUsesSequencerReceipt(): Promise<void> {
  const successHash: Hex = `0x${randomBytes(32).toString('hex')}`;
  const revertedHash: Hex = `0x${randomBytes(32).toString('hex')}`;
  const missingHash: Hex = `0x${randomBytes(32).toString('hex')}`;
  const defaults = EnvSchema.parse({
    CORS_ORIGINS: 'http://localhost:3000',
    SUPABASE_URL: 'https://supabase.test.invalid',
    DATABASE_URL: 'postgres://localhost:5432/test',
    WALLET_MASTER_KEY: randomBytes(32).toString('base64'),
  });
  const receipts = new Map<Hex, SequencerReceiptResult>(
    [successHash, revertedHash].map((hash) => [
      hash,
      {
        txHash: hash,
        nonce: '0x0',
        rung: 'receipted',
        seq: '0x2a',
        height: '0x64',
        index: '0x0',
        timestamp: '0x6553f100',
        status: hash === successHash ? '0x1' : '0x0',
        returnData: '0x',
        logs: [],
        gasUsed: '0x5208',
        cumulativeGasUsed: '0x5208',
        effectiveGasPrice: '0x3b9aca00',
        feeDenom: 'upax',
        feeAmount: '0x7',
        sponsor: '0x0000000000000000000000000000000000000000',
        from: '0x0000000000000000000000000000000000000001',
        to: '0x0000000000000000000000000000000000000002',
        contractAddress: '0x0000000000000000000000000000000000000000',
        vmError: hash === successHash ? '' : 'execution reverted',
        keyId: `0x${'34'.repeat(32)}`,
        receipt: '0x010203040506070809',
      },
    ]),
  );

  const original = {
    SEQUENCER_RPC_URL: env.SEQUENCER_RPC_URL,
    RPC_URLS: env.RPC_URLS,
    HYPERPAXEER_RPC_URL: env.HYPERPAXEER_RPC_URL,
    ACTION_CONFIRMATIONS: env.ACTION_CONFIRMATIONS,
    RPC_TIMEOUT_MS: env.RPC_TIMEOUT_MS,
    RPC_HEALTH_INTERVAL_MS: env.RPC_HEALTH_INTERVAL_MS,
  };
  const calls: { path: string; method: string; params: unknown[] }[] = [];
  const errors: unknown[] = [];
  const server = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
        id: number;
        method: string;
        params: Hex[];
      };
      calls.push({ path: req.url!, method: body.method, params: body.params });
      res.writeHead(200, { 'content-type': 'application/json' });
      if (req.url === '/sequencer' && body.method === 'pax_sequencerReceipt') {
        res.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id: body.id,
            result: receipts.get(body.params[0]!) ?? null,
          }),
        );
      } else if (req.url === '/read' && body.method === 'eth_blockNumber') {
        res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result: '0x64' }));
      } else {
        res.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id: body.id,
            error: { code: -32601, message: 'method not found' },
          }),
        );
      }
    } catch (error) {
      errors.push(error);
      res.destroy(error instanceof Error ? error : new Error(String(error)));
    }
  });

  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  env.SEQUENCER_RPC_URL = `${url}/sequencer`;
  env.RPC_URLS = [`${url}/read`];
  env.HYPERPAXEER_RPC_URL = `${url}/read`;
  env.ACTION_CONFIRMATIONS = defaults.ACTION_CONFIRMATIONS;
  env.RPC_TIMEOUT_MS = 500;
  env.RPC_HEALTH_INTERVAL_MS = 60_000;

  const rpc = sharedRpcPool();
  try {
    expect(env.ACTION_CONFIRMATIONS).toBe(1);
    for (const [hash, status, state] of [
      [successHash, '0x1', 'confirmed'],
      [revertedHash, '0x0', 'reverted'],
    ] as const) {
      const receipt = receipts.get(hash);
      expect(receipt).not.toBeNull();
      expect(receipt?.rung).not.toBe('submitted');
      if (!receipt || receipt.rung === 'submitted') {
        throw new Error('transaction must have an executed sequencer receipt');
      }
      expect(receipt.status).toBe(status);
      expect(receipt.txHash).toBe(hash);
      const start = calls.length;
      expect(await waitForReceipt(hash, { timeoutMs: 500, pollMs: 5 })).toEqual({
        state,
        receipt: {
          status: state === 'confirmed' ? 'success' : 'reverted',
          block_number: BigInt(
            receipt.rung === 'committed' ? receipt.blockNumber : receipt.height,
          ).toString(),
          gas_used: BigInt(receipt.gasUsed).toString(),
          tx_hash: hash,
        },
      });
      expect(calls.slice(start).filter((call) => call.path === '/sequencer')).toEqual([
        { path: '/sequencer', method: 'pax_sequencerReceipt', params: [hash] },
      ]);
    }

    const start = calls.length;
    const startedAt = Date.now();
    expect(await waitForReceipt(missingHash, { timeoutMs: 50, pollMs: 5 })).toEqual({
      state: 'timeout',
    });
    expect(Date.now() - startedAt).toBeGreaterThanOrEqual(50);
    const polls = calls.slice(start).filter((call) => call.path === '/sequencer');
    expect(polls.length).toBeGreaterThan(1);
    expect(polls).toEqual(
      polls.map(() => ({
        path: '/sequencer',
        method: 'pax_sequencerReceipt',
        params: [missingHash],
      })),
    );
    expect(
      calls.some(
        (call) =>
          call.method === 'eth_getTransactionReceipt' || call.method === 'eth_getTransactionByHash',
      ),
    ).toBe(false);
    expect(errors).toEqual([]);
  } finally {
    rpc.stop();
    Object.assign(env, original);
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
}

test(actionReceiptWaitUsesSequencerReceipt, actionReceiptWaitUsesSequencerReceipt);
