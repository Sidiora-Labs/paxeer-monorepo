import { once } from 'node:events';
import type { ServerResponse } from 'node:http';
import { createServer } from 'node:http';
import type { Address, GetLogsParameters, Hex, RpcLog } from 'viem';
import { createPublicClient, encodeAbiParameters, encodeEventTopics, http, parseAbiItem, parseEventLogs, toHex } from 'viem';

import { afterAll, beforeAll, expect, test } from 'vitest';

import getLogs, { IncompleteLogScanError } from './getLogs';

beforeAll(() => {
  fetchMock.disableMocks();
});
afterAll(() => {
  fetchMock.enableMocks();
});

const token: Address = '0x1111111111111111111111111111111111111111';
const owner: Address = '0x2222222222222222222222222222222222222222';
const spender: Address = '0x3333333333333333333333333333333333333333';
const approval = parseAbiItem('event Approval(address indexed owner, address indexed spender, uint256 value)');
const filter = { address: token } satisfies GetLogsParameters;
const rangeError = { code: -32005, message: 'block range is too wide' };
const resultLimitError = { code: -32000, message: 'query returned more than 10000 results' };
const parameterRangeError = { code: -32602, message: 'requested block range exceeds the limit' };
const storedLogs: Array<RpcLog> = Array.from({ length: 16 }, (_, block) => ({
  address: token,
  topics: encodeEventTopics({ abi: [ approval ], eventName: 'Approval', args: { owner, spender } }) as [Hex, ...Array<Hex>],
  data: encodeAbiParameters([ { type: 'uint256' } ], [ BigInt(block + 1) ]),
  blockNumber: toHex(block),
  blockHash: toHex(block + 1, { size: 32 }),
  transactionHash: toHex(block + 101, { size: 32 }),
  transactionIndex: '0x0',
  logIndex: '0x0',
  removed: false,
}));

type Mode = 'split' | 'single' | 'partial' | 'budget' | 'hang' | 'auth' | 'rate' | 'internal' | 'rangeTextHttp';
type RpcFilter = { address: Address; fromBlock: Hex; toBlock: Hex; topics: Array<Hex | null> };
type LogRequest = { id: number; method: string; params: [RpcFilter] };

