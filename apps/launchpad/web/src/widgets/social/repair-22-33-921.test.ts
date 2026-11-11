import { createServer, type IncomingHttpHeaders } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import React, { act } from 'react';
import type { Root } from 'react-dom/client';
import { afterAll, afterEach, expect, test } from 'vitest';
import type { Callout, CalloutListResponse, FeedCard, FeedPage } from '@/core/market/use-feed';

// Committed loopback service fixtures exercise the SDK codecs and the mounted consumers.
const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (
    html: string,
    options: { url: string; pretendToBeVisual: boolean }
  ) => { window: Window & typeof globalThis };
};
const dom = new JSDOM('<!doctype html><html><body></body></html>', {
  url: 'http://localhost:3000',
  pretendToBeVisual: true,
});
const globals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] =
    typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
globals.window = dom.window;
globals.document = dom.window.document;
globals.IS_REACT_ACT_ENVIRONMENT = true;

const viewer = '0x1111111111111111111111111111111111111111';
const subject = '0x2222222222222222222222222222222222222222';
const poolA = '0x3333333333333333333333333333333333333333';
const poolB = '0x4444444444444444444444444444444444444444';
const timestamp = '2026-10-12T12:00:00.000Z';
const requestId = '8f0b7a8e-3a55-4f0f-9a43-1b1d7c3f2a10';
function feedCard(id: string, removed = false): FeedCard {
  return {
    Body: `post-${id}`,
    PositionDisclosure: '',
    PositionDisclosureKind: 'declared',
    PositionVerified: false,
    State: removed ? 'removed' : 'visible',
    Source: {
      Authorized: true,
      Corrected: removed,
      CreatorPublicID: 'account-author',
      EventID: `callout:${id}:1`,
      ID: id,
      Kind: 'callout',
      Pools: [],
      Public: true,
      PublishedAt: timestamp,
      SourceRevision: 1,
      Tombstone: removed,
      VisibilityRevision: 1,
    },
  };
}
function feedPage(entries: FeedCard[], cursor = ''): FeedPage {
  return {
    Algorithm: 'chronological-v1',
    Cursor: cursor,
    Entries: entries,
    FallbackReason: '',
    Personalized: false,
    Snapshot: 'snapshot-1',
  };
}
function poolCallout(pool: string, id: string): Callout {
  return {
    authorWallet: subject,
    body: `pool-post-${id}`,
    calloutId: `${pool.slice(-8)}-0000-4000-8000-${id.endsWith('one') ? '000000000001' : '000000000002'}`,
    createdAt: timestamp,
    direction: 'neutral',
    disclosure: 'organic',
    poolAddress: pool,
    revision: '1',
  };
}
function poolPage(pool: string, continuation: boolean): CalloutListResponse {
  return {
    data: {
      items: (continuation ? ['one', 'two'] : ['one']).map((id) =>
        poolCallout(pool, `${pool.slice(-4)}-${id}`)
      ),
    },
    generatedAt: timestamp,
    nextCursor: continuation ? null : 'pool-page-2',
    quality: 'complete',
    requestId,
    revision: null,
    watermark: null,
  };
}
const unavailable = {
  code: 'DEPENDENCY_UNAVAILABLE',
  message: 'Service unavailable',
  requestId,
  retryable: false,
  retryAfterMs: null,
  fieldErrors: [],
  details: null,
};
const seen: { path: string; method: string; headers: IncomingHttpHeaders }[] = [];
let failFeedPage = true;
let failPoolPage = true;
let stalled = false;
let interruptWrite = false;
const applied = new Set<string>();
const server = createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://localhost');
  const path = url.pathname;
  const method = request.method ?? 'GET';
  seen.push({ path: request.url ?? '', method, headers: request.headers });
  function json(status: number, body: unknown) {
    response.writeHead(status, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(body));
  }
  if (path === '/v2/discovery/feed') {
    if (url.searchParams.get('order') === 'chronological')
      return json(200, feedPage([feedCard('latest')]));
    if (!url.searchParams.has('cursor'))
      return json(200, feedPage([feedCard('a'), feedCard('b')], 'feed-page-2'));
    if (failFeedPage) return json(503, unavailable);
    return json(200, feedPage([feedCard('a', true), feedCard('c')]));
  }
  const pool = path.match(/^\/v2\/pools\/(0x[0-9a-f]+)\/callouts$/)?.[1];
  if (pool) {
    const continuation = url.searchParams.has('cursor');
    if (continuation && failPoolPage) return json(503, unavailable);
    return json(200, poolPage(pool, continuation));
  }
  if (path.startsWith('/v2/social/')) {
    if (request.headers.authorization !== 'Bearer repair-session')
      return json(401, { code: 'AUTH_REQUIRED' });
    if (method === 'PUT' || method === 'DELETE') {
      applied.add(`${method}:${request.headers['idempotency-key']}`);
      if (interruptWrite) {
        interruptWrite = false;
        response.destroy();
        return;
      }
    }
    if (stalled) {
      // One read stalls before headers, one after headers; DELETE stalls in an error body.
      if (path.endsWith('/followers') || method === 'DELETE') {
        response.writeHead(method === 'DELETE' ? 500 : 200, { 'Content-Type': 'application/json' });
        response.write('{"data":');
      }
      return;
    }
    if (method !== 'GET') {
      response.writeHead(204);
      response.end();
      return;
    }
    if (path.endsWith('/follow-stats'))
      return json(200, {
        data: { wallet: subject, followers: 1, following: 0, is_following: false },
      });
    return json(200, { data: { items: [viewer] }, nextCursor: null });
  }
  return json(404, { code: 'NOT_FOUND' });
});
const origin = await new Promise<string>((resolve) =>
  server.listen(0, '127.0.0.1', () =>
    resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)
  )
);
for (const name of ['DATA_API_URL', 'GATEWAY_URL', 'METADATA_API_URL'])
  process.env[`NEXT_PUBLIC_${name}`] = origin;
