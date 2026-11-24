// @vitest-environment jsdom

import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query';
import { act, cleanup, render, renderHook, waitFor } from '@testing-library/react';
import fetch from 'node-fetch';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import type { Server } from 'node:http';
import { createServer } from 'node:http';
import { resolve } from 'node:path';
import type { Channel } from 'phoenix';
import React from 'react';
import { array, looseObject, nullable, object, parse, string, tuple, unknown } from 'valibot';
import { WebSocket, WebSocketServer } from 'ws';

import type { SocketMessage } from './types';

import { afterEach, expect, test, vi } from 'vitest';

import { SocketProvider, useSocket } from './context';
import useSocketBuffer, { SOCKET_BUFFER_LIMIT, SOCKET_FLUSH_INTERVAL_MS, useSocketCountBuffer } from './useSocketBuffer';
import useSocketChannel from './useSocketChannel';
import useSocketMessage from './useSocketMessage';

const ROOT = resolve(__dirname, '../../../..');
const TOPIC = 'transactions:new_transaction';
const ADDRESS_TOPIC = 'addresses:0x1234567890123456789012345678901234567890';
const SOCKET_PATH = '/explorer/backend/socket/v2/websocket';
const PAGE_PATH = '/explorer/backend/api/v2/transactions';
const FIRST_PAGE_KEY = [ 'socket-repair', 'first-page' ];
const OPTIONS = { reconnectAfterMs: () => 20, rejoinAfterMs: () => 20 };
const rowSchema = looseObject({ hash: string() });
const pageSchema = object({ items: array(rowSchema) });
const frameSchema = tuple([ nullable(string()), nullable(string()), string(), string(), unknown() ]);
const recording = parse(object({ response: object({ body: string() }) }), JSON.parse(readFileSync(resolve(ROOT,
  'services/explorer/services/libs/blockscout-client/crate/tests/recorded/eth_blockscout_com/_api_v2_transactions.json',
), 'utf8')));
const recordedRows = parse(pageSchema, JSON.parse(recording.response.body)).items;
type Row = typeof recordedRows[number];

vi.setConfig({ testTimeout: 15_000 });
afterEach(cleanup);

// This peer replays committed API rows and the notifier's committed count conformance case.
// It is a Phoenix protocol fixture, not an execution of the Elixir notifier or Rust gateway.
class SocketPeer {
  server?: Server;
  transport?: WebSocketServer;
  port = 0;
  page: Array<Row> = recordedRows.slice(0, 3);
  pageReads = 0;
  joins = 0;
  leaves = 0;
  targets: Array<string> = [];
  channels = new Map<WebSocket, Map<string, string | null>>();

  get url() {
    return `ws://127.0.0.1:${ this.port }/explorer/backend/socket/v2`;
  }

  get pageUrl() {
    return `http://127.0.0.1:${ this.port }${ PAGE_PATH }`;
  }

  async open() {
    const server = createServer((request, response) => {
      this.targets.push(request.url || '');
      if (request.method !== 'GET' || request.url !== PAGE_PATH) {
        response.writeHead(404).end();
        return;
      }
      this.pageReads++;
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ items: this.page, next_page_params: null }));
    });
    const transport = new WebSocketServer({ server, path: SOCKET_PATH });
    transport.on('connection', (socket, request) => {
      this.targets.push(request.url || '');
      const topics = new Map<string, string | null>();
      this.channels.set(socket, topics);
      socket.on('close', () => this.channels.delete(socket));
      socket.on('message', (raw) => {
        const [ joinRef, ref, topic, event ] = parse(frameSchema, JSON.parse(raw.toString()));
        if (event === 'phx_join') {
          topics.set(topic, joinRef);
          this.joins++;
        } else if (event === 'phx_leave') {
          topics.delete(topic);
          this.leaves++;
        } else if (event !== 'heartbeat') {
          throw new Error(`Unexpected Phoenix fixture event: ${ event }`);
        }
        socket.send(JSON.stringify([ joinRef, ref, topic, 'phx_reply', { response: {}, status: 'ok' } ]));
      });
    });
    server.listen(this.port, '127.0.0.1');
    await once(server, 'listening');
    const address = server.address();
    if (!address || typeof address === 'string') {
      throw new Error('Socket fixture requires a loopback TCP address');
    }
    this.port = address.port;
    this.server = server;
    this.transport = transport;
  }

  publish(topic: string, payload: object) {
    let deliveries = 0;
    for (const [ socket, topics ] of this.channels) {
      if (topics.has(topic) && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify([ topics.get(topic), null, topic, 'transaction', payload ]));
        deliveries++;
      }
    }
    if (deliveries === 0) {
      throw new Error(`No joined subscriber for ${ topic }`);
    }
  }

  async stop() {
    const { transport, server } = this;
    if (!transport || !server) {
      throw new Error('Socket fixture is not open');
    }
    this.channels.clear();
    transport.clients.forEach((socket) => socket.terminate());
    await Promise.all([
      new Promise<void>((done, reject) => transport.close((error) => error ? reject(error) : done())),
      new Promise<void>((done, reject) => {
        server.close((error) => error ? reject(error) : done());
        server.closeAllConnections();
      }),
    ]);
    this.server = undefined;
    this.transport = undefined;
  }
}

