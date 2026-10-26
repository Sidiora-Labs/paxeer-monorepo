import { afterAll, beforeAll, beforeEach, describe, expect, it, test } from 'vitest';
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { keccak256, type Hex } from 'viem';
import {
  PendingReadRefusedError,
  RpcPool,
  RpcResponseError,
  RpcUnavailableError,
  rpcPoolFromConfig,
  type SequencerReceiptResult,
} from '../src/rpc/pool.js';
import { env } from '../src/env.js';

interface Node {
  server: Server;
  url: string;
  failing: boolean;
  head: number;
  calls: string[];
  requests: { method: string; params: unknown[] }[];
  revertCalls: boolean;
  sendError: { code: number; message: string } | null;
  receipt: SequencerReceiptResult | null;
}

const nodes: Node[] = [];

function startNode(): Promise<Node> {
  const node: Node = {
    server: null as unknown as Server, url: '', failing: false, head: 1000, calls: [], requests: [],
    revertCalls: false, sendError: null, receipt: null,
  };
  node.server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (c: Buffer) => chunks.push(c));
    req.on('end', () => {
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as { id: number; method: string; params: unknown[] };
      node.calls.push(body.method);
      node.requests.push({ method: body.method, params: body.params });
      if (node.failing) {
        res.writeHead(500, { 'content-type': 'text/plain' });
        res.end('upstream unavailable');
        return;
      }
      const reply = (payload: Record<string, unknown>): void => {
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, ...payload }));
      };
      switch (body.method) {
        case 'eth_blockNumber':
          return reply({ result: `0x${node.head.toString(16)}` });
        case 'eth_chainId':
          return reply({ result: '0x7d' });
        case 'eth_getTransactionCount':
          if (body.params[1] === 'pending') return reply({ error: { code: -32000, message: 'pending block is not available' } });
          return reply({ result: '0x9' });
        case 'eth_call':
          if (node.revertCalls) return reply({ error: { code: 3, message: 'execution reverted', data: '0x' } });
          return reply({ result: '0x' });
        case 'eth_estimateGas':
          return reply({ result: '0x5208' });
        case 'eth_maxPriorityFeePerGas':
          return reply({ result: '0x3b9aca00' });
        case 'eth_getBlockByNumber':
          return reply({
            result: {
              number: `0x${node.head.toString(16)}`,
              hash: `0x${'ab'.repeat(32)}`,
              parentHash: `0x${'cd'.repeat(32)}`,
              timestamp: '0x6500',
              gasLimit: '0x1c9c380',
              gasUsed: '0x0',
              baseFeePerGas: '0x77359400',
              transactions: [],
            },
          });
        case 'eth_sendRawTransaction':
          if (node.sendError) return reply({ error: node.sendError });
          return reply({ result: `0x${'11'.repeat(32)}` });
        case 'pax_sequencerReceipt':
          return reply({ result: node.receipt });
        default:
          return reply({ error: { code: -32601, message: 'method not found' } });
      }
    });
  });
  return new Promise((r) =>
    node.server.listen(0, '127.0.0.1', () => {
      node.url = `http://127.0.0.1:${(node.server.address() as AddressInfo).port}`;
      nodes.push(node);
      r(node);
    }),
  );
}

let a: Node;
let b: Node;
let sequencer: Node;

function pool(lag = 20): RpcPool {
  return new RpcPool({ urls: [a.url, b.url], sequencerUrl: a.url, chainId: 125, lagThresholdBlocks: lag, timeoutMs: 3_000, healthIntervalMs: 60_000 });
}

beforeAll(async () => {
  a = await startNode();
  b = await startNode();
  sequencer = await startNode();
});

beforeEach(() => {
  for (const n of nodes) {
    n.failing = false;
    n.head = 1000;
    n.calls = [];
    n.requests = [];
    n.revertCalls = false;
    n.sendError = null;
    n.receipt = null;
  }
});

afterAll(async () => {
  await Promise.all(nodes.map((n) => new Promise<void>((r) => n.server.close(() => r()))));
});

