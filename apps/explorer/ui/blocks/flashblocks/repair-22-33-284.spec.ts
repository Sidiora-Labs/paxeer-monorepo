// @vitest-environment jsdom

import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { Blob as NodeBlob } from 'node:buffer';
import type { AddressInfo } from 'node:net';
import { brotliCompressSync } from 'node:zlib';
import type { WebSocket as ServerSocket } from 'ws';
import { WebSocketServer } from 'ws';

import type { FlashblockItemApiMegaEth, FlashblockItemApiOptimism } from 'types/api/flashblocks';

import { afterEach, expect, test, vi } from 'vitest';

import megaEthFixture from './fixtures/megaeth.json';
import optimismFixture from './fixtures/optimism.json';
import { formatFlashblockItemOptimism } from './formatFlashblockItem';
import { parseSocketEventDataOptimism } from './parseSocketEventData';

const servers: Array<WebSocketServer> = [];
const originalEnvs = window.__envs;
const originalBlob = globalThis.Blob;

afterEach(async() => {
  cleanup();
  window.__envs = originalEnvs;
  globalThis.Blob = originalBlob;
  await Promise.all(servers.splice(0).map((server) => new Promise<void>((resolve, reject) => {
    server.clients.forEach((client) => client.terminate());
    server.close((error) => error ? reject(error) : resolve());
  })));
});

async function createFeed(type: 'optimism' | 'megaEth') {
  const server = new WebSocketServer({ host: '127.0.0.1', port: 0 });
  servers.push(server);
  const connections: Array<ServerSocket> = [];
  const subscriptions: Array<unknown> = [];
  let acknowledge = true;
  server.on('connection', (socket) => {
    connections.push(socket);
    socket.on('message', (bytes) => {
      const request: unknown = JSON.parse(bytes.toString());
      subscriptions.push(request);
      if (acknowledge) {
        socket.send(JSON.stringify({ jsonrpc: '2.0', id: 1, result: `subscription-${ connections.length }` }));
      }
    });
  });
  await new Promise<void>((resolve) => server.once('listening', resolve));
  const url = `ws://127.0.0.1:${ (server.address() as AddressInfo).port }`;
  window.__envs = {
    ...originalEnvs,
    NEXT_PUBLIC_FLASHBLOCKS_SOCKET_URL: type === 'optimism' ? url : '',
    NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_RPC: type === 'megaEth' ? url : '',
    NEXT_PUBLIC_MEGA_ETH_SOCKET_URL_METRICS: '',
  };
  vi.resetModules();
  const { 'default': useFlashblocksSocketData } = await import('./useFlashblocksSocketData');
  const hook = renderHook(useFlashblocksSocketData);
  await waitFor(() => expect(hook.result.current.status).toBe('connected'));

  return {
    ...hook,
    connections,
    subscriptions,
    holdAcknowledgement() {
      acknowledge = false;
    },
    optimism(index: number, encoding: 'text' | 'json-binary' | 'compressed' = 'text') {
      const item: FlashblockItemApiOptimism = { ...optimismFixture, index };
      const text = JSON.stringify(item);
      const frame = encoding === 'compressed' ? brotliCompressSync(new TextEncoder().encode(text)) : text;
      connections[connections.length - 1].send(frame, { binary: encoding !== 'text' });
    },
    megaEth(index: number) {
      const item: FlashblockItemApiMegaEth = { ...megaEthFixture, index: `0x${ index.toString(16) }` };
      connections[connections.length - 1].send(JSON.stringify({
        jsonrpc: '2.0', method: 'eth_subscription', params: { subscription: `subscription-${ connections.length }`, result: item },
      }));
    },
  };
}

test('flashblockTextAndBinaryFrames', async function flashblockTextAndBinaryFrames() {
  const feed = await createFeed('optimism');
  feed.optimism(1);
  feed.optimism(2, 'json-binary');
  feed.optimism(3, 'compressed');
  await waitFor(() => expect(feed.result.current.itemsNum).toBe(3));
  expect(feed.result.current.items.map((item) => item.index)).toEqual([ 3, 2, 1 ]);
  expect(feed.result.current.items.every((item) => item.block_number === 42 && item.gas_used === 42000)).toBe(true);
  expect(feed.result.current.txsNum).toBe(6);
  expect(feed.result.current.status).toBe('connected');
});

test('flashblockBlobFrames', async function flashblockBlobFrames() {
  // Use Node's real web Blob: jsdom's Blob does not implement the browser arrayBuffer API.
  Object.defineProperty(globalThis, 'Blob', { value: NodeBlob, configurable: true, writable: true });
  const text = JSON.stringify(optimismFixture);
  const blob = new Blob([ text ]);
  const compressed = new Blob([ new Uint8Array(brotliCompressSync(new TextEncoder().encode(text))) ]);
  const plainItem = await parseSocketEventDataOptimism(new MessageEvent('message', { data: blob }));
  const compressedItem = await parseSocketEventDataOptimism(new MessageEvent('message', { data: compressed }));
  expect(formatFlashblockItemOptimism(plainItem)).toEqual(formatFlashblockItemOptimism(compressedItem));
  expect(formatFlashblockItemOptimism(plainItem).transactions_count).toBe(2);
});

test('flashblockMalformedFrameReportsError', async function flashblockMalformedFrameReportsError() {
  const feed = await createFeed('optimism');
  feed.connections[0].send('{');
  await waitFor(() => expect(feed.result.current.status).toBe('error'));
  expect(feed.result.current.itemsNum).toBe(0);
  act(() => feed.result.current.resume());
  await waitFor(() => expect(feed.connections).toHaveLength(2));
  await waitFor(() => expect(feed.result.current.status).toBe('connected'));
  feed.connections[1].send(JSON.stringify({ ...optimismFixture, diff: { ...optimismFixture.diff, transactions: null } }));
  await waitFor(() => expect(feed.result.current.status).toBe('error'));
  expect(feed.result.current.itemsNum).toBe(0);
});