function setVisibility(state: 'hidden' | 'visible') {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
  document.dispatchEvent(new Event('visibilitychange'));
}

test('socketBufferBoundsTheEntireHiddenBurst', async function socketBufferBoundsTheEntireHiddenBurst() {
  const flushes: Array<{ items: Array<number>; overflowed: boolean }> = [];
  const mounted = renderHook(() => useSocketBuffer<number>({
    onFlush: (items, status) => flushes.push({ items, overflowed: status.overflowed }),
    intervalMs: 20,
  }));
  try {
    expect(SOCKET_FLUSH_INTERVAL_MS).toBe(2000);
    act(() => setVisibility('hidden'));
    expect(mounted.result.current.isHeld).toBe(true);
    for (let index = 0; index < 100_000; index++) {
      mounted.result.current.push(index);
      expect(mounted.result.current.size).toBeLessThanOrEqual(SOCKET_BUFFER_LIMIT);
    }
    expect(mounted.result.current.size).toBe(SOCKET_BUFFER_LIMIT);
    expect(mounted.result.current.overflowed).toBe(true);
    expect(flushes).toEqual([]);
    act(() => setVisibility('visible'));
    await waitFor(() => expect(flushes).toHaveLength(1));
    expect(flushes[0].items).toEqual(Array.from({ length: SOCKET_BUFFER_LIMIT }, (_, index) => 100_000 - SOCKET_BUFFER_LIMIT + index));
    expect(flushes[0].overflowed).toBe(true);
    expect(mounted.result.current.size).toBe(0);
    expect(mounted.result.current.overflowed).toBe(false);
  } finally {
    mounted.unmount();
    setVisibility('visible');
  }
});

test('socketCountBufferAccumulatesWithoutGrowingWhileHeld', async function socketCountBufferAccumulatesWithoutGrowingWhileHeld() {
  const flushes: Array<number> = [];
  const mounted = renderHook(({ paused }) => useSocketCountBuffer({
    onFlush: (count) => flushes.push(count), isPaused: paused, intervalMs: 20,
  }), { initialProps: { paused: true } });
  act(() => mounted.result.current.hoverProps.onMouseEnter());
  for (let count = 1; count <= 100_000; count++) {
    mounted.result.current.push(count);
    expect(mounted.result.current.size).toBe(1);
  }
  expect(mounted.result.current.overflowed).toBe(false);
  expect(flushes).toEqual([]);
  mounted.rerender({ paused: false });
  expect(mounted.result.current.isHeld).toBe(true);
  act(() => mounted.result.current.hoverProps.onMouseLeave());
  await waitFor(() => expect(flushes).toEqual([ 5_000_050_000 ]));
  expect(mounted.result.current.size).toBe(0);
  mounted.result.current.push(7);
  await waitFor(() => expect(flushes).toEqual([ 5_000_050_000, 7 ]));
  for (const invalid of [ NaN, Infinity, -1, 1.5, Number.MAX_SAFE_INTEGER + 1 ]) {
    expect(() => mounted.result.current.push(invalid)).toThrow(RangeError);
  }
  mounted.result.current.push(Number.MAX_SAFE_INTEGER);
  expect(() => mounted.result.current.push(1)).toThrow(RangeError);
  expect(mounted.result.current.size).toBe(1);
});

