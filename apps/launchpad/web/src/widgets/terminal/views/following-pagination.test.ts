import { createServer, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import React, { act, createElement } from 'react';
import { afterAll, afterEach, beforeEach, expect, test } from 'vitest';

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

const ownerA = '0x' + '11'.repeat(20);
const ownerB = '0x' + '22'.repeat(20);
const followedA = '0x' + 'ab'.repeat(20);
const followedB = '0x' + 'cd'.repeat(20);
const followedC = '0x' + 'ef'.repeat(20);
const otherFollow = '0x' + '34'.repeat(20);
const iso = '2026-10-12T15:00:00.000Z';
// socialFollows emits followCursor as RawURLEncoding(JSON), then interactionData.
const cursor = Buffer.from(JSON.stringify({ at: 1791817200, wallet: followedB })).toString('base64url');
const envelope = (items: unknown, nextCursor: string | null = null) => ({
  requestId: '6f1c2a54-1b9e-4c55-9d0e-3f1a2b3c4d5e', data: { items }, nextCursor,
  watermark: null, revision: null, quality: 'complete', generatedAt: iso,
});
// Committed, producer-shaped pages from social.go's followEdge/interactionData contract.
const firstPage = envelope([{ wallet: followedA, created_at: iso }, { wallet: followedB, created_at: iso }], cursor);
const secondPage = envelope([{ wallet: followedB.toUpperCase().replace('0X', '0x'), created_at: iso }, { wallet: followedC, created_at: iso }]);
const otherPage = envelope([{ wallet: otherFollow, created_at: iso }]);
const requests: { wallet: string; cursor: string | null; limit: string | null; authorization: string | undefined; method: string | undefined }[] = [];
const profileRequests: string[][] = [];
let continuationStatus = 200;
let holdContinuation = false;
let heldResponse: ServerResponse | undefined;

function reply(response: ServerResponse, status: number, body: unknown) {
  response.writeHead(status, { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*' });
  response.end(JSON.stringify(body));
}

const server = createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://fixture');
  const match = /^\/v2\/social\/users\/(0x[0-9a-f]{40})\/following$/.exec(url.pathname);
  if (match) {
    const wallet = match[1]!;
    const after = url.searchParams.get('cursor');
    requests.push({ wallet, cursor: after, limit: url.searchParams.get('limit'), authorization: request.headers.authorization, method: request.method });
    const token = wallet === ownerB ? 'token-b' : 'token-a';
    if (request.headers.authorization !== `Bearer ${token}`) return reply(response, 401, { code: 'AUTH_EXPIRED' });
    if (wallet === ownerB) return reply(response, 200, otherPage);
    if (!after) return reply(response, 200, firstPage);
    if (after !== cursor) return reply(response, 400, { code: 'INVALID_INPUT' });
    if (holdContinuation) { heldResponse = response; return; }
    return reply(response, continuationStatus, continuationStatus === 200 ? secondPage : { code: 'DEPENDENCY_UNAVAILABLE' });
  }
  if (url.pathname === '/v2/profiles:batch') {
    const wallets = (url.searchParams.get('wallets') ?? '').split(',');
    profileRequests.push(wallets);
    return reply(response, 200, envelope(wallets.map(wallet => ({ wallet, displayName: `Trader ${wallet.slice(2, 4)}`, bio: '', avatarAssetId: null, bannerAssetId: null, website: null, revision: '7' }))));
  }
  // Real follow-count reads use their normal unavailable presentation.
  return reply(response, 404, { code: 'NOT_FOUND' });
});
await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
for (const name of ['DATA_API_URL', 'GATEWAY_URL', 'METADATA_API_URL']) process.env[`NEXT_PUBLIC_${name}`] = origin;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { FollowingList, fetchFollowing } = await import('./track');
const { accountEpoch, clearAccountSession, installAccountSession } = await import('@/widgets/profile/account-transport');

function signIn(wallet = ownerA, sessionId = 'session-a') {
  installAccountSession({ wallet, sessionId, accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(), refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(), deviceLabel: 'Web browser', revokedAt: null, scopes: ['account'] }, wallet === ownerB ? 'token-b' : 'token-a', accountEpoch());
}

function mountFollowing() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => root.render(createElement(QueryClientProvider, { client }, createElement(FollowingList))));
  return { client, container, unmount: () => { act(() => root.unmount()); client.clear(); container.remove(); } };
}

async function until(check: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!check() && Date.now() < deadline) await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
  expect(check()).toBe(true);
}

function rows(container: HTMLElement) {
  return [...container.querySelectorAll('ul[aria-label="Followed wallets"] > li')];
}

function click(container: HTMLElement, label: string) {
  const button = [...container.querySelectorAll('button')].find(button => button.textContent === label);
  expect(button, `Missing ${label} control`).toBeDefined();
  act(() => button!.click());
}

beforeEach(() => {
  requests.length = 0; profileRequests.length = 0;
  continuationStatus = 200; holdContinuation = false; heldResponse = undefined;
  clearAccountSession(false);
  signIn();
});
afterEach(() => { clearAccountSession(false); heldResponse?.destroy(); });
afterAll(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  dom.window.close();
});