test('flashblockUnsupportedBinaryReportsError', async function flashblockUnsupportedBinaryReportsError() {
  const feed = await createFeed('optimism');
  feed.connections[0].send(Buffer.from([ 255, 0, 254, 0 ]));
  await waitFor(() => expect(feed.result.current.status).toBe('error'));
  expect(feed.result.current.items).toEqual([]);
});

test('flashblockTailFlushDuringSilence', async function flashblockTailFlushDuringSilence() {
  const feed = await createFeed('optimism');
  // Publish one batch, then send the final burst inside its next batching window.
  feed.optimism(1);
  await waitFor(() => expect(feed.result.current.itemsNum).toBe(1));
  const start = Date.now();
  feed.optimism(2, 'json-binary');
  feed.optimism(3, 'json-binary');
  feed.optimism(4, 'json-binary');
  await waitFor(() => expect(feed.result.current.itemsNum).toBe(4), { timeout: 250, interval: 5 });
  expect(Date.now() - start).toBeLessThan(250);
  expect(feed.result.current.items.map((item) => item.index)).toEqual([ 4, 3, 2, 1 ]);
  expect(feed.result.current.txsNum).toBe(8);
  expect(feed.result.current.initialTs).toBeTypeOf('number');

  act(() => feed.result.current.pause());
  feed.optimism(5, 'json-binary');
  await waitFor(() => expect(feed.result.current.newItemsNum).toBe(1), { timeout: 250, interval: 5 });
  expect(feed.result.current.itemsNum).toBe(5);
  expect(feed.result.current.txsNum).toBe(10);
  expect(feed.result.current.items.map((item) => item.index)).toEqual([ 4, 3, 2, 1 ]);
});

test('flashblockReconnectResubscribesAndRecovers', async function flashblockReconnectResubscribesAndRecovers() {
  const feed = await createFeed('megaEth');
  expect(feed.subscriptions).toEqual([ { jsonrpc: '2.0', method: 'eth_subscribe', params: [ 'miniBlocks' ], id: 1 } ]);
  feed.megaEth(1);
  await waitFor(() => expect(feed.result.current.itemsNum).toBe(1));
  feed.connections[0].close();
  await waitFor(() => expect(feed.result.current.status).toBe('disconnected'));
  feed.holdAcknowledgement();
  // This is the action Flashblocks.handleAlertLinkClick already invokes.
  act(() => feed.result.current.resume());
  await waitFor(() => expect(feed.subscriptions).toHaveLength(2));
  expect(feed.connections[0].readyState).toBe(3);
  expect(feed.result.current.status).toBe('initial');
  feed.connections[1].send(JSON.stringify({ jsonrpc: '2.0', id: 1, result: 'subscription-2' }));
  await waitFor(() => expect(feed.result.current.status).toBe('connected'));
  feed.megaEth(2);
  await waitFor(() => expect(feed.result.current.itemsNum).toBe(2));
  expect(feed.result.current.items.map((item) => item.index)).toEqual([ 2, 1 ]);
  expect(feed.result.current.txsNum).toBe(4);
  expect(feed.subscriptions[1]).toEqual(feed.subscriptions[0]);

  act(() => feed.result.current.reconnect());
  await waitFor(() => expect(feed.subscriptions).toHaveLength(3));
  await waitFor(() => expect(feed.connections[1].readyState).toBe(3));
  feed.connections[2].send(JSON.stringify({ jsonrpc: '2.0', id: 1, result: 'subscription-3' }));
  await waitFor(() => expect(feed.result.current.status).toBe('connected'));
  feed.megaEth(3);
  await waitFor(() => expect(feed.result.current.itemsNum).toBe(3));
  expect(feed.result.current.items.map((item) => item.index)).toEqual([ 3, 2, 1 ]);
  expect(feed.result.current.txsNum).toBe(6);
});

test('flashblockSubscriptionTimeoutIsBounded', async function flashblockSubscriptionTimeoutIsBounded() {
  const feed = await createFeed('megaEth');
  feed.holdAcknowledgement();
  act(() => feed.result.current.reconnect());
  await waitFor(() => expect(feed.subscriptions).toHaveLength(2));
  await waitFor(() => expect(feed.result.current.status).toBe('error'), { timeout: 6000, interval: 25 });
  await waitFor(() => expect(feed.connections[1].readyState).toBe(3));
  expect(feed.connections).toHaveLength(2);
  expect(feed.subscriptions).toHaveLength(2);
  expect(feed.result.current.itemsNum).toBe(0);
}, 10000);

test('flashblockReopenCancelsOldSubscription', async function flashblockReopenCancelsOldSubscription() {
  const feed = await createFeed('megaEth');
  feed.holdAcknowledgement();
  act(() => feed.result.current.reconnect());
  await waitFor(() => expect(feed.subscriptions).toHaveLength(2));
  const attempts = feed.subscriptions.length;
  feed.unmount();
  await waitFor(() => expect(feed.connections[1].readyState).toBe(3));
  const reopened = await createFeed('megaEth');
  reopened.megaEth(7);
  await waitFor(() => expect(reopened.result.current.itemsNum).toBe(1));
  expect(reopened.result.current.items[0].index).toBe(7);
  expect(reopened.result.current.txsNum).toBe(2);
  expect(feed.subscriptions).toHaveLength(attempts);
  expect(reopened.subscriptions).toHaveLength(1);
});