async function writeGoesToSequencerEndpointOnly(): Promise<void> {
  const p = rpcPoolFromConfig({ ...env, RPC_URLS: [a.url, b.url], SEQUENCER_RPC_URL: sequencer.url });
  const signed: Hex = '0x02f86b';
  expect(await p.sendRawTransactionVia(signed)).toEqual({ hash: `0x${'11'.repeat(32)}`, member: sequencer.url });
  expect(sequencer.requests).toEqual([{ method: 'eth_sendRawTransaction', params: [signed] }]);
  expect(a.calls).toEqual([]);
  expect(b.calls).toEqual([]);

  sequencer.sendError = { code: -32000, message: 'already known' };
  expect(await p.sendRawTransactionVia(signed)).toEqual({ hash: keccak256(signed), member: sequencer.url });
  expect(sequencer.calls).toEqual(['eth_sendRawTransaction', 'eth_sendRawTransaction']);

  sequencer.failing = true;
  await expect(p.sendRawTransactionVia(signed)).rejects.toThrow('HTTP 500');
  expect(sequencer.requests).toEqual(Array.from({ length: 3 }, () => ({ method: 'eth_sendRawTransaction', params: [signed] })));
  expect(a.calls).toEqual([]);
  expect(b.calls).toEqual([]);

  const unconfigured = rpcPoolFromConfig({ ...env, RPC_URLS: [a.url, b.url], SEQUENCER_RPC_URL: undefined });
  await expect(unconfigured.sendRawTransactionVia(signed)).rejects.toThrow('SEQUENCER_RPC_URL is not configured');
  expect(a.calls).toEqual([]);
  expect(b.calls).toEqual([]);

  a.failing = true;
  expect(await p.getTransactionCount('0x00000000000000000000000000000000000000b1')).toBe(9);
  expect(a.calls).toEqual(['eth_getTransactionCount']);
  expect(b.calls).toEqual(['eth_getTransactionCount']);
  expect(sequencer.calls).toHaveLength(3);
}

async function sequencerReceiptReadsPaxSequencerReceipt(): Promise<void> {
  const p = rpcPoolFromConfig({ ...env, RPC_URLS: [a.url, b.url], SEQUENCER_RPC_URL: sequencer.url });
  const hash: Hex = `0x${'11'.repeat(32)}`;
  expect(await p.sequencerReceipt(hash)).toBeNull();
  expect(sequencer.requests).toEqual([{ method: 'pax_sequencerReceipt', params: [hash] }]);

  sequencer.receipt = {
    rung: 'receipted', txHash: hash, nonce: '0x9', seq: '0x2a', height: '0x3e8', index: '0x0',
    timestamp: '0x6500', status: '0x1', returnData: '0xabcd',
    logs: [{ address: '0x00000000000000000000000000000000000000b1', topics: [hash], data: '0xab', logIndex: '0x0' }],
    gasUsed: '0x5208', cumulativeGasUsed: '0x5208', effectiveGasPrice: '0x7', feeDenom: 'apax', feeAmount: '0x23e38',
    sponsor: '0x0000000000000000000000000000000000000000', from: '0x00000000000000000000000000000000000000b1',
    to: '0x00000000000000000000000000000000000000b2', contractAddress: '0x0000000000000000000000000000000000000000',
    vmError: '', keyId: `0x${'22'.repeat(32)}`, receipt: '0x01020304',
  };
  expect(await p.sequencerReceipt(hash)).toEqual(sequencer.receipt);

  sequencer.receipt = { ...sequencer.receipt, rung: 'committed', blockNumber: '0x3e9', blockHash: hash };
  expect(await p.sequencerReceipt(hash)).toEqual(sequencer.receipt);

  sequencer.receipt = { rung: 'submitted', txHash: hash, nonce: '0x9' };
  expect(await p.sequencerReceipt(hash)).toEqual(sequencer.receipt);

  sequencer.failing = true;
  await expect(p.sequencerReceipt(hash)).rejects.toThrow('HTTP 500');
  expect(sequencer.requests).toEqual(Array.from({ length: 5 }, () => ({ method: 'pax_sequencerReceipt', params: [hash] })));
  expect(a.calls).toEqual([]);
  expect(b.calls).toEqual([]);

  const unconfigured = rpcPoolFromConfig({ ...env, RPC_URLS: [a.url, b.url], SEQUENCER_RPC_URL: undefined });
  await expect(unconfigured.sequencerReceipt(hash)).rejects.toThrow('SEQUENCER_RPC_URL is not configured');
  expect(sequencer.calls).toHaveLength(5);
  expect(a.calls).toEqual([]);
  expect(b.calls).toEqual([]);
}

test(writeGoesToSequencerEndpointOnly, writeGoesToSequencerEndpointOnly);
test(sequencerReceiptReadsPaxSequencerReceipt, sequencerReceiptReadsPaxSequencerReceipt);

