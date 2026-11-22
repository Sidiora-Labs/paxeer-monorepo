import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { createServer, type Server } from 'node:http';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SdkClient, decodeV2Contract } from '@kindle/sdk';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { queryKeys } from '@/core/query-keys';
import type { StreamOperation } from '@/core/clients/livestream-api';
import { MarketClientContext } from '@/core/market/client';
import type { LiveDirectoryContinuation, LiveDirectoryData } from '@/widgets/live/use-live-directory';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as typeof import('jsdom');
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { pretendToBeVisual: true });
Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true });
const detail = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body);
const streamId = randomUUID();
const otherId = randomUUID();
const pool = detail.data.pool.poolAddress;
const wallet = detail.data.pool.creator;
const title = '🎙'.repeat(120);
const rootPath = resolve(__dirname, '../../../../../..');
const temporary = mkdtempSync(resolve(tmpdir(), 'live-flow-'));
type WriterState = { operation: unknown; stream: unknown };
type WriterOutput = Record<StreamOperation['status'], WriterState> & {
  create: unknown; start: unknown; end: unknown; liveUpdated: unknown; otherLive: unknown;
  directoryError: { status: number; body: unknown }; privateError: { status: number; body: unknown };
};
let wire: WriterOutput;
let server: Server;
let origin: string;
let api: typeof import('@/core/clients/livestream-api');
let creator: typeof import('@/widgets/live/use-creator-stream');
let directory: typeof import('@/widgets/live/use-live-directory');
let page: typeof import('./index');
let player: typeof import('@/widgets/live/stream-player');
let publicState: StreamOperation['status'] = 'suspended';
let privateUnavailable = false;
const requests: { method: string; path: string; authorization?: string }[] = [];
let root: Root | undefined;
let queryClient: QueryClient | undefined;
const previousOrigin = process.env.NEXT_PUBLIC_LIVESTREAM_API_URL;

beforeAll(async () => {
  const service = resolve(rootPath, 'services/launchpad/livestream');
  const overlay = resolve(temporary, 'overlay.json');
  writeFileSync(overlay, JSON.stringify({ Replace: {
    [resolve(service, 'cmd/livestreamd/main.go')]: resolve(__dirname, 'flow-live-writer.go.txt'),
  } }), { mode: 0o600 });
  wire = JSON.parse(execFileSync('go', ['run', '-overlay', overlay, './cmd/livestreamd'], {
    cwd: service, env: { ...process.env, GOWORK: 'off' }, encoding: 'utf8', timeout: 120_000,
    input: JSON.stringify({ RequestID: randomUUID(), StreamID: streamId, OtherID: otherId, Pool: pool, Wallet: wallet, Title: title }),
  })) as WriterOutput;
  // Replay the bytes emitted by the service adapters through the browser client.
  server = createServer((request, response) => {
    request.resume();
    const path = new URL(request.url ?? '/', 'http://localhost').pathname;
    requests.push({ method: request.method ?? 'GET', path, authorization: request.headers.authorization });
    response.setHeader('Content-Type', 'application/json');
    if (path === `/v2/streams/${streamId}`) { response.end(JSON.stringify(wire[publicState].stream)); return; }
    if (path === `/v2/streams/${streamId}/private-playback`) {
      if (privateUnavailable) { response.writeHead(wire.privateError.status); response.end(JSON.stringify(wire.privateError.body)); }
      else response.end(JSON.stringify(wire.live.stream));
      return;
    }
    if (path === '/v2/streams/live') { response.writeHead(wire.directoryError.status); response.end(JSON.stringify(wire.directoryError.body)); return; }
    response.writeHead(404); response.end();
  });
  await new Promise<void>(done => server.listen(0, '127.0.0.1', done));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_LIVESTREAM_API_URL = origin;
  api = await import('@/core/clients/livestream-api');
  creator = await import('@/widgets/live/use-creator-stream');
  directory = await import('@/widgets/live/use-live-directory');
  page = await import('./index');
  player = await import('@/widgets/live/stream-player');
}, 130_000);

afterEach(async () => {
  if (root) act(() => root!.unmount());
  root = undefined;
  await queryClient?.cancelQueries();
  queryClient?.clear();
  queryClient = undefined;
  dom.window.document.body.replaceChildren();
  requests.length = 0;
  publicState = 'suspended';
  privateUnavailable = false;
});
afterAll(async () => {
  if (server) await new Promise<void>(done => server.close(() => done()));
  if (previousOrigin === undefined) delete process.env.NEXT_PUBLIC_LIVESTREAM_API_URL;
  else process.env.NEXT_PUBLIC_LIVESTREAM_API_URL = previousOrigin;
  rmSync(temporary, { recursive: true, force: true });
  dom.window.close();
});