const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { HotCalloutsPanel } = await import('./hot-callouts-panel');
const {
  fetchFollowStats,
  fetchFollowers,
  setFollowing,
  resetFollowRoute,
  socialRequestTimeoutMs,
  SocialRequestDeadlineError,
} = await import('./social-transport');
const { followErrorMessage, useFollow } = await import('./use-follow');
const { accountEpoch, clearAccountSession, installAccountSession } =
  await import('@/widgets/profile/account-transport');

const mounted: { root: Root; host: HTMLDivElement; client: InstanceType<typeof QueryClient> }[] =
  [];
function mount(element: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const host = document.createElement('div');
  document.body.appendChild(host);
  const root = createRoot(host);
  const render = (node: React.ReactElement) =>
    act(() => root.render(React.createElement(QueryClientProvider, { client }, node)));
  render(element);
  mounted.push({ root, host, client });
  return { host, render };
}
async function until(predicate: () => boolean) {
  const limit = Date.now() + 2500;
  while (!predicate() && Date.now() < limit)
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
  expect(predicate()).toBe(true);
}
function button(host: ParentNode, label: string) {
  const found = Array.from(host.querySelectorAll('button')).find(
    (node) => node.textContent?.trim() === label
  );
  expect(found, label).toBeDefined();
  return found!;
}
function click(node: HTMLElement) {
  act(() => node.click());
}
function signIn() {
  installAccountSession(
    {
      sessionId: 'repair-session',
      wallet: viewer,
      deviceLabel: 'Browser',
      scopes: ['account'],
      accessExpiresAt: new Date(Date.now() + 3600000).toISOString(),
      refreshExpiresAt: new Date(Date.now() + 86400000).toISOString(),
      revokedAt: null,
    },
    'repair-session',
    accountEpoch()
  );
}
afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.root.unmount());
    view.client.clear();
    view.host.remove();
  }
  clearAccountSession(false);
  resetFollowRoute();
  stalled = false;
});
afterAll(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve) => server.close(() => resolve()));
});