describe('RpcPool', () => {
  it('fails over from a failing endpoint and takes it back once it recovers', async () => {
    const p = pool();
    a.failing = true;
    expect(await p.getTransactionCount('0x00000000000000000000000000000000000000b1')).toBe(9);
    expect(a.calls).toContain('eth_getTransactionCount');
    expect(b.calls).toContain('eth_getTransactionCount');
    expect(p.status().find((s) => s.url === a.url)?.state).toBe('down');

    a.calls = [];
    b.calls = [];
    expect(await p.request('eth_chainId')).toBe('0x7d');
    expect(a.calls).toEqual([]);
    expect(b.calls).toEqual(['eth_chainId']);

    a.failing = false;
    const status = await p.checkHealth();
    expect(status.map((s) => s.state)).toEqual(['healthy', 'healthy']);
    expect(p.healthyCount()).toBe(2);

    b.failing = true;
    a.calls = [];
    const hash = await p.sendRawTransaction('0x02f86b');
    expect(hash).toBe(`0x${'11'.repeat(32)}`);
    expect(a.calls).toContain('eth_sendRawTransaction');
  });

  it('removes an endpoint that lags the best head and refuses rather than reading stale state', async () => {
    const p = pool(20);
    b.head = 900;
    const status = await p.checkHealth();
    expect(status.find((s) => s.url === b.url)?.state).toBe('lagging');
    expect(status.find((s) => s.url === a.url)?.state).toBe('healthy');
    b.calls = [];
    await p.request('eth_chainId');
    expect(b.calls).toEqual([]);

    a.failing = true;
    await expect(p.request('eth_chainId')).rejects.toBeInstanceOf(RpcUnavailableError);
    expect(b.calls).toEqual([]);

    a.failing = false;
    b.head = 995;
    await p.checkHealth();
    expect(p.healthyCount()).toBe(2);
  });

  it('surfaces a JSON-RPC error without failing over', async () => {
    const p = pool();
    await p.checkHealth();
    a.revertCalls = true;
    b.revertCalls = true;
    a.calls = [];
    b.calls = [];
    const err = await p
      .simulate({ from: '0x00000000000000000000000000000000000000b2', to: '0x00000000000000000000000000000000000000b3', data: '0x' })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(RpcResponseError);
    expect((err as RpcResponseError).code).toBe(3);
    expect(a.calls.length + b.calls.length).toBe(1);
    expect(p.healthyCount()).toBe(2);
  });

  it('reads the nonce at latest and surfaces a -32000 pending refusal as a typed error without failing over', async () => {
    const p = pool();
    await p.checkHealth();
    a.calls = [];
    b.calls = [];
    expect(await p.getTransactionCount('0x00000000000000000000000000000000000000b4')).toBe(9);
    a.calls = [];
    b.calls = [];
    const err = await p
      .request('eth_getTransactionCount', ['0x00000000000000000000000000000000000000b4', 'pending'])
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PendingReadRefusedError);
    expect(err).toBeInstanceOf(RpcResponseError);
    expect((err as PendingReadRefusedError).code).toBe(-32000);
    expect(a.calls.length + b.calls.length).toBe(1);
    expect(p.healthyCount()).toBe(2);
  });

  it('estimates gas and fees through the pool', async () => {
    const p = pool();
    a.failing = true;
    const pinned = await p.prepareGas({
      from: '0x00000000000000000000000000000000000000b4',
      to: '0x00000000000000000000000000000000000000b5',
      value: 1n,
      maxFeePerGas: 7n,
      maxPriorityFeePerGas: 3n,
    });
    expect(pinned).toEqual({ gas: 25_200n, maxFeePerGas: 7n, maxPriorityFeePerGas: 3n });

    const live = await p.prepareGas({ from: '0x00000000000000000000000000000000000000b4', gas: 50_000n });
    expect(live.gas).toBe(50_000n);
    expect(live.maxPriorityFeePerGas).toBe(1_000_000_000n);
    expect(live.maxFeePerGas).toBeGreaterThan(2_000_000_000n);
    expect(b.calls).toContain('eth_getBlockByNumber');
  });

  it('defaults to the single router URL without contacting it', () => {
    const p = rpcPoolFromConfig(env);
    const urls = p.status().map((s) => s.url);
    expect(urls).toEqual(['https://api-mainnet-beta.paxeer.network']);
    expect(p.status().every((s) => s.state === 'unknown' && s.checkedAt === null)).toBe(true);
  });
});