function operation(status: StreamOperation['status']): StreamOperation {
  return api.parseStreamOperation(wire[status].operation);
}
function entries(...rows: unknown[]) {
  const streams = rows.map(api.parseStream);
  if (!streams.every(api.isDirectoryEntry)) throw new Error('Directory projections required');
  return directory.assembleDirectory(streams, new Map(), new Map(), {});
}
function state(data = entries(wire.live.stream)): LiveDirectoryData {
  return { entries: data, fetchedAt: new Date().toISOString(), pages: 1, truncated: false, nextCursor: null,
    sources: { stats: { state: 'skipped' }, identity: { state: 'skipped' }, metadata: { state: 'skipped' } } };
}
function renderDirectory(data?: LiveDirectoryData) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (data) client.setQueryData(queryKeys.liveDirectory(), data);
  try { return renderToStaticMarkup(React.createElement(QueryClientProvider, { client }, React.createElement(page.default))); }
  finally { client.clear(); }
}

function live_creator_terminal_streams_allow_another_broadcast() {
  for (const status of ['failed', 'ended'] as const) {
    const held = operation(status);
    expect(creator.creatorSubmitAction(held)).toBe('create');
    const created = api.parseStreamOperation(wire.create);
    expect(creator.creatorSubmitAction(created)).toBe('edit');
    expect(created.status).toBe('requested');
  }
  expect(creator.creatorSubmitAction(null)).toBe('create');
}
test(live_creator_terminal_streams_allow_another_broadcast, live_creator_terminal_streams_allow_another_broadcast);

function live_creator_suspended_streams_offer_restoration_without_editing() {
  const held = operation('suspended');
  expect(creator.actionAvailability(held, false)).toEqual({ edit: false, start: false, end: false, credential: false, rotate: false, restore: true });
  expect(creator.creatorSubmitAction(held)).toBe('edit');
  expect(creator.actionAvailability(operation('ready'), false).edit).toBe(true);
}
test(live_creator_suspended_streams_offer_restoration_without_editing, live_creator_suspended_streams_offer_restoration_without_editing);

function live_titles_preserve_the_service_character_limit() {
  expect(api.parseStream(wire.ready.stream).title).toBe(title);
  expect(creator.streamTitle(title + 'extra')).toBe(title);
  expect([...creator.streamTitle(title + 'extra')]).toHaveLength(creator.TITLE_MAX);
  expect(creator.streamTitle('a'.repeat(creator.TITLE_MAX + 1))).toHaveLength(creator.TITLE_MAX);
  expect(creator.streamTitle('🎙')).toBe('🎙');
}
test(live_titles_preserve_the_service_character_limit, live_titles_preserve_the_service_character_limit);

function live_service_lifecycle_answers_drive_controls_and_player_states() {
  expect(api.parseStreamOperation(wire.start).status).toBe('ready');
  expect(api.parseStreamOperation(wire.end).status).toBe('ending');
  let held: StreamOperation | null = null;
  for (const status of ['requested', 'provisioning', 'ready', 'live', 'ending', 'ended', 'failed', 'suspended'] as const) {
    const next = operation(status);
    held = creator.mergeOperation(held, next);
    expect(held).toEqual(next);
    const available = creator.actionAvailability(held, false);
    expect(available.start).toBe(status === 'ready');
    expect(available.end).toBe(status === 'live');
    const stream = api.parseStream(wire[status].stream);
    expect(stream).not.toHaveProperty('streamKey');
    expect(player.lifecycleState(stream, false)).toBe(status === 'live' ? 'playable' : status === 'failed' ? 'failed' : status === 'suspended' ? 'blocked' : status === 'ending' || status === 'ended' ? 'ended' : 'connecting');
  }
  expect(creator.mergeOperation(held, operation('ready'))).toBe(held);
}
test(live_service_lifecycle_answers_drive_controls_and_player_states, live_service_lifecycle_answers_drive_controls_and_player_states);

function live_directory_pages_keep_one_current_projection_per_stream() {
  const first = entries(wire.live.stream);
  const next = entries(wire.liveUpdated, wire.otherLive, wire.live.stream);
  const joined = directory.mergeLiveEntries(first, next, first);
  expect(joined.map(entry => entry.stream.id)).toEqual([streamId, otherId]);
  expect(joined.map(entry => entry.stream.revision)).toEqual(['9', '10']);
  expect(joined.every(entry => entry.mcapRaw === null && entry.viewerCount === null)).toBe(true);
}
test(live_directory_pages_keep_one_current_projection_per_stream, live_directory_pages_keep_one_current_projection_per_stream);

function live_directory_loading_content_and_empty_states_keep_working_links() {
  expect(renderDirectory()).toContain('Loading streams');
  const ready = renderDirectory(state());
  expect(ready).toContain('data-stream-id');
  expect(ready).toContain('href="/live/all?sort=viewers"');
  expect(ready).toContain('href="/live/all?sort=mcap"');
  expect(ready).toContain(`/coin/${pool}`);
  expect(ready).toContain('No viewer count yet');
  const empty = renderDirectory(state([]));
  expect(empty).toContain('No live streams right now');
  expect(empty).toContain('href="/explore"');
  expect(empty).toContain('href="/create"');
  expect(empty).not.toContain('<video');
}
test(live_directory_loading_content_and_empty_states_keep_working_links, live_directory_loading_content_and_empty_states_keep_working_links);

