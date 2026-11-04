/**
 * Task 22.33.182 / receipt-read-hides-outage: the named tests below drive the
 * production chain reader and viem codec through a loopback JSON-RPC peer.
 * Baseline verification was not run: the ordered gate contract requires the
 * entire implementation before hf-build/hf-done and forbids a manual verify.
 * The runner's hf-done evidence records the exact verified candidate revision.
 * Wiring request (wallet domain): services/wallet/gateway/src/routes/agent.ts should map
 * ChainReadUnavailableError to a retryable unavailable response, preserving 404
 * exclusively for an explicitly absent receipt.
 * Wiring request (wallet domain): services/wallet/gateway/src/agent/actions/orchestrator.ts should park
 * unavailable observations for reconciliation without replacing or rebroadcasting.
 */
import { createServer, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { Hex } from 'viem';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';

const hash: Hex = `0x${'ab'.repeat(32)}`;
const otherHash: Hex = `0x${'cd'.repeat(32)}`;
const address = `0x${'11'.repeat(20)}`;
const receipt = {
  transactionHash: hash,
  transactionIndex: '0x0',
  blockHash: otherHash,
  blockNumber: '0x64',
  from: address,
  to: address,
  cumulativeGasUsed: '0x5208',
  gasUsed: '0x5208',
  contractAddress: null,
  logs: [],
  logsBloom: `0x${'00'.repeat(256)}`,
  status: '0x1',
  effectiveGasPrice: '0x3b9aca00',
  type: '0x2',
};
const pendingTransaction = {
  hash,
  blockHash: null,
  blockNumber: null,
  transactionIndex: null,
  from: address,
  to: address,
  nonce: '0x0',
  gas: '0x5208',
  value: '0x1',
  input: '0x',
  type: '0x2',
  chainId: '0x7d',
  maxFeePerGas: '0x77359400',
  maxPriorityFeePerGas: '0x3b9aca00',
  v: '0x0',
  r: `0x${'12'.repeat(32)}`,
  s: `0x${'34'.repeat(32)}`,
};

type Method = 'eth_getTransactionReceipt' | 'eth_getTransactionByHash';
type Answer =
  | { result: unknown }
  | { failure: 'http' | 'disconnect' | 'invalid-json' | 'missing-result' | 'rpc-error' };
const answers = new Map<Method, Answer>();
const requests: { method: Method; params: Hex[] }[] = [];
const serverErrors: unknown[] = [];
let server: Server | undefined;
let reads: typeof import('../src/chainReads.js');
let rpc: typeof import('../src/rpc/pool.js');
const originalUrl = process.env.HYPERPAXEER_RPC_URL;

beforeAll(async () => {
  server = createServer(async (req, res) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of req) chunks.push(Buffer.from(chunk));
      const body = JSON.parse(Buffer.concat(chunks).toString('utf8')) as {
        id: number; method: Method; params: Hex[];
      };
      requests.push({ method: body.method, params: body.params });
      const answer = answers.get(body.method);
      if (!answer) throw new Error(`unexpected RPC method: ${body.method}`);
      if ('failure' in answer) {
        switch (answer.failure) {
          case 'disconnect':
            req.socket.destroy();
            return;
          case 'http':
            res.writeHead(503);
            res.end('node temporarily unavailable');
            return;
          case 'invalid-json':
            res.end('{"jsonrpc":');
            return;
          case 'missing-result':
            res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id }));
            return;
          case 'rpc-error':
            res.end(JSON.stringify({
              jsonrpc: '2.0', id: body.id,
              error: { code: -32000, message: 'transaction index unavailable' },
            }));
            return;
        }
      }
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result: answer.result }));
    } catch (error) {
      serverErrors.push(error);
      res.destroy();
    }
  });
  await new Promise<void>((resolve, reject) => {
    server!.once('error', reject);
    server!.listen(0, '127.0.0.1', resolve);
  });
  process.env.HYPERPAXEER_RPC_URL = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  reads = await import('../src/chainReads.js');
  rpc = await import('../src/rpc/pool.js');
});

beforeEach(() => {
  answers.clear();
  requests.length = 0;
  serverErrors.length = 0;
  answers.set('eth_getTransactionReceipt', { result: null });
  answers.set('eth_getTransactionByHash', { result: null });
});

afterEach(() => {
  expect(serverErrors).toEqual([]);
  expect(requests.length).toBeGreaterThan(0);
  for (const request of requests) expect(request.params).toEqual([hash]);
});

afterAll(async () => {
  if (originalUrl === undefined) delete process.env.HYPERPAXEER_RPC_URL;
  else process.env.HYPERPAXEER_RPC_URL = originalUrl;
  if (server) {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server!.close(() => resolve()));
  }
});

async function expectUnavailable(method: Method, read: Promise<unknown>): Promise<void> {
  const error: unknown = await read.catch((cause: unknown) => cause);
  expect(error).toBeInstanceOf(reads.ChainReadUnavailableError);
  expect(error).toBeInstanceOf(rpc.RpcUnavailableError);
  expect(error).toMatchObject({
    name: 'ChainReadUnavailableError', code: 'chain_read_unavailable', method,
    message: `${method} observation unavailable`,
  });
  if (!(error instanceof reads.ChainReadUnavailableError)) throw error;
  expect(error.cause).toBeInstanceOf(Error);
}

async function explicitReceiptAndTransactionAbsence(): Promise<void> {
  expect(await reads.getReceipt(hash)).toBeNull();
  expect(await reads.isTxKnown(hash)).toBe(false);
  expect(requests.map((request) => request.method)).toEqual([
    'eth_getTransactionReceipt', 'eth_getTransactionByHash', 'eth_getTransactionReceipt',
  ]);
}

