import { createServer, type IncomingHttpHeaders, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import React, { act } from 'react';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { positionPage, positionPool, positionToken } from './repair-22-33-935.fixture';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url?: string; pretendToBeVisual?: boolean; storageQuota?: number }) => BrowserDom };
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

const { deploySwapChain, swapTestChain } = await import('@/widgets/swap/swap-chain.fixture');
const chain = await deploySwapChain();
const wallet = chain.account.toLowerCase();
interface Seen { url: string; headers: IncomingHttpHeaders }
const seen: Seen[] = [];
let generation = 0;
let holdContinuation = false;
let failContinuation = false;
let mismatchContinuation = false;
let nextAfterMatch = false;
const pending: { response: ServerResponse; body: ReturnType<typeof positionPage> }[] = [];
const server = createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://localhost');
  seen.push({ url: `${url.pathname}${url.search}`, headers: request.headers });
  response.setHeader('Content-Type', 'application/json');
  if (url.pathname !== '/v3/me/portfolio' || request.method !== 'GET') {
    response.statusCode = 404;
    response.end(JSON.stringify({ code: 'NOT_FOUND' }));
    return;
  }
  if (request.headers.authorization !== 'Bearer repair-position-session') {
    response.statusCode = 401;
    response.end(JSON.stringify({ code: 'AUTH_REQUIRED' }));
    return;
  }
  const cursor = url.searchParams.get('cursor');
  if (cursor && failContinuation) {
    response.statusCode = 400;
    response.end(JSON.stringify({ code: 'INVALID_CURSOR', retryable: false }));
    return;
  }
  const pageGeneration = cursor ? Number(/^snapshot-(\d)-page-two$/.exec(cursor)?.[1]) : generation;
  if (!Number.isInteger(pageGeneration) || pageGeneration < 0 || pageGeneration > 1) {
    response.statusCode = 400;
    response.end(JSON.stringify({ code: 'INVALID_CURSOR', retryable: false }));
    return;
  }
  const body = positionPage(wallet, cursor && mismatchContinuation ? 1 : pageGeneration, !!cursor, nextAfterMatch);
  if (cursor && holdContinuation) { pending.push({ response, body }); return; }
  response.end(JSON.stringify(body));
});
const origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)));
for (const name of ['PNL_API_URL', 'DATA_API_URL', 'METADATA_API_URL', 'GATEWAY_URL']) process.env[`NEXT_PUBLIC_${name}`] = origin;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { WagmiProvider, createConfig, custom } = await import('wagmi');
const { injected } = await import('wagmi/connectors');
const { connect, disconnect } = await import('wagmi/actions');
const { accountEpoch, accountSession, clearAccountSession, installAccountSession } = await import('@/widgets/profile/account-transport');
const { AccordionCard, accordionStorageKey } = await import('./accordion');
const { PositionSummary } = await import('./position-summary');
// The declared gate also executes the existing complete/empty/partial-candle assertions
// and the new snapshot, duplicate-event and missing-history regressions.
await import('./window-stats.test');

const wagmiConfig = createConfig({
  chains: [swapTestChain],
  connectors: [injected({ shimDisconnect: false, target: { id: 'recorded-chain', name: 'Recorded chain wallet', provider: chain.provider } })],
  transports: { 125: custom(chain.provider, { retryCount: 0 }) },
  ssr: false, multiInjectedProviderDiscovery: false,
});