async function calloutPaginationFeed() {
  const view = mount(React.createElement(HotCalloutsPanel));
  await until(() => view.host.textContent?.includes('post-b') === true);
  click(button(view.host, 'Load more callouts'));
  await until(() => view.host.querySelector('[data-state="unavailable"]') !== null);
  expect(view.host.textContent).toContain('post-b');
  failFeedPage = false;
  click(button(view.host, 'Try again'));
  await until(() => view.host.textContent?.includes('post-c') === true);
  expect(view.host.querySelectorAll('article')).toHaveLength(3);
  expect(view.host.textContent).toContain('This callout was removed');
  expect(view.host.textContent).not.toContain('post-a');
  expect(
    Array.from(view.host.querySelectorAll('button')).some(
      (node) => node.textContent === 'Load more callouts'
    )
  ).toBe(false);
  expect(seen.filter((row) => row.path.includes('cursor=feed-page-2'))).toHaveLength(2);
  click(view.host.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]')!);
  click(button(document.body, 'Latest callouts'));
  await until(() => view.host.textContent?.includes('post-latest') === true);
  expect(view.host.textContent).not.toContain('post-c');
  expect(
    seen.some((row) => row.path.includes('order=chronological') && !row.path.includes('cursor='))
  ).toBe(true);
  click(view.host.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]')!);
  click(button(document.body, 'Hot callouts'));
  await until(() => view.host.textContent?.includes('post-a') === true);
  expect(view.host.querySelectorAll('article')).toHaveLength(2);
  expect(view.host.textContent).not.toContain('post-c');
}
test(calloutPaginationFeed, calloutPaginationFeed);

async function calloutPaginationPool() {
  const view = mount(
    React.createElement(HotCalloutsPanel, { poolAddress: poolA, initialFeed: 'coin' })
  );
  await until(() => view.host.textContent?.includes('pool-post-3333-one') === true);
  click(button(view.host, 'Load more callouts'));
  await until(() => view.host.querySelector('[data-state="unavailable"]') !== null);
  expect(view.host.textContent).toContain('pool-post-3333-one');
  failPoolPage = false;
  click(button(view.host, 'Try again'));
  await until(() => view.host.textContent?.includes('pool-post-3333-two') === true);
  expect(view.host.querySelectorAll('article')).toHaveLength(2);
  expect(
    seen.filter(
      (row) => row.path.includes(`${poolA}/callouts`) && row.path.includes('cursor=pool-page-2')
    )
  ).toHaveLength(2);
  view.render(React.createElement(HotCalloutsPanel, { poolAddress: poolB, initialFeed: 'coin' }));
  await until(() => view.host.textContent?.includes('pool-post-4444-one') === true);
  expect(view.host.textContent).not.toContain('pool-post-3333');
  expect(view.host.querySelectorAll('article')).toHaveLength(1);
  expect(
    seen
      .filter((row) => row.path.includes(`${poolB}/callouts`))
      .every((row) => !row.path.includes('cursor='))
  ).toBe(true);
  click(button(view.host, 'Load more callouts'));
  await until(() => view.host.textContent?.includes('pool-post-4444-two') === true);
  expect(view.host.querySelectorAll('article')).toHaveLength(2);
}
test(calloutPaginationPool, calloutPaginationPool);

async function socialRequestDeadline() {
  signIn();
  stalled = true;
  const start = Date.now();
  const requests = [
    fetchFollowStats(subject),
    fetchFollowers(subject),
    setFollowing(subject, false, 'repair-put'),
    setFollowing(subject, true, 'repair-delete'),
  ];
  const results = await Promise.allSettled(requests);
  expect(Date.now() - start).toBeLessThan(socialRequestTimeoutMs + 2500);
  expect(seen.filter((row) => row.path.startsWith('/v2/social/'))).toHaveLength(4);
  for (const result of results) {
    expect(result.status).toBe('rejected');
    if (result.status !== 'rejected') throw new Error('A stalled response completed');
    expect(result.reason).toBeInstanceOf(SocialRequestDeadlineError);
    expect(result.reason).toMatchObject({
      status: 408,
      code: 'REQUEST_TIMEOUT',
      retryable: true,
      route: 'social-proxy',
    });
    expect(followErrorMessage(result.reason)).toBe('The request timed out, try again');
  }
  stalled = false;
  for (const index of [2, 3]) {
    const result = results[index];
    if (result.status !== 'rejected' || !(result.reason instanceof SocialRequestDeadlineError))
      throw new Error('Missing retry identity');
    await expect(
      setFollowing(subject, index === 3, result.reason.idempotencyKey)
    ).resolves.toMatchObject({ following: index !== 3, route: 'social-proxy' });
  }
  expect(applied.size).toBe(2);
  for (const key of ['repair-put', 'repair-delete'])
    expect(seen.filter((row) => row.headers['idempotency-key'] === key)).toHaveLength(2);
}
test(socialRequestDeadline, { timeout: 15000 }, socialRequestDeadline);