async function live_creator_preview_reads_authorized_private_playback() {
  const stream = await api.getPlaybackStream(streamId, randomUUID());
  expect(stream?.isLive).toBe(true);
  expect(stream?.playbackUrl).toBe(api.parseStream(wire.live.stream).playbackUrl);
  expect(requests.map(request => request.path)).toEqual([`/v2/streams/${streamId}`, `/v2/streams/${streamId}/private-playback`]);
  expect(requests[0].authorization).toBeUndefined();
  expect(requests[1].authorization).toMatch(/^Bearer /);
}
test(live_creator_preview_reads_authorized_private_playback, live_creator_preview_reads_authorized_private_playback);

async function live_public_preview_keeps_suspension_and_private_errors_visible() {
  expect((await api.getPlaybackStream(streamId))?.status).toBe('suspended');
  expect(requests).toHaveLength(1);
  privateUnavailable = true;
  await expect(api.getPlaybackStream(streamId, randomUUID())).rejects.toMatchObject({ code: 'BROADCAST_NOT_PROVISIONED', status: 409 });
  publicState = 'live';
  const count = requests.length;
  expect((await api.getPlaybackStream(streamId, randomUUID()))?.isLive).toBe(true);
  expect(requests).toHaveLength(count + 1);
}
test(live_public_preview_keeps_suspension_and_private_errors_visible, live_public_preview_keeps_suspension_and_private_errors_visible);

async function until(predicate: () => boolean) {
  const deadline = Date.now() + 10_000;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error('Live view did not settle');
    await act(async () => { await new Promise(done => setTimeout(done, 20)); });
  }
}

async function live_continuation_retries_its_first_page_after_an_error() {
  let current: LiveDirectoryContinuation | undefined;
  function Continuation() {
    current = directory.useLiveDirectoryContinuation(randomCursor);
    return React.createElement('p', null, current.loading ? 'Loading streams' : current.error ?? 'Loaded');
  }
  const randomCursor = randomUUID();
  queryClient = new QueryClient();
  const client = new SdkClient({ apiOrigin: origin, mediaOrigin: origin, identityOrigin: origin, streamOrigin: origin.replace(/^http/, 'ws'), approvedStreamOrigins: [origin.replace(/^http/, 'ws')], production: false });
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  act(() => root!.render(React.createElement(QueryClientProvider, { client: queryClient! },
    React.createElement(MarketClientContext.Provider, { value: client }, React.createElement(Continuation)))));
  await until(() => !!current?.error && !current.loading);
  expect(current!.hasMore).toBe(true);
  const before = requests.length;
  act(() => current!.loadMore());
  await until(() => requests.length > before && !current!.loading);
  expect(requests.length).toBeGreaterThan(before);
  expect(current!.entries).toEqual([]);
  expect(container.textContent).toContain('Live is not loading right now');
}
test(live_continuation_retries_its_first_page_after_an_error, live_continuation_retries_its_first_page_after_an_error, 20_000);

async function live_continuation_follows_the_refreshed_directory_boundary() {
  const firstCursor = randomUUID();
  const nextCursor = randomUUID();
  let current: LiveDirectoryContinuation | undefined;
  function Continuation({ boundary }: { boundary: string | null }) {
    current = directory.useLiveDirectoryContinuation(firstCursor, boundary);
    return React.createElement('p', null, current.entries.map(entry => entry.stream.id).join(','));
  }
  queryClient = new QueryClient();
  queryClient.setQueryData(queryKeys.liveDirectoryContinuation(firstCursor), {
    pages: [state(entries(wire.live.stream)), state(entries(wire.liveUpdated))], pageParams: [firstCursor, randomUUID()],
  });
  queryClient.setQueryData(queryKeys.liveDirectoryContinuation(nextCursor), {
    pages: [state(entries(wire.otherLive))], pageParams: [nextCursor],
  });
  const client = new SdkClient({ apiOrigin: origin, mediaOrigin: origin, identityOrigin: origin, streamOrigin: origin.replace(/^http/, 'ws'), approvedStreamOrigins: [origin.replace(/^http/, 'ws')], production: false });
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  const view = (boundary: string | null) => React.createElement(QueryClientProvider, { client: queryClient! },
    React.createElement(MarketClientContext.Provider, { value: client }, React.createElement(Continuation, { boundary })));
  act(() => root!.render(view(firstCursor)));
  expect(current!.entries.map(entry => entry.stream.revision)).toEqual(['9']);
  expect(container.textContent).toBe(streamId);
  act(() => root!.render(view(nextCursor)));
  expect(container.textContent).toBe(otherId);
  expect(current!.hasMore).toBe(false);
  act(() => root!.render(view(null)));
  expect(current!.entries).toEqual([]);
  expect(current!.hasMore).toBe(false);
  expect(requests).toEqual([]);
}
test(live_continuation_follows_the_refreshed_directory_boundary, live_continuation_follows_the_refreshed_directory_boundary);
