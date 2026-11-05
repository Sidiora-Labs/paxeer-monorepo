import { createServer, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { PendingReadRefusedError, RpcPool, RpcUnavailableError } from '../src/rpc/pool.js';

interface RpcRequest {
  id: number;
  method: string;
  params: unknown[];
}

interface ServerState {
  requests: RpcRequest[];
  cancelled: number;
  errors: unknown[];
}

function pool(urls: string[], timeoutMs = 2_000): RpcPool {
  return new RpcPool({
    urls,
    chainId: 125,
    lagThresholdBlocks: 20,
    timeoutMs,
    healthIntervalMs: 60_000,
  });
}

async function withServer(
  answer: (request: RpcRequest, response: ServerResponse) => void,
  run: (url: string, state: ServerState) => Promise<void>,
): Promise<void> {
  const state: ServerState = { requests: [], cancelled: 0, errors: [] };
  const server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    res.once('close', () => {
      if (!res.writableFinished) state.cancelled++;
    });
    req.on('error', (error) => state.errors.push(error));
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      try {
        const request = JSON.parse(Buffer.concat(chunks).toString('utf8')) as RpcRequest;
        state.requests.push(request);
        answer(request, res);
      } catch (error) {
        state.errors.push(error);
        res.destroy();
      }
    });
  });
  try {
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    await run(url, state);
    expect(state.errors).toEqual([]);
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
  }
}

async function headersOnlyBodyTimeoutReleasesConnection(): Promise<void> {
  await withServer((_request, res) => {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.flushHeaders();
  }, async (url, state) => {
    const rpc = pool([url], 200);
    const started = performance.now();
    await expect(rpc.request('eth_chainId')).rejects.toThrow('RPC deadline exceeded after 200ms');
    expect(performance.now() - started).toBeLessThan(1_500);
    await expect.poll(() => state.cancelled).toBe(1);
    expect(state.requests).toMatchObject([{ method: 'eth_chainId', params: [] }]);
    expect(rpc.status()[0]).toMatchObject({ state: 'down' });
  });
}

async function slowBodyCannotExtendAbsoluteDeadline(): Promise<void> {
  let writes = 0;
  await withServer((request, res) => {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.write(`{"jsonrpc":"2.0","id":${request.id},"result":"`);
    const timer = setInterval(() => {
      writes++;
      res.write('a');
    }, 10);
    res.once('close', () => clearInterval(timer));
  }, async (url, state) => {
    const started = performance.now();
    await expect(pool([url], 200).request('eth_chainId')).rejects.toThrow('RPC deadline exceeded after 200ms');
    expect(performance.now() - started).toBeLessThan(1_500);
    await expect.poll(() => state.cancelled).toBe(1);
    expect(writes).toBeGreaterThan(1);
  });
}

async function chunkedOversizeBodyAbortsBeforeTimeout(): Promise<void> {
  await withServer((request, res) => {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.write(`{"jsonrpc":"2.0","id":${request.id},"result":"`);
    res.write('a'.repeat(64 * 1024));
  }, async (url, state) => {
    const started = performance.now();
    await expect(pool([url]).request('eth_chainId')).rejects.toThrow('response exceeds 65536 byte limit');
    expect(performance.now() - started).toBeLessThan(1_500);
    await expect.poll(() => state.cancelled).toBe(1);
  });
}

async function bodyLimitCountsUtf8Bytes(): Promise<void> {
  const result = 'é'.repeat(40 * 1024);
  expect(result.length).toBeLessThan(64 * 1024);
  expect(Buffer.byteLength(result)).toBeGreaterThan(64 * 1024);
  await withServer((request, res) => {
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }));
  }, async (url) => {
    await expect(pool([url]).request('eth_chainId')).rejects.toThrow('response exceeds 65536 byte limit');
  });
}

async function scalarLimitIncludesEnvelopeAndAcceptsExactBoundary(): Promise<void> {
  let replies = 0;
  await withServer((request, res) => {
    const body = { jsonrpc: '2.0', id: request.id, result: '0x7d', padding: '' };
    const envelopeBytes = Buffer.byteLength(JSON.stringify(body));
    body.padding = ' '.repeat(64 * 1024 - envelopeBytes + replies++);
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify(body));
  }, async (url, state) => {
    const rpc = pool([url]);
    expect(await rpc.request('eth_chainId')).toBe('0x7d');
    await expect(rpc.request('eth_chainId')).rejects.toThrow('response exceeds 65536 byte limit');
    expect(state.requests).toHaveLength(2);
  });
}

