// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, render, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { SocketConnectOption } from 'phoenix';
import React from 'react';
import type { WebSocket as ServerWebSocket } from 'ws';
import WebSocket, { WebSocketServer } from 'ws';

import type { RawTracesResponse } from 'types/api/rawTrace';
import type { Transaction } from 'types/api/transaction';

import { SocketProvider } from 'lib/socket/context';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';

import type { TxQuery } from './useTxQuery';

const server = createServer((request, response) => {
  const path = request.url?.split('/');
  const hash = path?.[4];
  const transaction = hash ? transactions.get(hash) : undefined;
  response.setHeader('content-type', 'application/json');
  if (path?.[1] !== 'api' || path[2] !== 'v2' || path[3] !== 'transactions' || !hash || !transaction) {
    response.writeHead(404).end(JSON.stringify({ message: 'Not found' }));
    return;
  }
  requests.set(hash, (requests.get(hash) || 0) + 1);
  response.end(JSON.stringify(path[5] === 'raw-trace' ? traces.get(hash) || [] : transaction));
});
const transactions = new Map<string, Transaction>();
const traces = new Map<string, RawTracesResponse>();
const requests = new Map<string, number>();
const sockets = new WebSocketServer({ server });
const channels = new Map<string, { socket: ServerWebSocket; joinRef: string }>();
let paused = false;
const joins: Array<() => void> = [];

sockets.on('connection', (socket) => {
  socket.on('message', (data) => {
    const [ joinRef, ref, topic, event ] = JSON.parse(data.toString()) as [ string, string, string, string ];
    if (event === 'phx_join') {
      const reply = () => {
        if (socket.readyState === WebSocket.OPEN) {
          channels.set(topic, { socket, joinRef });
          socket.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { status: 'ok', response: {} } ]));
        }
      };
      paused ? joins.push(reply) : reply();
    } else if (event === 'heartbeat' || event === 'phx_leave') {
      socket.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { status: 'ok', response: {} } ]));
      if (event === 'phx_leave' && channels.get(topic)?.socket === socket) {
        channels.delete(topic);
      }
    }
  });
  socket.on('close', () => {
    channels.forEach((channel, topic) => {
      if (channel.socket === socket) {
        channels.delete(topic);
      }
    });
  });
});

await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
server.unref();
const { port } = server.address() as AddressInfo;
const previousEnvs = window.__envs;
window.__envs = {
  ...previousEnvs,
  NEXT_PUBLIC_API_PROTOCOL: 'http',
  NEXT_PUBLIC_API_HOST: '127.0.0.1',
  NEXT_PUBLIC_API_PORT: String(port),
};

const { getResourceKey } = await import('lib/api/useApiQuery');
const { 'default': useTxQuery, useTxSocketStatus } = await import('./useTxQuery');
const { 'default': TxRawTrace } = await import('./TxRawTrace');
const { base: transactionFixture } = await import('mocks/txs/tx');
const { RouterContext } = await import('next/dist/shared/lib/router-context.shared-runtime');
const { 'default': Router } = await import('next/dist/shared/lib/router/router');
const { 'default': PageLoader } = await import('next/dist/client/page-loader');
const { 'default': App } = await import('next/app');
const { ChakraProvider } = await import('@chakra-ui/react');
const { 'default': theme } = await import('toolkit/theme/theme');
// Use the committed browser-host fixture while keeping the router, HTTP and socket modules real.
await import('ui/shared/layout/testWrapper');

const socketOptions: Partial<SocketConnectOption> = {
  transport: WebSocket,
  reconnectAfterMs: () => 10,
  rejoinAfterMs: () => 10,
};

beforeAll((): void => {
  fetchMock.disableMocks();
});
afterAll(async() => {
  window.__envs = previousEnvs;
  fetchMock.enableMocks();
  sockets.clients.forEach((socket) => socket.terminate());
  await new Promise<void>((resolve, reject) => sockets.close((error) => error ? reject(error) : resolve()));
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

function createWrapper(hash: string, client: QueryClient) {
  window.__NEXT_DATA__ = { props: {}, page: '/tx/[hash]', query: { hash }, buildId: 'repair-22-31-405' };
  window.__DEV_MIDDLEWARE_MATCHERS = [];
  const router = new Router('/tx/[hash]', { hash }, `/tx/${ hash }`, {
    initialProps: {},
    pageLoader: new PageLoader('repair-22-31-405', ''),
    App,
    Component: TransactionTracePanel,
    wrapApp: (component) => component,
    subscription: async() => {},
    isFallback: false,
  });
  const Wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(RouterContext.Provider, { value: router },
      React.createElement(QueryClientProvider, { client },
        React.createElement(SocketProvider, {
          url: `ws://127.0.0.1:${ port }/socket`,
          options: socketOptions,
          children: React.createElement(ChakraProvider, { value: theme, children }),
        })));

  return { wrapper: Wrapper, dispose: () => window.removeEventListener('popstate', router.onPopState) };
}

function TransactionTracePanel() {
  const txQuery = useTxQuery();
  return React.createElement(TxRawTrace, { txQuery });
}

function push(topic: string, event: string, payload: unknown) {
  const channel = channels.get(topic);
  if (!channel) {
    throw new Error(`No subscriber for ${ topic }`);
  }
  channel.socket.send(JSON.stringify([ channel.joinRef, null, topic, event, payload ]));
}

function seedPending(hash: string) {
  const pending: Transaction = { ...transactionFixture, hash, status: null, block_number: null, timestamp: null };
  transactions.set(hash, pending);
  return pending;
}

function commit(hash: string) {
  const committed: Transaction = { ...transactionFixture, hash };
  transactions.set(hash, committed);
  return committed;
}

function createQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } });
}