async function socialCallerCancellation() {
  signIn();
  // Resolve the real operation miss and proxy before testing caller cancellation.
  await fetchFollowStats(subject);
  stalled = true;
  const count = seen.length;
  const controller = new AbortController();
  const request = fetchFollowers(subject, null, 50, controller.signal);
  const result = Promise.allSettled([request]);
  await until(() => seen.length > count);
  controller.abort();
  const [cancelled] = await result;
  expect(cancelled.status).toBe('rejected');
  if (cancelled.status === 'rejected')
    expect(cancelled.reason).toMatchObject({ name: 'AbortError' });

  const customController = new AbortController();
  const customReason = new Error('Viewer left the followers panel');
  const customCount = seen.length;
  const customRequest = fetchFollowers(subject, null, 50, customController.signal);
  const customResult = Promise.allSettled([customRequest]);
  await until(() => seen.length > customCount);
  customController.abort(customReason);
  const [customCancelled] = await customResult;
  expect(customCancelled.status).toBe('rejected');
  if (customCancelled.status === 'rejected') expect(customCancelled.reason).toBe(customReason);

  const beforeCancelledRead = seen.length;
  await expect(fetchFollowStats(subject, customController.signal)).rejects.toBe(customReason);
  expect(seen).toHaveLength(beforeCancelledRead);
}
test(socialCallerCancellation, socialCallerCancellation);

function FollowConsumer() {
  const follow = useFollow(subject);
  return React.createElement(
    'button',
    { disabled: follow.pending, onClick: follow.toggle },
    `${follow.pending ? 'pending' : 'settled'}:${follow.following}`
  );
}
async function socialFollowRetryIdentity() {
  signIn();
  const view = mount(React.createElement(FollowConsumer));
  await until(() => view.host.textContent === 'settled:false');
  interruptWrite = true;
  const start = seen.length;
  click(button(view.host, 'settled:false'));
  await until(
    () =>
      seen.slice(start).some((row) => row.method === 'PUT' && row.path.startsWith('/v2/social/')) &&
      view.host.textContent === 'settled:false'
  );
  click(button(view.host, 'settled:false'));
  await until(
    () =>
      seen.slice(start).filter((row) => row.method === 'PUT' && row.path.startsWith('/v2/social/'))
        .length === 2 && view.host.textContent?.startsWith('settled:') === true
  );
  const writes = seen
    .slice(start)
    .filter((row) => row.method === 'PUT' && row.path.startsWith('/v2/social/'));
  expect(writes[0].headers['idempotency-key']).toBeTruthy();
  expect(writes[1].headers['idempotency-key']).toBe(writes[0].headers['idempotency-key']);
}
test(socialFollowRetryIdentity, socialFollowRetryIdentity);

async function socialFollowDeadlineRetry() {
  signIn();
  const view = mount(React.createElement(FollowConsumer));
  await until(() => view.host.textContent === 'settled:false');
  stalled = true;
  const start = seen.length;
  const startedAt = Date.now();
  click(button(view.host, 'settled:false'));
  await until(() =>
    seen.slice(start).some((row) => row.method === 'PUT' && row.path.startsWith('/v2/social/'))
  );
  expect(button(view.host, 'pending:true').disabled).toBe(true);
  // Let the production deadline expire while the loopback service stays connected.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, socialRequestTimeoutMs));
  });
  await until(() => view.host.textContent === 'settled:false');
  expect(Date.now() - startedAt).toBeLessThan(socialRequestTimeoutMs + 2500);
  expect(button(view.host, 'settled:false').disabled).toBe(false);
  stalled = false;
  click(button(view.host, 'settled:false'));
  await until(
    () =>
      seen.slice(start).filter((row) => row.method === 'PUT' && row.path.startsWith('/v2/social/'))
        .length === 2 && view.host.textContent?.startsWith('settled:') === true
  );
  const writes = seen
    .slice(start)
    .filter((row) => row.method === 'PUT' && row.path.startsWith('/v2/social/'));
  expect(writes[0].headers['idempotency-key']).toBeTruthy();
  expect(writes[1].headers['idempotency-key']).toBe(writes[0].headers['idempotency-key']);
}
test(socialFollowDeadlineRetry, { timeout: 15000 }, socialFollowDeadlineRetry);