function mount(node: React.ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  function render() {
    act(() => root.render(React.createElement(WagmiProvider, { config: wagmiConfig },
      React.createElement(QueryClientProvider, { client: queryClient }, node))));
  }
  render();
  const view = { container, queryClient, render, unmount() {
    act(() => root.unmount());
    queryClient.clear();
    container.remove();
    views.delete(view);
  } };
  views.add(view);
  return view;
}
const views = new Set<ReturnType<typeof mount>>();
function releaseContinuations() {
  holdContinuation = false;
  for (const { response, body } of pending.splice(0)) response.end(JSON.stringify(body));
}
async function waitFor(check: () => boolean) {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() > deadline) throw new Error(`Condition not met: ${document.body.textContent?.slice(0, 600)}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 15)); });
  }
}
function signIn() {
  installAccountSession({
    accessExpiresAt: new Date(Date.now() + 3_600_000).toISOString(), deviceLabel: 'Web browser',
    refreshExpiresAt: new Date(Date.now() + 86_400_000).toISOString(), revokedAt: null,
    scopes: ['account'], sessionId: 'repair-position-session', wallet,
  }, 'repair-position-session', accountEpoch());
}
function mountPosition(tokenAddress = positionToken) {
  signIn();
  return mount(React.createElement(PositionSummary, { poolAddress: positionPool, tokenAddress }));
}
function values(view: ReturnType<typeof mount>) {
  return Array.from(view.container.querySelectorAll('dd')).map(node => node.textContent);
}
function button(view: ReturnType<typeof mount>, text: string) {
  const found = Array.from(view.container.querySelectorAll('button')).find(node => node.textContent === text);
  expect(found, `Missing button: ${text}`).toBeDefined();
  return found!;
}
async function refresh(view: ReturnType<typeof mount>) {
  await act(async () => { await view.queryClient.invalidateQueries({ predicate: query => query.queryKey[0] === 'private-account' }); });
}

beforeAll(async () => { await connect(wagmiConfig, { connector: wagmiConfig.connectors[0] }); });
beforeEach(() => {
  generation = 0; failContinuation = false; mismatchContinuation = false; nextAfterMatch = false;
  seen.length = 0;
});
afterEach(() => {
  releaseContinuations();
  for (const view of Array.from(views)) view.unmount();
  clearAccountSession(false);
  browserGlobals.window = dom.window;
});
afterAll(async () => {
  await disconnect(wagmiConfig);
  await chain.provider.disconnect();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
});

function accordionStorageFallbackSurvivesBlockedReadsAndRemount() {
  const opaque = new JSDOM('<!DOCTYPE html><html><body></body></html>', {});
  browserGlobals.window = opaque.window;
  expect(() => opaque.window.localStorage).toThrow();
  const node = React.createElement(AccordionCard, { id: 'trade', title: 'Trade', children: 'Trade contents' });
  const view = mount(node);
  expect(button(view, 'Trade').getAttribute('aria-expanded')).toBe('true');
  act(() => button(view, 'Trade').click());
  expect(button(view, 'Trade').getAttribute('aria-expanded')).toBe('false');
  view.render();
  expect(view.container.querySelector('[role="region"]')?.hasAttribute('hidden')).toBe(true);
  view.unmount();
  const reopened = mount(node);
  expect(button(reopened, 'Trade').getAttribute('aria-expanded')).toBe('false');
  act(() => button(reopened, 'Trade').click());
  expect(button(reopened, 'Trade').getAttribute('aria-expanded')).toBe('true');
}

function accordionStorageFallbackPreservesObservedStorageBaseline() {
  const raw = JSON.stringify({ trade: true, security: false, similar: true });
  const full = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', storageQuota: accordionStorageKey.length + raw.length });
  browserGlobals.window = full.window;
  full.window.localStorage.setItem(accordionStorageKey, raw);
  const view = mount(React.createElement(AccordionCard, { id: 'trade', title: 'Trade', children: 'Trade contents' }));
  act(() => button(view, 'Trade').click());
  expect(full.window.localStorage.getItem(accordionStorageKey)).toBe(raw);
  expect(button(view, 'Trade').getAttribute('aria-expanded')).toBe('false');
  view.render();
  expect(button(view, 'Trade').getAttribute('aria-expanded')).toBe('false');
  view.unmount();
  const reopened = mount(React.createElement(AccordionCard, { id: 'trade', title: 'Trade', children: 'Trade contents' }));
  expect(button(reopened, 'Trade').getAttribute('aria-expanded')).toBe('false');
  expect(full.window.localStorage.getItem(accordionStorageKey)).toBe(raw);
}

async function positionPaginationFindsPageTwoAndStops() {
  nextAfterMatch = true;
  const view = mountPosition('0x' + 'BB'.repeat(20));
  await waitFor(() => values(view)[0] === '3.00');
  expect(values(view)).toEqual(['3.00', '2.50', '+0.40', '+0.90']);
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(seen.map(row => row.url)).toEqual(['/v3/me/portfolio?limit=50', '/v3/me/portfolio?limit=50&cursor=snapshot-0-page-two']);
  expect(seen.every(row => row.headers.authorization === 'Bearer repair-position-session' && row.headers.cookie === undefined)).toBe(true);
  expect(chain.provider.errors).toEqual([]);
}

async function positionPaginationStopsOnPageOne() {
  const view = mountPosition('0x' + '0'.repeat(39) + '1');
  await waitFor(() => values(view)[0] === '3.00');
  expect(seen.map(row => row.url)).toEqual(['/v3/me/portfolio?limit=50']);
}

async function positionPaginationReportsAbsenceAfterLastPage() {
  const view = mountPosition('0x' + 'dd'.repeat(20));
  await waitFor(() => view.container.querySelector('[aria-label*="You don\'t hold this coin"]') !== null);
  expect(values(view)).toEqual(['-', '-', '-', '-']);
  expect(seen.filter(row => row.url.includes('cursor=')).map(row => row.url)).toEqual(['/v3/me/portfolio?limit=50&cursor=snapshot-0-page-two']);
}

async function positionPaginationResetsOnSnapshotChangeAndIgnoresLatePages() {
  holdContinuation = true;
  const view = mountPosition();
  await waitFor(() => pending.length === 1);
  generation = 1;
  await refresh(view);
  await waitFor(() => pending.length === 2);
  expect(values(view)).toEqual(['...', '...', '...', '...']);
  const latest = pending.pop()!;
  await act(async () => { latest.response.end(JSON.stringify(latest.body)); });
  await waitFor(() => values(view)[0] === '4.00');
  releaseContinuations();
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(values(view)).toEqual(['4.00', '2.50', '+0.40', '+1.90']);
  expect(seen.filter(row => row.url.includes('cursor=')).map(row => row.url)).toEqual([
    '/v3/me/portfolio?limit=50&cursor=snapshot-0-page-two', '/v3/me/portfolio?limit=50&cursor=snapshot-1-page-two',
  ]);
}

async function positionPaginationRejectsMismatchedSnapshotAndRefreshes() {
  mismatchContinuation = true;
  const view = mountPosition();
  await waitFor(() => view.container.textContent?.includes('Refresh your portfolio') === true);
  expect(values(view)).toEqual(['-', '-', '-', '-']);
  expect(seen.filter(row => row.url.includes('cursor='))).toHaveLength(1);
  generation = 1; mismatchContinuation = false;
  act(() => button(view, 'Refresh your portfolio').click());
  await waitFor(() => values(view)[0] === '4.00');
  expect(values(view)).toEqual(['4.00', '2.50', '+0.40', '+1.90']);
}

async function positionPaginationSurfacesContinuationFailureAndRetries() {
  failContinuation = true;
  const view = mountPosition();
  await waitFor(() => view.container.textContent?.includes('Try loading your position again') === true);
  expect(values(view)).toEqual(['-', '-', '-', '-']);
  expect(view.container.querySelector('[role="status"]')?.textContent).toContain("Your position can't load right now");
  expect(seen.filter(row => row.url.includes('cursor='))).toHaveLength(1);
  failContinuation = false;
  act(() => button(view, 'Try loading your position again').click());
  await waitFor(() => values(view)[0] === '3.00');
  expect(seen.filter(row => row.url.includes('cursor='))).toHaveLength(2);
}

async function positionPaginationClearsAmountsOnSignOut() {
  const view = mountPosition();
  await waitFor(() => values(view)[0] === '3.00');
  const requests = seen.length;
  act(() => clearAccountSession(false));
  expect(accountSession()).toBeNull();
  expect(values(view)).toEqual(['-', '-', '-', '-']);
  expect(view.container.textContent).toContain('Sign in to see your position');
  expect(seen).toHaveLength(requests);
}

async function positionPaginationDiscardsPendingPageOnSignOut() {
  holdContinuation = true;
  const view = mountPosition();
  await waitFor(() => pending.length === 1);
  act(() => clearAccountSession(false));
  releaseContinuations();
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 30)); });
  expect(accountSession()).toBeNull();
  expect(values(view)).toEqual(['-', '-', '-', '-']);
  expect(view.container.textContent).toContain('Sign in to see your position');
  expect(seen.filter(row => row.url.includes('cursor='))).toHaveLength(1);
}

test(accordionStorageFallbackSurvivesBlockedReadsAndRemount.name, accordionStorageFallbackSurvivesBlockedReadsAndRemount);
test(accordionStorageFallbackPreservesObservedStorageBaseline.name, accordionStorageFallbackPreservesObservedStorageBaseline);
test(positionPaginationFindsPageTwoAndStops.name, positionPaginationFindsPageTwoAndStops);
test(positionPaginationStopsOnPageOne.name, positionPaginationStopsOnPageOne);
test(positionPaginationReportsAbsenceAfterLastPage.name, positionPaginationReportsAbsenceAfterLastPage);
test(positionPaginationResetsOnSnapshotChangeAndIgnoresLatePages.name, positionPaginationResetsOnSnapshotChangeAndIgnoresLatePages);
test(positionPaginationRejectsMismatchedSnapshotAndRefreshes.name, positionPaginationRejectsMismatchedSnapshotAndRefreshes);
test(positionPaginationSurfacesContinuationFailureAndRetries.name, positionPaginationSurfacesContinuationFailureAndRetries);
test(positionPaginationClearsAmountsOnSignOut.name, positionPaginationClearsAmountsOnSignOut);
test(positionPaginationDiscardsPendingPageOnSignOut.name, positionPaginationDiscardsPendingPageOnSignOut);