async function startLogStore(mode: Mode, limitError = rangeError) {
  const requests: Array<RpcFilter> = [];
  const invalidRequests: Array<unknown> = [];
  let active = 0;
  let maxActive = 0;
  let disconnected = 0;
  let notifyRequest: () => void = () => {};
  let notifyDisconnect: () => void = () => {};
  const received = new Promise<void>((resolve) => {
    notifyRequest = resolve;
  });
  const closed = new Promise<void>((resolve) => {
    notifyDisconnect = resolve;
  });
  const server = createServer(async(request, response: ServerResponse) => {
    let chunks = 0;
    response.on('close', () => {
      active -= chunks;
      if (!response.writableEnded) {
        disconnected++;
        notifyDisconnect();
      }
    });
    let body = '';
    for await (const chunk of request) body += chunk.toString();
    const decoded = JSON.parse(body) as LogRequest | Array<LogRequest>;
    const calls = Array.isArray(decoded) ? decoded : [ decoded ];
    chunks = calls.length;
    active += chunks;
    maxActive = Math.max(maxActive, active);
    const replies = calls.map((rpc) => {
      const query = rpc.params?.[0];
      if (rpc.method !== 'eth_getLogs' || !query || query.address !== token ||
        query.topics.length !== 0 || request.headers['x-scan-fixture'] !== 'approvals') {
        invalidRequests.push(rpc);
        return { jsonrpc: '2.0', id: rpc.id, error: { code: -32600, message: 'Invalid log query' } };
      }
      requests.push(query);
      const from = BigInt(query.fromBlock);
      const to = BigInt(query.toBlock);
      let error: typeof rangeError | undefined;
      if (mode === 'rate') error = { code: -32005, message: 'rate limit exceeded' };
      else if (mode === 'internal') error = { code: -32603, message: 'Internal error' };
      else if (mode === 'single' || mode === 'budget' || (mode === 'partial' && from >= BigInt(2)) || to - from >= BigInt(2)) error = limitError;
      const result = storedLogs.filter((log) => BigInt(log.blockNumber!) >= from && BigInt(log.blockNumber!) <= to);
      return { jsonrpc: '2.0', id: rpc.id, ...(error ? { error } : { result }) };
    });
    notifyRequest();
    if (mode === 'hang') return;
    response.setHeader('Content-Type', 'application/json');
    if (mode === 'auth' || mode === 'rangeTextHttp') {
      response.writeHead(401).end(JSON.stringify({ error: mode === 'auth' ? 'Unauthorized' : 'block range is too wide' }));
      return;
    }
    // Both batched and individual requests use the same committed log store.
    const reply = () => response.end(JSON.stringify(Array.isArray(decoded) ? replies : replies[0]));
    if (mode === 'budget') reply();
    else setTimeout(reply, 1);
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a loopback TCP listener');
  const client = createPublicClient({
    transport: http(`http://127.0.0.1:${ address.port }`, {
      retryCount: 3, retryDelay: 1, batch: { wait: 1 }, fetchOptions: { headers: { 'x-scan-fixture': 'approvals' } },
    }),
  });
  return {
    client,
    requests,
    received,
    closed,
    get maxActive() {
      return maxActive;
    },
    get disconnected() {
      return disconnected;
    },
    async stop() {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
      expect(invalidRequests).toEqual([]);
    },
  };
}

test('logSplitTerminationReturnsEveryDecodedApprovalInOrder', async function logSplitTerminationReturnsEveryDecodedApprovalInOrder() {
  for (const limit of [ rangeError, resultLimitError, parameterRangeError ]) {
    const store = await startLogStore('split', limit);
    try {
      const logs = await getLogs(store.client, filter, BigInt(0), BigInt(15));
      expect(logs.map((log) => log.blockNumber)).toEqual(storedLogs.map((log) => BigInt(log.blockNumber!)));
      expect(logs.map((log) => log.transactionHash)).toEqual(storedLogs.map((log) => log.transactionHash));
      const approvals = parseEventLogs({ abi: [ approval ], logs, args: { owner }, strict: true });
      expect(approvals).toHaveLength(16);
      expect(approvals.map((log) => log.args)).toEqual(
        storedLogs.map((_, block) => ({ owner, spender, value: BigInt(block + 1) })),
      );
      expect(store.requests).toHaveLength(15);
      expect(store.maxActive).toBe(1);
      for (const query of store.requests) expect(BigInt(query.fromBlock)).toBeLessThanOrEqual(BigInt(query.toBlock));
    } finally {
      await store.stop();
    }
  }
});

test('logSplitTerminationRejectsInvalidRangesBeforeRpc', async function logSplitTerminationRejectsInvalidRangesBeforeRpc() {
  const store = await startLogStore('split');
  try {
    for (const [ from, to ] of [ [ -1, 0 ], [ 2, 1 ], [ -2, -1 ] ]) {
      await expect(getLogs(store.client, filter, BigInt(from), BigInt(to))).rejects.toBeInstanceOf(RangeError);
    }
    await expect(getLogs(store.client, { blockHash: toHex(1, { size: 32 }), events: undefined }, BigInt(0), BigInt(2))).rejects.toBeInstanceOf(RangeError);
    expect(store.requests).toHaveLength(0);
  } finally {
    await store.stop();
  }
});

test('logSplitTerminationDoesNotSplitOrRetryOtherFailures', async function logSplitTerminationDoesNotSplitOrRetryOtherFailures() {
  for (const mode of [ 'auth', 'rate', 'internal', 'rangeTextHttp' ] as const) {
    const store = await startLogStore(mode);
    try {
      await expect(getLogs(store.client, filter, BigInt(0), BigInt(15))).rejects.toMatchObject({
        name: 'IncompleteLogScanError', fromBlock: BigInt(0), toBlock: BigInt(15), cause: expect.any(Error),
      });
      expect(store.requests).toHaveLength(1);
    } finally {
      await store.stop();
    }
  }
});

test('logSplitTerminationStopsAtOneBlockAndDiscardsPartialResults', async function logSplitTerminationStopsAtOneBlockAndDiscardsPartialResults() {
  const single = await startLogStore('single');
  try {
    await expect(getLogs(single.client, filter, BigInt(7), BigInt(7))).rejects.toBeInstanceOf(IncompleteLogScanError);
    expect(single.requests.map((query) => [ query.fromBlock, query.toBlock ])).toEqual([ [ '0x7', '0x7' ] ]);
  } finally {
    await single.stop();
  }
  const partial = await startLogStore('partial');
  try {
    await expect(getLogs(partial.client, filter, BigInt(0), BigInt(3))).rejects.toThrow('Incomplete log scan');
    expect(partial.requests.map((query) => [ query.fromBlock, query.toBlock ])).toEqual([
      [ '0x0', '0x3' ], [ '0x0', '0x1' ], [ '0x2', '0x3' ], [ '0x2', '0x2' ],
    ]);
    expect(partial.maxActive).toBe(1);
  } finally {
    await partial.stop();
  }
});

test('logSplitTerminationBoundsTotalRequestsBeforeAnIndivisibleRange', async function logSplitTerminationBoundsTotalRequestsBeforeAnIndivisibleRange() {
  const store = await startLogStore('budget');
  try {
    await expect(getLogs(store.client, filter, BigInt(0), BigInt(1) << BigInt(2048))).rejects.toMatchObject({
      name: 'IncompleteLogScanError', cause: expect.objectContaining({ message: 'Log scan request budget exhausted' }),
    });
    expect(store.requests.length).toBeGreaterThan(1);
    expect(store.requests).toHaveLength(1024);
    expect(store.requests.every((query) => BigInt(query.toBlock) > BigInt(query.fromBlock))).toBe(true);
    expect(store.maxActive).toBe(1);
  } finally {
    await store.stop();
  }
});

test('logSplitTerminationCancelsTheInflightHttpRequest', async function logSplitTerminationCancelsTheInflightHttpRequest() {
  const store = await startLogStore('hang');
  const controller = new AbortController();
  try {
    const scan = getLogs(store.client, filter, BigInt(0), BigInt(15), controller.signal);
    await expect(Promise.all([ scan, store.received.then(() => controller.abort()) ]))
      .rejects.toMatchObject({ name: 'AbortError' });
    await store.closed;
    expect(store.disconnected).toBe(1);
    expect(store.requests).toHaveLength(1);
    await expect(getLogs(store.client, filter, BigInt(0), BigInt(15), controller.signal)).rejects.toMatchObject({ name: 'AbortError' });
    expect(store.requests).toHaveLength(1);
  } finally {
    await store.stop();
  }
});
