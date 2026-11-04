import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { SocketConnectOption } from 'phoenix';
import React from 'react';
import type { WebSocket as ServerWebSocket } from 'ws';
import WebSocket, { WebSocketServer } from 'ws';

import type { RawTracesResponse } from 'types/api/rawTrace';
import type { Transaction } from 'types/api/transaction';

import { afterAll, afterEach, expect } from 'vitest';

const nodeHash = `0x${ 'a5'.repeat(32) }`;
const blockHash = `0x${ 'b6'.repeat(32) }`;
const from = `0x${ 'c7'.repeat(20) }`;
const to = `0x${ 'd8'.repeat(20) }`;
const rpcTransaction = {
  hash: nodeHash, blockHash, blockNumber: '0x1234', from, to,
  gas: '0x186a0', gasPrice: '0x3b9aca00', input: '0x12345678', nonce: '0x6b5',
  transactionIndex: '0x2', value: '0xde0b6b3a7640000', type: '0x0', v: '0x25',
  r: `0x${ '1'.repeat(64) }`, s: `0x${ '2'.repeat(64) }`,
};
const rpcReceipt = {
  transactionHash: nodeHash, blockHash, blockNumber: '0x1234', from, to,
  transactionIndex: '0x2', contractAddress: null, cumulativeGasUsed: '0xd903',
  gasUsed: '0xd903', effectiveGasPrice: '0x77359400', logs: [],
  logsBloom: `0x${ '0'.repeat(512) }`, status: '0x1', type: '0x0',
};
const trace: RawTracesResponse = [ {
  action: {
    callType: 'call', from, to, gas: '0x8600', input: '0x12345678', value: '0xde0b6b3a7640000',
  },
  result: { gasUsed: '0x7d37', output: '0xfeedbeef' },
  error: null, subtraces: 0, traceAddress: [], type: 'call',
} ];

let receiptState: 'success' | 'reverted' | 'unavailable' = 'success';
let indexed: Transaction | undefined;
const rpcRequests: Array<{ method: string; params: Array<unknown> }> = [];
const apiResponses: Array<number> = [];
const traceRequests: Array<string> = [];
const channels = new Map<string, { socket: ServerWebSocket; joinRef: string }>();
const completions = new Map<string, RawTracesResponse>();
const frames: Array<unknown> = [];

const server = createServer(async(request, response) => {
  response.setHeader('content-type', 'application/json');
  if (request.url === '/rpc') {
    let body = '';
    for await (const chunk of request) {
      body += chunk;
    }
    const { id, method, params } = JSON.parse(body) as { id: number; method: string; params: Array<unknown> };
    rpcRequests.push({ method, params });
    let result: unknown;
    switch (method) {
      case 'eth_getTransactionByHash':
        result = params[0] === nodeHash ? rpcTransaction : null;
        break;
      case 'eth_getTransactionReceipt':
        if (receiptState === 'unavailable') {
          response.end(JSON.stringify({ jsonrpc: '2.0', id, error: { code: -32001, message: 'Receipt unavailable' } }));
          return;
        }
        result = { ...rpcReceipt, status: receiptState === 'success' ? '0x1' : '0x0' };
        break;
      case 'eth_getBlockByHash':
      case 'eth_getBlockByNumber':
        result = {
          hash: blockHash, parentHash: blockHash, number: method === 'eth_getBlockByHash' ? '0x1234' : '0x1238',
          timestamp: '0x6553f100', baseFeePerGas: '0x3b9aca00', gasLimit: '0x1c9c380', gasUsed: '0xd903',
          transactions: [], uncles: [], difficulty: '0x0', totalDifficulty: '0x0', size: '0x200',
          nonce: '0x0000000000000000', extraData: '0x', logsBloom: rpcReceipt.logsBloom,
          miner: from, receiptsRoot: blockHash, stateRoot: blockHash, transactionsRoot: blockHash,
          sha3Uncles: blockHash, mixHash: blockHash,
        };
        break;
      default:
        response.end(JSON.stringify({ jsonrpc: '2.0', id, error: { code: -32601, message: `Unsupported method ${ method }` } }));
        return;
    }
    response.end(JSON.stringify({ jsonrpc: '2.0', id, result }));
    return;
  }

  const path = request.url?.split('?')[0];
  if (path === `/api/v2/transactions/${ nodeHash }`) {
    const status = indexed ? 200 : 501;
    apiResponses.push(status);
    response.writeHead(status).end(JSON.stringify(indexed || { message: 'Indexer unavailable' }));
    return;
  }
  if (path === `/api/v2/transactions/${ nodeHash }/raw-trace`) {
    traceRequests.push(nodeHash);
    response.end(JSON.stringify(completions.get(nodeHash) || []));
    return;
  }
  response.writeHead(404).end(JSON.stringify({ message: 'Not found' }));
});
const sockets = new WebSocketServer({ server });
sockets.on('connection', (socket) => {
  socket.on('message', (data) => {
    const [ joinRef, ref, topic, event ] = JSON.parse(data.toString()) as [ string, string, string, string ];
    if (event === 'phx_join') {
      channels.set(topic, { socket, joinRef });
    }
    if ([ 'phx_join', 'heartbeat', 'phx_leave' ].includes(event)) {
      socket.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { status: 'ok', response: {} } ]));
    }
    if (event === 'phx_leave') {
      channels.delete(topic);
    }
  });
  socket.on('close', () => channels.forEach((channel, topic) => {
    if (channel.socket === socket) {
      channels.delete(topic);
    }
  }));
});
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
server.unref();
const { port } = server.address() as AddressInfo;
const previousEnvs = window.__envs;
window.__envs = {
  ...previousEnvs,
  NEXT_PUBLIC_NETWORK_RPC_URL: `http://127.0.0.1:${ port }/rpc`,
  NEXT_PUBLIC_API_PROTOCOL: 'http', NEXT_PUBLIC_API_HOST: '127.0.0.1', NEXT_PUBLIC_API_PORT: String(port),
};