async function pendingTransactionRemainsKnownWithoutReceipt(): Promise<void> {
  answers.set('eth_getTransactionByHash', { result: pendingTransaction });
  expect(await reads.getReceipt(hash)).toBeNull();
  expect(await reads.isTxKnown(hash)).toBe(true);
  expect(requests.map((request) => request.method)).toEqual([
    'eth_getTransactionReceipt', 'eth_getTransactionByHash',
  ]);
}

async function minedReceiptsPreserveSuccessAndRevert(): Promise<void> {
  for (const [status, expected] of [['0x1', 'success'], ['0x0', 'reverted']]) {
    answers.set('eth_getTransactionReceipt', { result: { ...receipt, status } });
    expect(await reads.getReceipt(hash)).toEqual({
      status: expected, block_number: '100', gas_used: '21000', tx_hash: hash,
    });
    // A missing transaction index entry still has affirmative receipt evidence.
    expect(await reads.isTxKnown(hash)).toBe(true);
  }
}

async function receiptHttpOutageIsUnavailable(): Promise<void> {
  answers.set('eth_getTransactionReceipt', { failure: 'http' });
  await expectUnavailable('eth_getTransactionReceipt', reads.getReceipt(hash));
  await expectUnavailable('eth_getTransactionReceipt', reads.isTxKnown(hash));
}

async function transactionDisconnectCannotAuthorizeAbsence(): Promise<void> {
  answers.set('eth_getTransactionByHash', { failure: 'disconnect' });
  await expectUnavailable('eth_getTransactionByHash', reads.isTxKnown(hash));
  expect(requests.every((request) => request.method === 'eth_getTransactionByHash')).toBe(true);
}

async function nodeRpcErrorsAreUnavailable(): Promise<void> {
  answers.set('eth_getTransactionReceipt', { failure: 'rpc-error' });
  await expectUnavailable('eth_getTransactionReceipt', reads.getReceipt(hash));
  answers.set('eth_getTransactionByHash', { failure: 'rpc-error' });
  await expectUnavailable('eth_getTransactionByHash', reads.isTxKnown(hash));
}

async function invalidJsonIsUnavailable(): Promise<void> {
  answers.set('eth_getTransactionReceipt', { failure: 'invalid-json' });
  await expectUnavailable('eth_getTransactionReceipt', reads.getReceipt(hash));
  answers.set('eth_getTransactionByHash', { failure: 'invalid-json' });
  await expectUnavailable('eth_getTransactionByHash', reads.isTxKnown(hash));
}

async function missingResultsAreUnavailable(): Promise<void> {
  answers.set('eth_getTransactionReceipt', { failure: 'missing-result' });
  await expectUnavailable('eth_getTransactionReceipt', reads.getReceipt(hash));
  answers.set('eth_getTransactionByHash', { failure: 'missing-result' });
  await expectUnavailable('eth_getTransactionByHash', reads.isTxKnown(hash));
}

async function malformedReceiptsNeverMeanAbsentOrConfirmed(): Promise<void> {
  for (const result of [
    false, 0, '', {}, [],
    { ...receipt, status: '0x2' },
    { ...receipt, status: undefined },
    { ...receipt, blockNumber: null },
    { ...receipt, gasUsed: 'not-a-quantity' },
    { ...receipt, gasUsed: '-0x1' },
    { ...receipt, transactionHash: otherHash },
    { ...receipt, logs: {} },
  ]) {
    answers.set('eth_getTransactionReceipt', { result });
    await expectUnavailable('eth_getTransactionReceipt', reads.getReceipt(hash));
    await expectUnavailable('eth_getTransactionReceipt', reads.isTxKnown(hash));
  }
}

async function malformedTransactionsCannotProveLiveness(): Promise<void> {
  for (const result of [
    false, 0, '', {}, [],
    { ...pendingTransaction, hash: otherHash },
    { ...pendingTransaction, hash: '0x12' },
    { ...pendingTransaction, blockNumber: undefined },
    { ...pendingTransaction, blockNumber: 'invalid' },
    { ...pendingTransaction, gas: 'invalid' },
  ]) {
    answers.set('eth_getTransactionByHash', { result });
    await expectUnavailable('eth_getTransactionByHash', reads.isTxKnown(hash));
  }
  expect(requests.every((request) => request.method === 'eth_getTransactionByHash')).toBe(true);
}

test(explicitReceiptAndTransactionAbsence.name, explicitReceiptAndTransactionAbsence);
test(pendingTransactionRemainsKnownWithoutReceipt.name, pendingTransactionRemainsKnownWithoutReceipt);
test(minedReceiptsPreserveSuccessAndRevert.name, minedReceiptsPreserveSuccessAndRevert);
test(receiptHttpOutageIsUnavailable.name, receiptHttpOutageIsUnavailable);
test(transactionDisconnectCannotAuthorizeAbsence.name, transactionDisconnectCannotAuthorizeAbsence);
test(nodeRpcErrorsAreUnavailable.name, nodeRpcErrorsAreUnavailable);
test(invalidJsonIsUnavailable.name, invalidJsonIsUnavailable);
test(missingResultsAreUnavailable.name, missingResultsAreUnavailable);
test(malformedReceiptsNeverMeanAbsentOrConfirmed.name, malformedReceiptsNeverMeanAbsentOrConfirmed);
test(malformedTransactionsCannotProveLiveness.name, malformedTransactionsCannotProveLiveness);