test('socketBufferHonorsReducedLimitsAndCurrentRecoveryCallbacks', async function socketBufferHonorsReducedLimitsAndCurrentRecoveryCallbacks() {
  const flushes: Array<Array<number>> = [];
  const recoveries: Array<string> = [];
  const mounted = renderHook(({ limit, marker, paused }) => useSocketBuffer<number>({
    limit, isPaused: paused, intervalMs: 20,
    onFlush: (items) => flushes.push(items),
    onOverflow: () => {
      recoveries.push(marker);
    },
  }), { initialProps: { limit: 5, marker: 'old', paused: true } });
  for (let item = 0; item < 5; item++) {
    mounted.result.current.push(item);
  }
  mounted.rerender({ limit: 2, marker: 'current', paused: true });
  expect(mounted.result.current.size).toBe(2);
  expect(mounted.result.current.overflowed).toBe(true);
  expect(recoveries).toEqual([]);
  mounted.rerender({ limit: 2, marker: 'current', paused: false });
  await waitFor(() => expect(recoveries).toEqual([ 'current' ]));
  expect(flushes).toEqual([]);
  mounted.result.current.push(8);
  await waitFor(() => expect(flushes).toEqual([ [ 8 ] ]));
});

test('socketRowsRefetchTheFirstPageAfterAPausedOverflow', async function socketRowsRefetchTheFirstPageAfterAPausedOverflow() {
  const peer = new SocketPeer();
  await peer.open();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } } });
  let received = 0;
  let flushed = 0;
  const wrapper = ({ children }: { children: React.ReactNode }) => React.createElement(QueryClientProvider, {
    client,
    children: React.createElement(SocketProvider, { url: peer.url, options: OPTIONS, children }),
  });
  const mounted = renderHook(({ paused }) => {
    const queryClient = useQueryClient();
    const page = useQuery({
      queryKey: FIRST_PAGE_KEY,
      queryFn: async() => parse(pageSchema, await (await fetch(peer.pageUrl)).json()).items,
    });
    const buffer = useSocketBuffer<Row>({
      isPaused: paused, limit: 3, intervalMs: 20,
      onOverflow: () => {
        void queryClient.invalidateQueries({ queryKey: FIRST_PAGE_KEY, exact: true });
      },
      onFlush: (rows) => {
        flushed++;
        queryClient.setQueryData<Array<Row>>(FIRST_PAGE_KEY, (previous) => [ ...rows, ...(previous || []) ].slice(0, 3));
      },
    });
    const channel = useSocketChannel({ topic: ADDRESS_TOPIC, isDisabled: false });
    const handleTransaction: SocketMessage.AddressTxs['handler'] = (payload) => {
      payload.transactions.forEach(buffer.push);
      received += payload.transactions.length;
    };
    useSocketMessage({ channel, event: 'transaction', handler: handleTransaction });
    return { page, buffer, channel };
  }, { wrapper, initialProps: { paused: true } });
  try {
    await waitFor(() => expect(mounted.result.current.channel?.state).toBe('joined'));
    await waitFor(() => expect(mounted.result.current.page.data).toEqual(recordedRows.slice(0, 3)));
    peer.page = recordedRows.slice(8, 11);
    for (let index = 0; index < 1000; index++) {
      peer.publish(ADDRESS_TOPIC, { transactions: recordedRows.slice(index % 40, index % 40 + 1) });
    }
    await waitFor(() => expect(received).toBe(1000));
    expect(mounted.result.current.buffer.size).toBe(3);
    expect(mounted.result.current.buffer.overflowed).toBe(true);
    expect(peer.pageReads).toBe(1);
    expect(flushed).toBe(0);
    mounted.rerender({ paused: false });
    await waitFor(() => expect(mounted.result.current.page.data).toEqual(recordedRows.slice(8, 11)));
    expect(peer.pageReads).toBe(2);
    expect(flushed).toBe(0);
    expect(mounted.result.current.buffer.size).toBe(0);
    peer.publish(ADDRESS_TOPIC, { transactions: recordedRows.slice(0, 1) });
    await waitFor(() => expect(flushed).toBe(1));
    expect(mounted.result.current.page.data?.map((row) => row.hash)).toEqual([
      recordedRows[0].hash, recordedRows[8].hash, recordedRows[9].hash,
    ]);
    expect(peer.targets).toContain(`${ SOCKET_PATH }?vsn=2.0.0`);
    expect(peer.targets.filter((path) => path === PAGE_PATH)).toHaveLength(2);
  } finally {
    mounted.unmount();
    client.clear();
    await peer.stop();
  }
});