async function following_pagination_pages_and_deduplication() {
  const mounted = mountFollowing();
  try {
    await until(() => rows(mounted.container).length === 2 && mounted.container.textContent!.includes('Trader ab'));
    expect(requests).toEqual([{ wallet: ownerA, cursor: null, limit: '50', authorization: 'Bearer token-a', method: 'GET' }]);
    expect(mounted.container.querySelector('h2')?.textContent).toContain('2+');
    click(mounted.container, 'Load more followed wallets');
    await until(() => rows(mounted.container).length === 3 && mounted.container.textContent!.includes('Trader ef'));
    expect(requests.map(request => request.cursor)).toEqual([null, cursor]);
    expect(rows(mounted.container).map(row => row.querySelector('a')?.getAttribute('href'))).toEqual([followedA, followedB, followedC].map(wallet => `/profile/${wallet}`));
    expect(profileRequests.flat()).toContain(followedC);
    expect(mounted.container.querySelector('h2')?.textContent).toBe('Following3');
    expect([...mounted.container.querySelectorAll('button')].some(button => button.textContent === 'Load more followed wallets')).toBe(false);
  } finally { mounted.unmount(); }
}
test(following_pagination_pages_and_deduplication.name, following_pagination_pages_and_deduplication);

async function following_pagination_account_and_session_resets() {
  const mounted = mountFollowing();
  try {
    await until(() => rows(mounted.container).length === 2);
    holdContinuation = true;
    click(mounted.container, 'Load more followed wallets');
    await until(() => heldResponse !== undefined);
    expect(mounted.container.querySelector('button')?.disabled).toBe(true);
    act(() => signIn(ownerB, 'session-b'));
    expect(rows(mounted.container)).toHaveLength(0);
    await until(() => rows(mounted.container).length === 1);
    expect(rows(mounted.container)[0]!.querySelector('a')?.getAttribute('href')).toBe(`/profile/${otherFollow}`);
    reply(heldResponse!, 200, secondPage);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 30)); });
    expect(rows(mounted.container)).toHaveLength(1);
    expect(mounted.client.getQueryCache().findAll({ predicate: query => query.queryKey[0] === 'private-account' && query.queryKey[1] === 'session-a' })).toHaveLength(0);
    act(() => clearAccountSession(false));
    expect(rows(mounted.container)).toHaveLength(0);
    expect(mounted.client.getQueryCache().findAll({ predicate: query => query.queryKey[0] === 'private-account' && query.queryKey[1] === 'session-b' })).toHaveLength(0);
    holdContinuation = false;
    act(() => signIn());
    await until(() => rows(mounted.container).length === 2);
    click(mounted.container, 'Load more followed wallets');
    await until(() => rows(mounted.container).length === 3);
    // Even reusing the same wallet and session id starts a new epoch at page one.
    act(() => signIn());
    expect(rows(mounted.container)).toHaveLength(0);
    await until(() => rows(mounted.container).length === 2);
    expect(requests.map(request => [request.wallet, request.cursor, request.authorization])).toEqual([
      [ownerA, null, 'Bearer token-a'], [ownerA, cursor, 'Bearer token-a'],
      [ownerB, null, 'Bearer token-b'], [ownerA, null, 'Bearer token-a'],
      [ownerA, cursor, 'Bearer token-a'], [ownerA, null, 'Bearer token-a'],
    ]);
  } finally { mounted.unmount(); }
}
test(following_pagination_account_and_session_resets.name, following_pagination_account_and_session_resets);

async function following_pagination_continuation_failures() {
  for (const status of [400, 503]) {
    act(() => signIn());
    const mounted = mountFollowing();
    try {
      await until(() => rows(mounted.container).length === 2);
      const start = requests.length;
      continuationStatus = status;
      click(mounted.container, 'Load more followed wallets');
      await until(() => mounted.container.querySelector('[role="alert"]') !== null);
      expect(rows(mounted.container)).toHaveLength(2);
      expect(mounted.container.textContent).toContain("Couldn't load more, try again");
      expect(requests.slice(start).map(request => request.cursor)).toEqual([cursor]);
      continuationStatus = 200;
      click(mounted.container, 'Try again');
      await until(() => rows(mounted.container).length === 3);
      expect(requests.slice(start).map(request => request.cursor)).toEqual([cursor, cursor]);
      expect(mounted.container.querySelector('[role="alert"]')).toBeNull();
    } finally { mounted.unmount(); }
  }
}
test(following_pagination_continuation_failures.name, following_pagination_continuation_failures);

async function following_pagination_rejects_late_session_response() {
  holdContinuation = true;
  const request = fetchFollowing(ownerA, undefined, fetch, cursor);
  const failure = expect(request).rejects.toBeInstanceOf(Error);
  await until(() => heldResponse !== undefined);
  act(() => signIn(ownerB, 'session-b'));
  reply(heldResponse!, 200, secondPage);
  await failure;
}
test(following_pagination_rejects_late_session_response.name, following_pagination_rejects_late_session_response);