async function waitForTransaction(result: { current: TxQuery }, hash: string) {
  await waitFor(() => {
    expect(result.current.isPlaceholderData).toBe(false);
    expect(result.current.isSuccess).toBe(true);
    expect(result.current.data?.hash).toBe(hash);
  });
}

afterEach(cleanup);

function pendingTransactionRejoinInvalidatesRestState() {
  const client = new QueryClient();
  const key = getResourceKey('general:tx', { pathParams: { hash: 'rejoining-transaction' } });
  const otherKey = getResourceKey('general:tx', { pathParams: { hash: 'other-transaction' } });
  const query = client.getQueryCache().build(client, { queryKey: key });
  const otherQuery = client.getQueryCache().build(client, { queryKey: otherKey });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client }, children);
  const { result, unmount } = renderHook(() => useTxSocketStatus('rejoining-transaction'), { wrapper });

  expect(query.state.isInvalidated).toBe(false);
  act(() => result.current.handleSocketJoin());
  expect(query.state.isInvalidated).toBe(true);
  expect(otherQuery.state.isInvalidated).toBe(false);

  act(() => result.current.handleSocketClose());
  expect(result.current.socketStatus).toBe('close');
  query.setState({ isInvalidated: false });
  act(() => result.current.handleSocketJoin());
  expect(result.current.socketStatus).toBeUndefined();
  expect(query.state.isInvalidated).toBe(true);

  act(() => result.current.handleSocketError());
  expect(result.current.socketStatus).toBe('error');
  query.setState({ isInvalidated: false });
  act(() => result.current.handleSocketJoin());
  expect(result.current.socketStatus).toBeUndefined();
  expect(query.state.isInvalidated).toBe(true);

  unmount();
  query.setState({ isInvalidated: false });
  const reopened = renderHook(() => useTxSocketStatus('rejoining-transaction'), { wrapper });
  act(() => reopened.result.current.handleSocketJoin());
  expect(query.state.isInvalidated).toBe(true);
  reopened.unmount();
  client.clear();
}

test(pendingTransactionRejoinInvalidatesRestState, pendingTransactionRejoinInvalidatesRestState);

function rejoinReconcilesTheCurrentTransactionHash() {
  const client = new QueryClient();
  const first = client.getQueryCache().build(client, {
    queryKey: getResourceKey('general:tx', { pathParams: { hash: 'first-transaction' } }),
  });
  const second = client.getQueryCache().build(client, {
    queryKey: getResourceKey('general:tx', { pathParams: { hash: 'second-transaction' } }),
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client }, children);
  const { result, rerender, unmount } = renderHook(({ hash }) => useTxSocketStatus(hash), {
    wrapper,
    initialProps: { hash: 'first-transaction' },
  });

  rerender({ hash: 'second-transaction' });
  act(() => result.current.handleSocketJoin());
  expect(second.state.isInvalidated).toBe(true);
  expect(first.state.isInvalidated).toBe(false);
  unmount();
  client.clear();
}

test(rejoinReconcilesTheCurrentTransactionHash, rejoinReconcilesTheCurrentTransactionHash);

async function rawTraceEnvelopeReachesPanelAndSurvivesReopen() {
  const hash = `0x${ '1'.repeat(64) }`;
  const topic = `transactions:${ hash }`;
  commit(hash);
  traces.set(hash, []);
  let client = createQueryClient();
  let session = createWrapper(hash, client);
  const completedTrace: RawTracesResponse = [ {
    action: {
      callType: 'call',
      from: '0xa931c862e662134b85e4dc4baf5c70cc9ba74db4',
      to: '0x1469b17ebf82fedf56f04109e5207bdc4554288c',
      gas: '0x8600',
      input: '0xb118e2db0000000000000000000000000000000000000000000000000000000000000008',
      value: '0x174876e800',
    },
    result: { gasUsed: '0x7d37', output: '0x' },
    error: null,
    subtraces: 0,
    traceAddress: [],
    type: 'call',
  } ];
  let panel = render(React.createElement(TransactionTracePanel), { wrapper: session.wrapper });

  try {
    await waitFor(() => {
      expect(channels.has(topic)).toBe(true);
      expect(panel.container.textContent).toContain('No trace entries found.');
    });
    const traceQueryKey = getResourceKey('general:tx_raw_trace', { pathParams: { hash } });
    expect(client.getQueryData(traceQueryKey)).toEqual([]);
    traces.set(hash, completedTrace);
    act(() => push(topic, 'raw_trace', { raw_trace: completedTrace }));
    const text = JSON.stringify(completedTrace, undefined, 4);
    await waitFor(() => {
      expect(panel.container.querySelector('section')?.textContent).toContain(text);
      expect(panel.container.textContent).not.toContain('"raw_trace"');
    });
    // Only the socket has completed the initial request; the REST cache still holds its empty response.
    expect(client.getQueryData(traceQueryKey)).toEqual([]);

    panel.unmount();
    session.dispose();
    client.clear();
    client = createQueryClient();
    session = createWrapper(hash, client);
    panel = render(React.createElement(TransactionTracePanel), { wrapper: session.wrapper });
    await waitFor(() => {
      expect(panel.container.querySelector('section')?.textContent).toContain(text);
      expect(client.getQueryData(traceQueryKey)).toEqual(completedTrace);
    });

    act(() => push(topic, 'raw_trace', { raw_trace: [] }));
    await waitFor(() => expect(panel.container.textContent).toContain('No trace entries found.'));
  } finally {
    panel.unmount();
    session.dispose();
    client.clear();
  }
}