test('sharedChannelReleasesOneOwnerAndRejoinsEveryRemainingSubscriber', async function sharedChannelReleasesOneOwnerAndRejoinsEveryRemainingSubscriber() {
  const peer = new SocketPeer();
  await peer.open();
  const seen = {
    first: { channel: undefined as Channel | undefined, counts: [] as Array<number>, joins: 0 },
    second: { channel: undefined as Channel | undefined, counts: [] as Array<number>, joins: 0 },
  };
  const context: { current: ReturnType<typeof useSocket> } = { current: null };
  function Subscriber({ id, disabled }: { id: keyof typeof seen; disabled: boolean }) {
    const state = seen[id];
    context.current = useSocket();
    const channel = useSocketChannel({
      topic: ADDRESS_TOPIC, isDisabled: disabled, onJoin: () => {
        state.joins++;
      },
    });
    state.channel = channel;
    const handleTransaction: SocketMessage.AddressTxs['handler'] = (payload) => state.counts.push(payload.transactions.length);
    useSocketMessage({ channel, event: 'transaction', handler: handleTransaction });
    return null;
  }
  function view(firstDisabled: boolean, secondDisabled: boolean) {
    return React.createElement(SocketProvider, { url: peer.url, options: OPTIONS, children: [
      React.createElement(Subscriber, { key: 'first', id: 'first', disabled: firstDisabled }),
      React.createElement(Subscriber, { key: 'second', id: 'second', disabled: secondDisabled }),
    ] });
  }
  const mounted = render(view(false, false));
  try {
    await waitFor(() => expect(seen.first.channel?.state).toBe('joined'));
    await waitFor(() => expect(seen.second.joins).toBe(1));
    expect(seen.first.channel).toBe(seen.second.channel);
    expect(context.current?.channelRegistry.current[ADDRESS_TOPIC].subscribers).toBe(2);
    expect(peer.joins).toBe(1);
    peer.publish(ADDRESS_TOPIC, { transactions: recordedRows.slice(0, 1) });
    await waitFor(() => expect(seen.first.counts).toEqual([ 1 ]));
    await waitFor(() => expect(seen.second.counts).toEqual([ 1 ]));
    mounted.rerender(view(true, false));
    await waitFor(() => expect(seen.first.channel).toBeUndefined());
    expect(context.current?.channelRegistry.current[ADDRESS_TOPIC].subscribers).toBe(1);
    expect(seen.second.channel?.state).toBe('joined');
    expect(peer.leaves).toBe(0);
    peer.publish(ADDRESS_TOPIC, { transactions: recordedRows.slice(0, 2) });
    await waitFor(() => expect(seen.second.counts).toEqual([ 1, 2 ]));
    expect(seen.first.counts).toEqual([ 1 ]);
    await peer.stop();
    await peer.open();
    await waitFor(() => expect(seen.second.joins).toBe(2));
    expect(peer.joins).toBe(2);
    expect(seen.first.joins).toBe(1);
    peer.publish(ADDRESS_TOPIC, { transactions: recordedRows.slice(0, 3) });
    await waitFor(() => expect(seen.second.counts).toEqual([ 1, 2, 3 ]));
    mounted.rerender(view(false, false));
    await waitFor(() => expect(seen.first.joins).toBe(2));
    expect(peer.joins).toBe(2);
    await peer.stop();
    await peer.open();
    await waitFor(() => expect(seen.first.joins).toBe(3));
    await waitFor(() => expect(seen.second.joins).toBe(3));
    peer.publish(ADDRESS_TOPIC, { transactions: recordedRows.slice(0, 4) });
    await waitFor(() => expect(seen.first.counts).toEqual([ 1, 4 ]));
    await waitFor(() => expect(seen.second.counts).toEqual([ 1, 2, 3, 4 ]));
    mounted.rerender(view(true, false));
    expect(peer.leaves).toBe(0);
    mounted.rerender(view(true, true));
    await waitFor(() => expect(peer.leaves).toBe(1));
    expect(context.current?.channelRegistry.current[ADDRESS_TOPIC]).toBeUndefined();
  } finally {
    mounted.unmount();
    await peer.stop();
  }
});