async function callUsesLargerBoundAndPreservesDecodedResult(): Promise<void> {
  const result = `0x${'ab'.repeat(48 * 1024)}`;
  const call = { from: '0x0000000000000000000000000000000000000001' as const };
  await withServer((request, res) => {
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }));
  }, async (url, state) => {
    expect(await pool([url]).simulate(call)).toBe(result);
    expect(state.requests).toMatchObject([{ method: 'eth_call', params: [call, 'latest'] }]);
  });
}

async function bulkAndDefaultMethodsRemainBounded(): Promise<void> {
  for (const [method, limit] of [['eth_getLogs', 8 * 1024 * 1024], ['eth_feeHistory', 1024 * 1024]] as const) {
    await withServer((request, res) => {
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result: 'a'.repeat(limit) }));
    }, async (url, state) => {
      await expect(pool([url]).request(method)).rejects.toThrow(`response exceeds ${limit} byte limit`);
      expect(state.requests).toMatchObject([{ method }]);
    });
  }
}

async function oversizeMemberFailsOverAfterReleasingBody(): Promise<void> {
  await withServer((request, res) => {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.write(`{"jsonrpc":"2.0","id":${request.id},"result":"${'a'.repeat(64 * 1024)}`);
  }, async (badUrl, badState) => {
    await withServer((request, res) => {
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result: '0x7d' }));
    }, async (goodUrl, goodState) => {
      const rpc = pool([badUrl, goodUrl]);
      expect(await rpc.requestVia('eth_chainId')).toEqual({ result: '0x7d', member: goodUrl });
      await expect.poll(() => badState.cancelled).toBe(1);
      expect(badState.requests).toHaveLength(1);
      expect(goodState.requests).toHaveLength(1);
      expect(rpc.status().map(({ state }) => state)).toEqual(['down', 'healthy']);
    });
  });
}

async function httpFailureReleasesUnreadBody(): Promise<void> {
  await withServer((_request, res) => {
    res.writeHead(503, { 'content-type': 'application/json' });
    res.flushHeaders();
  }, async (url, state) => {
    await expect(pool([url]).request('eth_chainId')).rejects.toThrow('HTTP 503');
    await expect.poll(() => state.cancelled).toBe(1);
  });
}

async function boundedMalformedBodyFailsAndRpcRefusalRetainsType(): Promise<void> {
  await withServer((request, res) => {
    res.setHeader('content-type', 'application/json');
    if (request.method === 'eth_chainId') res.end('{"result":');
    else res.end(JSON.stringify({
      jsonrpc: '2.0', id: request.id,
      error: { code: -32000, message: 'pending block unavailable', data: '0x' },
    }));
  }, async (url, state) => {
    await expect(pool([url]).request('eth_chainId')).rejects.toBeInstanceOf(RpcUnavailableError);
    await expect(pool([url]).request('eth_chainId')).rejects.toThrow('non-JSON answer');
    await expect(pool([url]).request('eth_getTransactionCount', [
      '0x0000000000000000000000000000000000000001', 'pending',
    ])).rejects.toBeInstanceOf(PendingReadRefusedError);
    expect(state.requests).toHaveLength(3);
  });
}

test(headersOnlyBodyTimeoutReleasesConnection.name, headersOnlyBodyTimeoutReleasesConnection);
test(slowBodyCannotExtendAbsoluteDeadline.name, slowBodyCannotExtendAbsoluteDeadline);
test(chunkedOversizeBodyAbortsBeforeTimeout.name, chunkedOversizeBodyAbortsBeforeTimeout);
test(bodyLimitCountsUtf8Bytes.name, bodyLimitCountsUtf8Bytes);
test(scalarLimitIncludesEnvelopeAndAcceptsExactBoundary.name, scalarLimitIncludesEnvelopeAndAcceptsExactBoundary);
test(callUsesLargerBoundAndPreservesDecodedResult.name, callUsesLargerBoundAndPreservesDecodedResult);
test(bulkAndDefaultMethodsRemainBounded.name, bulkAndDefaultMethodsRemainBounded);
test(oversizeMemberFailsOverAfterReleasingBody.name, oversizeMemberFailsOverAfterReleasingBody);
test(httpFailureReleasesUnreadBody.name, httpFailureReleasesUnreadBody);
test(boundedMalformedBodyFailsAndRpcRefusalRetainsType.name, boundedMalformedBodyFailsAndRpcRefusalRetainsType);
