import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import React, { act, useState } from 'react';
import { afterAll, expect, test } from 'vitest';
import type { LiveDirectory, LiveDirectoryContinuation } from './use-live-directory';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const poolAddress = `0x${'1'.repeat(40)}`;
const creatorWallet = `0x${'2'.repeat(40)}`;
const streamId = (n: number) => `11111111-1111-4111-8111-${String(n).padStart(12, '0')}`;
const boundary = 'continuation-1';
let generation = 1;
let failBase = false;
const requests: string[] = [];
const unexpected: string[] = [];

function stream(n: number, viewerCount: number, status: 'live' | 'ended' = 'live') {
  return {
    streamId: streamId(n), poolAddress, creatorWallet, title: `Broadcast ${n}`,
    revision: String(generation), generatedAt: '2026-10-12T15:00:00.000Z', status,
    playbackUrl: status === 'live' ? `https://livepeercdn.studio/hls/stream${n}/index.m3u8` : null,
    viewerCount, viewerQuality: 'verified', thumbnailUrl: null,
  };
}

// Committed in-memory directory fixture, served over HTTP through the production codec.
const server = createServer((request, response) => {
  request.resume();
  const url = new URL(request.url ?? '/', 'http://127.0.0.1');
  response.setHeader('Content-Type', 'application/json');
  if (url.pathname === '/v2/streams/live') {
    const cursor = url.searchParams.get('cursor');
    requests.push(cursor ?? 'base');
    if (failBase && cursor === null) {
      response.writeHead(501);
      response.end(JSON.stringify({ code: 'NOT_IMPLEMENTED', retryable: false }));
      return;
    }
    if (cursor === null || /^base-[1-4]$/.test(cursor)) {
      const page = cursor === null ? 0 : Number(cursor.slice(5));
      response.end(JSON.stringify({ streams: [], nextCursor: page === 4 ? boundary : `base-${page + 1}` }));
      return;
    }
    if (cursor === boundary) {
      response.end(JSON.stringify({ streams: [stream(1, 12, generation === 1 ? 'live' : 'ended')], nextCursor: generation === 1 ? 'continuation-2' : 'continuation-replacement' }));
      return;
    }
    if (cursor === 'continuation-2') {
      response.end(JSON.stringify({ streams: [stream(2, 22)], nextCursor: null }));
      return;
    }
    if (cursor === 'continuation-replacement') {
      response.end(JSON.stringify({ streams: [stream(2, 99), stream(3, 33)], nextCursor: null }));
      return;
    }
  }
  // This directory fixture has no pool registry or statistics projection.
  if (url.pathname === `/v2/pools/${poolAddress}` || url.pathname === '/v2/pools/stats:batch') {
    response.writeHead(501);
    response.end('{}');
    return;
  }
  unexpected.push(url.pathname + url.search);
  response.writeHead(404);
  response.end('{}');
});
await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
const previousLivestreamUrl = process.env.NEXT_PUBLIC_LIVESTREAM_API_URL;
process.env.NEXT_PUBLIC_LIVESTREAM_API_URL = origin;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { SdkClient } = await import('@kindle/sdk');
const { MarketClientContext } = await import('@/core/market/client');
const { useLiveDirectory, useLiveDirectoryContinuation } = await import('./use-live-directory');
const { queryKeys } = await import('@/core/query-keys');

afterAll(async () => {
  if (previousLivestreamUrl === undefined) delete process.env.NEXT_PUBLIC_LIVESTREAM_API_URL;
  else process.env.NEXT_PUBLIC_LIVESTREAM_API_URL = previousLivestreamUrl;
  await new Promise<void>(resolve => server.close(() => resolve()));
  dom.window.close();
});

async function waitFor(check: () => boolean): Promise<void> {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() >= deadline) throw new Error('Directory refresh did not settle');
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}