test('socketStreamReconcilesMissedRowsAfterPeerAndConsumerRestart', async function socketStreamReconcilesMissedRowsAfterPeerAndConsumerRestart() {
  const peer = new SocketPeer();
  await peer.open();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } } });
  const wrapper = ({ children }: { children: React.ReactNode }) => React.createElement(QueryClientProvider, {
    client, children: React.createElement(SocketProvider, { url: peer.url, options: OPTIONS, children }),
  });
  let total = 0;
  let delivered = 0;
  function useConsumer() {
    const queryClient = useQueryClient();
    const page = useQuery({
      queryKey: FIRST_PAGE_KEY,
      queryFn: async() => parse(pageSchema, await (await fetch(peer.pageUrl)).json()).items,
    });
    const buffer = useSocketCountBuffer({ onFlush: (count) => {
      total += count;
    }, intervalMs: 20 });
    const channel = useSocketChannel({
      topic: TOPIC, isDisabled: false,
      onJoin: () => {
        void queryClient.invalidateQueries({ queryKey: FIRST_PAGE_KEY, exact: true });
      },
    });
    const handleTransaction: SocketMessage.NewTx['handler'] = (payload) => {
      buffer.push(payload.transaction);
      delivered++;
    };
    useSocketMessage({ channel, event: 'transaction', handler: handleTransaction });
    return { page, channel, buffer };
  }
  let mounted = renderHook(useConsumer, { wrapper });
  try {
    await waitFor(() => expect(mounted.result.current.channel?.state).toBe('joined'));
    await waitFor(() => expect(mounted.result.current.page.data).toEqual(recordedRows.slice(0, 3)));
    act(() => mounted.result.current.buffer.hoverProps.onMouseEnter());
    for (let index = 0; index < 2000; index++) {
      peer.publish(TOPIC, { transaction: 1 });
    }
    await waitFor(() => expect(delivered).toBe(2000));
    expect(mounted.result.current.buffer.size).toBe(1);
    expect(total).toBe(0);
    act(() => mounted.result.current.buffer.hoverProps.onMouseLeave());
    await waitFor(() => expect(total).toBe(2000));
    await peer.stop();
    peer.page = recordedRows.slice(12, 15);
    await peer.open();
    await waitFor(() => expect(peer.joins).toBe(2));
    await waitFor(() => expect(mounted.result.current.page.data).toEqual(recordedRows.slice(12, 15)));
    peer.publish(TOPIC, { transaction: 1 });
    await waitFor(() => expect(total).toBe(2001));
    const readsBeforeReopen = peer.pageReads;
    mounted.unmount();
    await waitFor(() => expect(peer.channels.size).toBe(0));
    peer.page = recordedRows.slice(20, 23);
    mounted = renderHook(useConsumer, { wrapper });
    await waitFor(() => expect(peer.joins).toBe(3));
    await waitFor(() => expect(mounted.result.current.page.data).toEqual(recordedRows.slice(20, 23)));
    expect(peer.pageReads).toBeGreaterThan(readsBeforeReopen);
    peer.publish(TOPIC, { transaction: 1 });
    await waitFor(() => expect(total).toBe(2002));
    expect(mounted.result.current.buffer.size).toBe(0);
  } finally {
    mounted.unmount();
    client.clear();
    await peer.stop();
  }
});

test('socketReplayContractsMatchTheCommittedNotifierAndGateway', function socketReplayContractsMatchTheCommittedNotifierAndGateway() {
  const backend = 'services/explorer/backend/apps/block_scout_web';
  const notifier = readFileSync(resolve(ROOT, backend, 'lib/block_scout_web/notifier.ex'), 'utf8');
  const channel = readFileSync(resolve(ROOT, backend, 'lib/block_scout_web/channels/v2/transaction_channel.ex'), 'utf8');
  const producerCase = readFileSync(resolve(ROOT, backend, 'test/block_scout_web/channels/v2/transaction_channel_test.exs'), 'utf8');
  const gateway = readFileSync(resolve(ROOT, 'services/platform/hosted/gateway/src/explorer_proxy.rs'), 'utf8');
  expect(notifier).toContain('broadcast_transactions_websocket_v2_inner(validated_transactions, "transactions:new_transaction", "transaction")');
  expect(notifier).toContain('String.to_existing_atom(event) => Enum.count(transactions)');
  expect(channel).toContain(`def join("${ TOPIC }", _params, socket)`);
  expect(producerCase).toContain('Notifier.handle_event({:chain_event, :transactions, :realtime, [transaction]})');
  expect(producerCase).toContain('assert payload.transaction == 1');
  expect(gateway).toContain('const SOCKET_PATH: &str = "/socket/v2/websocket";');
  expect(gateway).toMatch(/Route\s*\{\s*method: "GET",\s*path: "\/socket\/v2\/websocket",\s*session_csrf: false,\s*\}/);
  expect(gateway).toContain('path.strip_prefix(PREFIX) != Some(SOCKET_PATH)');
  expect(gateway).toContain('Some(ws::serve_explorer(');
  expect(gateway).toContain('path: "/api/v2/transactions",');
  expect(recordedRows.length).toBeGreaterThanOrEqual(40);
});