const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
const { 'default': Router } = await import('next/dist/shared/lib/router/router');
const { 'default': PageLoader } = await import('next/dist/client/page-loader');
const { 'default': App } = await import('next/app');
const { Provider } = await import('toolkit/chakra/provider');
const { AppContextProvider } = await import('lib/contexts/app');
const { GrowthBookProvider } = await import('@growthbook/growthbook-react');
const { SocketProvider } = await import('lib/socket/context');
const { 'default': TransactionPage } = await import('ui/pages/Transaction');
const { 'default': useTxQuery } = await import('./useTxQuery');
export const { 'default': TxDetailsDegraded } = await import('./TxDetailsDegraded');
export const { 'default': TxRawTrace } = await import('./TxRawTrace');
const { getResourceKey } = await import('lib/api/useApiQuery');
const { base } = await import('mocks/txs/tx');
const { GET_TRANSACTION, GET_TRANSACTION_RECEIPT } = await import('stubs/RPC');
await import('ui/shared/layout/testWrapper');

const options: Partial<SocketConnectOption> = { transport: WebSocket };
const disposers: Array<() => void> = [];
const clients: Array<QueryClient> = [];

export function renderTransaction(ui: React.ReactElement, hash = nodeHash) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
  clients.push(client);
  window.__NEXT_DATA__ = { props: {}, page: '/tx/[hash]', query: { hash }, buildId: 'repair-22-33-519' };
  window.__DEV_MIDDLEWARE_MATCHERS = [];
  const router = new Router('/tx/[hash]', { hash }, `/tx/${ hash }`, {
    initialProps: {}, pageLoader: new PageLoader('repair-22-33-519', ''), App, Component: TransactionPage,
    wrapApp: (component) => component, subscription: async() => {}, isFallback: false,
  });
  disposers.push(() => window.removeEventListener('popstate', router.onPopState));
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(RouterContext.Provider, { value: router },
      React.createElement(QueryClientProvider, { client },
        React.createElement(AppContextProvider, {
          pageProps: { cookies: '', referrer: '', query: { hash }, adBannerProvider: null, apiData: null, uuid: 'repair-22-33-519' },
          children: React.createElement(GrowthBookProvider, {
            children: React.createElement(SocketProvider, {
              url: `ws://127.0.0.1:${ port }/socket`, options,
              children: React.createElement(Provider, { children }),
            }),
          }),
        })));
  return { ...render(ui, { wrapper }), client };
}