async function refreshScenario(automatic: boolean): Promise<void> {
  generation = 1;
  failBase = false;
  requests.length = 0;
  unexpected.length = 0;
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: origin.replace('http', 'ws'), approvedStreamOrigins: [origin.replace('http', 'ws')], production: false });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  interface Snapshot { directory: LiveDirectory; more: LiveDirectoryContinuation; activate(): void }
  let latest: Snapshot | null = null;
  function current(): Snapshot {
    if (latest === null) throw new Error('Directory is not mounted');
    return latest;
  }
  function DirectoryConsumer() {
    const directory = useLiveDirectory({ refetchInterval: automatic ? 1000 : false });
    const [start, setStart] = useState<string | null>(null);
    const more = useLiveDirectoryContinuation(start);
    latest = { directory, more, activate: () => {
      if (directory.state.state === 'ready') setStart(directory.state.data.nextCursor);
    } };
    return React.createElement('output', null, more.entries.map(entry => `${entry.stream.title}:${entry.viewerCount}`).join(','));
  }
  try {
    act(() => root.render(React.createElement(QueryClientProvider, { client: queryClient },
      React.createElement(MarketClientContext.Provider, { value: client }, React.createElement(DirectoryConsumer)))));
    await waitFor(() => current().directory.state.state === 'ready');
    expect(requests.slice(0, 5)).toEqual(['base', 'base-1', 'base-2', 'base-3', 'base-4']);
    expect(current().more.entries).toEqual([]);
    expect(requests).not.toContain(boundary);
    act(() => current().activate());
    await waitFor(() => current().more.entries.length === 1 && !current().more.loading);
    expect(current().more.hasMore).toBe(true);
    act(() => current().more.loadMore());
    await waitFor(() => current().more.entries.length === 2 && !current().more.loading);
    expect(container.textContent).toBe('Broadcast 1:12,Broadcast 2:22');
    expect(current().more.hasMore).toBe(false);
    expect(current().more.error).toBeNull();
    const firstBoundaryRequests = requests.filter(cursor => cursor === boundary).length;

    // A failed base refresh must not claim a new compatible directory snapshot.
    if (!automatic) {
      failBase = true;
      act(() => current().directory.refresh());
      await waitFor(() => current().directory.state.state === 'unsupported');
      expect(requests.filter(cursor => cursor === boundary)).toHaveLength(firstBoundaryRequests);
      expect(container.textContent).toBe('Broadcast 1:12,Broadcast 2:22');
      failBase = false;
    }

    generation = 2;
    if (!automatic) act(() => current().directory.refresh());
    await waitFor(() => current().more.entries.some(entry => entry.stream.id === streamId(3)) && !current().more.loading);
    const state = current().directory.state;
    expect(state.state).toBe('ready');
    if (state.state !== 'ready') throw new Error('Base directory did not refresh');
    expect(state.data.nextCursor).toBe(boundary);
    expect(requests.filter(cursor => cursor === boundary).length).toBeGreaterThan(firstBoundaryRequests);
    expect(requests).toContain('continuation-replacement');
    expect(current().more.entries.map(entry => entry.stream.id)).toEqual([streamId(2), streamId(3)]);
    expect(current().more.entries.map(entry => entry.stream.revision)).toEqual(['2', '2']);
    expect(container.textContent).toBe('Broadcast 2:99,Broadcast 3:33');
    expect(current().more.hasMore).toBe(false);
    expect(current().more.error).toBeNull();
    expect(queryClient.getQueryState(queryKeys.liveDirectoryContinuation(boundary))?.isInvalidated).toBe(false);
    expect(unexpected).toEqual([]);
  } finally {
    act(() => root.unmount());
    await queryClient.cancelQueries();
    queryClient.clear();
    container.remove();
  }
}

async function live_continuation_refresh_manual_removes_ended_stream_with_unchanged_boundary(): Promise<void> {
  await refreshScenario(false);
}

async function live_continuation_refresh_poll_rebuilds_loaded_pages_with_unchanged_boundary(): Promise<void> {
  await refreshScenario(true);
}

test(live_continuation_refresh_manual_removes_ended_stream_with_unchanged_boundary.name, live_continuation_refresh_manual_removes_ended_stream_with_unchanged_boundary);
test(live_continuation_refresh_poll_rebuilds_loaded_pages_with_unchanged_boundary.name, live_continuation_refresh_poll_rebuilds_loaded_pages_with_unchanged_boundary);