test(rawTraceEnvelopeReachesPanelAndSurvivesReopen, rawTraceEnvelopeReachesPanelAndSurvivesReopen);

async function connectedPendingTransactionFetchesCommittedReceipt() {
  const hash = `0x${ '2'.repeat(64) }`;
  const topic = `transactions:${ hash }`;
  seedPending(hash);
  const client = createQueryClient();
  const { wrapper, dispose } = createWrapper(hash, client);
  const { result, unmount } = renderHook(() => useTxQuery({ hash }), { wrapper });

  try {
    await waitForTransaction(result, hash);
    await waitFor(() => expect(channels.has(topic)).toBe(true));
    expect(result.current.data?.status).toBeNull();
    await waitFor(() => expect(result.current.isFetching).toBe(false));
    const committed = commit(hash);
    const before = requests.get(hash) || 0;
    act(() => push(topic, 'collated', {}));
    await waitFor(() => {
      expect(result.current.data?.status).toBe(committed.status);
      expect(result.current.data?.block_number).toBe(committed.block_number);
      expect(requests.get(hash)).toBeGreaterThan(before);
    }, { timeout: 6500 });
  } finally {
    unmount();
    dispose();
    client.clear();
  }
}

test(connectedPendingTransactionFetchesCommittedReceipt, connectedPendingTransactionFetchesCommittedReceipt, 10000);

async function disconnectedPendingTransactionReconcilesReceiptOnRejoin() {
  const hash = `0x${ '3'.repeat(64) }`;
  const topic = `transactions:${ hash }`;
  seedPending(hash);
  const client = createQueryClient();
  const { wrapper, dispose } = createWrapper(hash, client);
  const { result, unmount } = renderHook(() => useTxQuery({ hash }), { wrapper });

  try {
    await waitForTransaction(result, hash);
    await waitFor(() => expect(channels.has(topic)).toBe(true));
    expect(result.current.data?.status).toBeNull();
    paused = true;
    act(() => sockets.clients.forEach((socket) => socket.terminate()));
    await waitFor(() => expect(result.current.socketStatus).toBe('close'));
    const committed = commit(hash);
    const before = requests.get(hash) || 0;
    paused = false;
    act(() => joins.splice(0).forEach((join) => join()));
    await waitFor(() => {
      expect(result.current.socketStatus).toBeUndefined();
      expect(result.current.data?.status).toBe(committed.status);
      expect(result.current.data?.block_number).toBe(committed.block_number);
      expect(requests.get(hash)).toBeGreaterThan(before);
    });
  } finally {
    paused = false;
    joins.splice(0).forEach((join) => join());
    unmount();
    dispose();
    client.clear();
  }
}

test(disconnectedPendingTransactionReconcilesReceiptOnRejoin, disconnectedPendingTransactionReconcilesReceiptOnRejoin);

async function reopenedPendingTransactionReconcilesCachedReceipt() {
  const hash = `0x${ '4'.repeat(64) }`;
  const topic = `transactions:${ hash }`;
  seedPending(hash);
  const client = createQueryClient();
  const { wrapper, dispose } = createWrapper(hash, client);
  let hook = renderHook(() => useTxQuery({ hash }), { wrapper });

  try {
    await waitForTransaction(hook.result, hash);
    await waitFor(() => expect(channels.has(topic)).toBe(true));
    expect(hook.result.current.data?.status).toBeNull();
    hook.unmount();
    const committed = commit(hash);
    const key = getResourceKey('general:tx', { pathParams: { hash } });
    expect(client.getQueryData<Transaction>(key)?.status).toBeNull();
    const before = requests.get(hash) || 0;
    hook = renderHook(() => useTxQuery({ hash }), { wrapper });
    await waitFor(() => {
      expect(hook.result.current.data?.status).toBe(committed.status);
      expect(hook.result.current.data?.block_number).toBe(committed.block_number);
      expect(requests.get(hash)).toBeGreaterThan(before);
    });
  } finally {
    hook.unmount();
    dispose();
    client.clear();
  }
}

test(reopenedPendingTransactionReconcilesCachedReceipt, reopenedPendingTransactionReconcilesCachedReceipt);