afterEach(() => {
  cleanup();
  disposers.splice(0).forEach((dispose) => dispose());
  clients.splice(0).forEach((client) => client.clear());
});
afterAll(async() => {
  window.__envs = previousEnvs;
  globalThis.fetchMock.enableMocks();
  sockets.clients.forEach((socket) => socket.terminate());
  await new Promise<void>((resolve, reject) => sockets.close((error) => error ? reject(error) : resolve()));
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

function resetProducer(state: typeof receiptState = 'success') {
  globalThis.fetchMock.disableMocks();
  receiptState = state;
  indexed = undefined;
  rpcRequests.length = 0;
  apiResponses.length = 0;
  traceRequests.length = 0;
  frames.length = 0;
  completions.clear();
}

function expandOverview(container: HTMLElement) {
  const toggle = container.querySelector<HTMLButtonElement>('[data-tx-info] [data-scan-expander] [data-toggle]');
  expect(toggle).not.toBeNull();
  if (!toggle) {
    throw new Error('Transaction details expander missing');
  }
  fireEvent.click(toggle);
}

async function assertNodeOverview(state: 'success' | 'reverted') {
  resetProducer(state);
  const page = renderTransaction(React.createElement(TransactionPage));
  try {
    await waitFor(() => {
      const overview = page.container.querySelector('[data-tx-info]');
      expect(page.client.getQueryData<Array<unknown>>([ 'RPC', 'tx', { hash: nodeHash } ])?.[0]).toMatchObject({ hash: nodeHash });
      expect(overview?.textContent).toContain(state === 'success' ? 'Success' : 'Failed');
    }, { timeout: 5000 });
    expandOverview(page.container);
    await waitFor(() => {
      expect(page.container.querySelector('[data-tx-info]')?.textContent).toContain('55,555');
      expect(page.container.querySelector('[data-tx-info]')?.textContent).toContain('0x12345678');
    });
    expect(nodeHash).not.toBe(GET_TRANSACTION.hash);
    expect(BigInt(rpcReceipt.gasUsed)).not.toBe(GET_TRANSACTION_RECEIPT.gasUsed);
    expect(apiResponses).toEqual([ 501 ]);
    expect(rpcRequests).toEqual(expect.arrayContaining([
      { method: 'eth_getTransactionByHash', params: [ nodeHash ] },
      { method: 'eth_getTransactionReceipt', params: [ nodeHash ] },
      { method: 'eth_getBlockByHash', params: [ blockHash, false ] },
      { method: 'eth_getBlockByNumber', params: [ 'latest', false ] },
    ]));
    const response = page.client.getQueryData<Array<unknown>>([ 'RPC', 'tx', { hash: nodeHash } ]);
    expect(response?.[2]).toBe(BigInt(5));
    const band = page.container.querySelector('[data-tx-info]')?.previousElementSibling;
    expect(band?.textContent).toContain('Data sync in progress');
    expect(band?.textContent).toContain('This is a testnet transaction only');
  } finally {
    page.unmount();
    globalThis.fetchMock.enableMocks();
  }
}

export async function degradedNodeReceiptReachesOverview() {
  await assertNodeOverview('success');
}

export async function degradedRevertedReceiptReachesOverview() {
  await assertNodeOverview('reverted');
}

export async function degradedReceiptErrorRecoversToIndexedTransaction() {
  resetProducer('unavailable');
  const page = renderTransaction(React.createElement(TransactionPage));
  try {
    await waitFor(() => {
      const overview = page.container.querySelector('[data-tx-info]');
      expect(overview?.textContent).toContain('Pending');
      expect(page.client.getQueryData([ 'RPC', 'tx', { hash: nodeHash } ])).toBeDefined();
      expect(overview?.textContent).not.toContain('Success');
    }, { timeout: 5000 });
    expandOverview(page.container);
    expect(page.container.querySelector('[data-tx-info]')?.textContent).toContain('0x12345678');
    expect(rpcRequests.some(({ method }) => method === 'eth_getTransactionReceipt')).toBe(true);
    const response = page.client.getQueryData<Array<unknown>>([ 'RPC', 'tx', { hash: nodeHash } ]);
    expect(response?.[1]).toBeNull();
    expect(page.container.textContent).toContain('Data sync in progress');
    indexed = { ...base, hash: nodeHash, raw_input: '0xdecafbad', nonce: 424242, gas_used: '65432', status: 'ok' };
    // Let the production degraded view's setRefetchEnabled handoff poll the recovered explorer API.
    await waitFor(() => {
      expect(page.container.querySelector('[data-tx-details]')).not.toBeNull();
      expect(page.container.textContent).not.toContain('Data sync in progress');
      expect(page.client.getQueryData(getResourceKey('general:tx', { pathParams: { hash: nodeHash } }))).toEqual(indexed);
    }, { timeout: 18000 });
    expandOverview(page.container);
    expect(page.container.querySelector('[data-tx-info]')?.textContent).toContain('0xdecafbad');
    expect(page.container.querySelector('[data-tx-info]')?.textContent).toContain('65,432');
    expect(apiResponses).toEqual([ 501, 200 ]);
  } finally {
    page.unmount();
    globalThis.fetchMock.enableMocks();
  }
}

function TracePanel() {
  return React.createElement(TxRawTrace, { txQuery: useTxQuery() });
}

function completeOnDemand(rawTrace: RawTracesResponse) {
  const topic = `transactions:${ nodeHash }`;
  const channel = channels.get(topic);
  if (!channel) {
    throw new Error(`No joined channel for ${ topic }`);
  }
  completions.set(nodeHash, rawTrace);
  const frame = [ channel.joinRef, null, topic, 'raw_trace', { raw_trace: rawTrace } ];
  frames.push(frame);
  channel.socket.send(JSON.stringify(frame));
}

export async function rawTraceOnDemandCompletionReachesPanel() {
  resetProducer();
  indexed = { ...base, hash: nodeHash };
  let panel = renderTransaction(React.createElement(TracePanel));
  const key = getResourceKey('general:tx_raw_trace', { pathParams: { hash: nodeHash } });
  try {
    await waitFor(() => {
      expect(traceRequests).toEqual([ nodeHash ]);
      expect(panel.container.textContent).toContain('No trace entries found.');
    });
    expect(channels.has(`transactions:${ nodeHash }`)).toBe(true);
    completeOnDemand(trace);
    await waitFor(() => {
      expect(panel.container.querySelector('[data-scan-table-card] [data-title]')?.textContent).toBe('Raw trace');
      expect(panel.container.querySelector('section')?.textContent).toContain(JSON.stringify(trace, undefined, 4));
      expect(panel.container.textContent).not.toContain('"raw_trace"');
    });
    expect(frames).toHaveLength(1);
    expect(panel.client.getQueryData(key)).toEqual([]);
    panel.unmount();
    await waitFor(() => expect(channels.has(`transactions:${ nodeHash }`)).toBe(false));
    panel = renderTransaction(React.createElement(TracePanel));
    await waitFor(() => {
      expect(traceRequests).toEqual([ nodeHash, nodeHash ]);
      expect(panel.client.getQueryData(key)).toEqual(trace);
      expect(panel.container.querySelector('section')?.textContent).toContain('0xfeedbeef');
    });
    completeOnDemand([]);
    await waitFor(() => expect(panel.container.textContent).toContain('No trace entries found.'));
    expect(panel.container.querySelector('[data-scan-table-card]')).toBeNull();
    expect(panel.client.getQueryData(key)).toEqual(trace);
    expect(frames).toHaveLength(2);
  } finally {
    panel.unmount();
    globalThis.fetchMock.enableMocks();
  }
}

export async function rawTraceEmptyCompletionReachesPanel() {
  resetProducer();
  completions.set(nodeHash, trace);
  indexed = { ...base, hash: nodeHash };
  const panel = renderTransaction(React.createElement(TracePanel));
  try {
    await waitFor(() => {
      expect(traceRequests).toEqual([ nodeHash ]);
      expect(panel.container.querySelector('section')?.textContent).toContain('0xfeedbeef');
    });
    completeOnDemand([]);
    await waitFor(() => expect(panel.container.textContent).toContain('No trace entries found.'));
    expect(panel.container.querySelector('[data-scan-table-card]')).toBeNull();
    expect(frames).toHaveLength(1);
    expect(completions.get(nodeHash)).toEqual([]);
  } finally {
    panel.unmount();
    globalThis.fetchMock.enableMocks();
  }
}
