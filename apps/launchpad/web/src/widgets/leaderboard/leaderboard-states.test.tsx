import { createServer, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { setTimeout as wait } from 'node:timers/promises';
import React, { act } from 'react';
import { afterAll, afterEach, describe, expect, it, vi } from 'vitest';
import { SdkClient, type V2ContractTypes } from '@kindle/sdk';
import { marketRetryPace } from '@/core/retry-pace';
import { assertControlEdges } from '@/core/app-foundation/control-edges.test-support';

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

type Board = V2ContractTypes['LeaderboardsResponse'];
type Category = Board['data']['snapshot']['category'];
type Period = Board['data']['snapshot']['window'];
type ReadState = 'pending' | 'rows' | 'empty' | 'unavailable';
type Caller = 'ticker' | 'home' | 'traders' | 'profile';
interface BoardRequest { path: string; category: Category; period: Period; limit: number; at: number }
const requests: BoardRequest[] = [];
const held = new Map<ServerResponse, BoardRequest>();
let answer: ReadState = 'pending';
let retryAfterMs = 1_000;
let retryable = true;
const wallet = '0x' + 'a1'.repeat(20);
const requestId = '0f0e2a8e-5b3c-4d8a-9a1e-2c6d7b8f9a01';
const watermark: V2ContractTypes['Watermark'] = {
  chainId: '125', height: '4200', blockHash: '0x' + '6'.repeat(64), projectionRevision: '7',
  integrity: 'healthy', observedAt: '2026-10-12T15:00:00.000Z',
};

function board(request: BoardRequest): Board {
  return {
    requestId, generatedAt: '2026-10-12T15:00:01.000Z', nextCursor: null, quality: 'complete', revision: '7', watermark,
    data: { snapshot: {
      snapshotId: '7b6f2b4e-3c1d-4e2f-9a8b-1c2d3e4f5a6b', category: request.category, window: request.period,
      methodologyVersion: 'pnl-v1', watermark,
      entries: answer === 'rows' ? [{ rank: 1, wallet, metricRaw: '2500000', metricSigned: '2500000', quality: 'complete' }] : [],
    } },
  };
}

function respond(response: ServerResponse, request: BoardRequest) {
  response.setHeader('Content-Type', 'application/json');
  if (answer === 'unavailable') {
    const error: V2ContractTypes['Error'] = {
      code: 'DEPENDENCY_UNAVAILABLE', message: 'DEPENDENCY_UNAVAILABLE', requestId,
      retryable, retryAfterMs: retryable ? retryAfterMs : null, fieldErrors: [], details: null,
    };
    response.writeHead(503, { 'Retry-After': String(retryAfterMs / 1_000) });
    response.end(JSON.stringify(error));
  } else {
    response.end(JSON.stringify(board(request)));
  }
}

const server = createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://localhost');
  const category = url.pathname.split('/')[3];
  const period = url.searchParams.get('window');
  if (url.pathname.startsWith('/v2/leaderboards/') && (category === 'trader' || category === 'creator' || category === 'volume') && (period === '24h' || period === '7d' || period === '30d')) {
    const read: BoardRequest = { path: request.url ?? '', category, period, limit: Number(url.searchParams.get('limit')), at: Date.now() };
    requests.push(read);
    if (answer === 'pending') {
      held.set(response, read);
      response.on('close', () => held.delete(response));
    } else respond(response, read);
    return;
  }
  if (url.pathname === '/v2/profiles:batch') {
    const profiles: V2ContractTypes['ProfilesBatchResponse'] = {
      requestId, generatedAt: '2026-10-12T15:00:01.000Z', nextCursor: null,
      quality: 'complete', revision: null, watermark: null, data: { items: [] },
    };
    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(profiles));
    return;
  }
  response.writeHead(404);
  response.end();
});
const origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)));
const previousOrigins = new Map<string, string | undefined>();
for (const name of ['NEXT_PUBLIC_DATA_API_URL', 'NEXT_PUBLIC_GATEWAY_URL', 'NEXT_PUBLIC_METADATA_API_URL']) {
  previousOrigins.set(name, process.env[name]);
  process.env[name] = origin;
}

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider, focusManager, onlineManager, notifyManager } = await import('@tanstack/react-query');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { defineChain } = await import('viem');
const { MarketClientContext } = await import('@/core/market/client');
const { queryKeys } = await import('@/core/query-keys');
const { default: TickerStrip } = await import('@/shell/ticker-strip');
const { LeaderboardWidget } = await import('@/widgets/home/leaderboard-widget');
const { LeaderboardPanel } = await import('@/widgets/profile/leaderboard-panel');
const { TradersBoard } = await import('./traders-board');

const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: ['http://127.0.0.1:1'] } } });
const wagmiConfig = createConfig({ chains: [chain], transports: { 125: http() }, ssr: true, multiInjectedProviderDiscovery: false });
const props = { columnId: 'leaderboard', filter: '', density: 'comfortable' as const, view: 'list' as const };
const callers: Caller[] = ['ticker', 'home', 'traders', 'profile'];
const limits: Record<Caller, number> = { ticker: 20, home: 50, traders: 100, profile: 25 };
const emptyCopy: Record<Caller, string> = {
  ticker: 'No top earners in the last 24h', home: 'No traders yet', traders: 'No traders yet', profile: 'No traders ranked yet',
};
const panelPadding: Record<Caller, Record<'empty' | 'unavailable', string[] | null>> = {
  ticker: { empty: null, unavailable: null },
  home: { empty: ['p-8'], unavailable: null },
  traders: { empty: ['p-8'], unavailable: ['px-4', 'py-8'] },
  profile: { empty: ['p-8'], unavailable: ['px-4', 'py-8'] },
};
const BORDER = /\b(border(?!-0\b|-none\b)(-[a-z0-9/\[\]-]+)?|divide-[a-z]|ring-(?!offset)[a-z0-9]|outline-(?!none|focus|2|offset)[a-z]+)\b/;
const tokens = readFileSync(new URL('../../app/tokens.css', import.meta.url), 'utf8');
let cleanup: (() => void) | null = null;

function element(caller: Caller): React.ReactElement {
  switch (caller) {
    case 'ticker': return <TickerStrip />;
    case 'home': return <LeaderboardWidget {...props} />;
    case 'traders': return <TradersBoard period="1d" />;
    case 'profile': return <LeaderboardPanel initialWindow="24h" />;
  }
}

function mount(node: React.ReactElement, width = 1440, scheme: 'light' | 'dark' = 'light') {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width });
  document.documentElement.style.colorScheme = scheme;
  notifyManager.setScheduler(queueMicrotask);
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: 'ws://127.0.0.1:1', approvedStreamOrigins: ['ws://127.0.0.1:1'], production: false });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  container.style.width = `${width}px`;
  document.body.appendChild(container);
  const root = createRoot(container);
  const render = (next: React.ReactElement) => act(() => {
    root.render(<WagmiProvider config={wagmiConfig}><QueryClientProvider client={queryClient}><MarketClientContext.Provider value={client}>{next}</MarketClientContext.Provider></QueryClientProvider></WagmiProvider>);
  });
  render(node);
  cleanup = () => { act(() => root.unmount()); queryClient.clear(); container.remove(); };
  return { container, queryClient, render };
}

async function waitFor(check: () => boolean) {
  const deadline = performance.now() + 4_000;
  while (!check()) {
    if (performance.now() > deadline) throw new Error(`Leaderboard state did not settle: ${document.body.textContent}`);
    await act(async () => {
      if (vi.isFakeTimers()) await vi.advanceTimersByTimeAsync(0);
      await wait(5);
    });
  }
}

async function advance(ms: number) {
  await act(async () => { await vi.advanceTimersByTimeAsync(ms); await wait(5); });
}

function release(state: Exclude<ReadState, 'pending'>) {
  answer = state;
  for (const [response, request] of held) respond(response, request);
  held.clear();
}

function placeholder(container: HTMLElement): boolean {
  return container.querySelector('[aria-busy="true"], .sl-skeleton') !== null;
}

function unavailable(container: HTMLElement, caller: Caller): boolean {
  if (caller === 'ticker') return requests.length > 0 && !placeholder(container) && container.querySelector('a[href^="/profile/"]') === null;
  return container.textContent?.includes('Leaderboard is not available yet') === true;
}

function retryButton(container: HTMLElement): HTMLButtonElement {
  const button = Array.from(container.querySelectorAll('button')).find(item => item.textContent === 'Try again');
  expect(button).toBeDefined();
  return button!;
}

function expectTokens(container: HTMLElement, caller: Caller, state: ReadState, width: number, scheme: 'light' | 'dark') {
  expect(window.innerWidth).toBe(width);
  expect(document.documentElement.style.colorScheme).toBe(scheme);
  for (const role of ['surface-1', 'surface-2', 'text', 'text-muted', 'text-faint']) expect(tokens).toContain(`--sl-${role}-${scheme}:`);
  expect(assertControlEdges(container.innerHTML)).not.toMatch(BORDER);
  expect(container.innerHTML).not.toMatch(/(?:color|background-color):\s*(?:#|rgb)/);
  if (caller === 'ticker') {
    const frame = container.querySelector('section[aria-label="Market ticker"]')!;
    expect(frame.className.split(/\s+/)).toEqual(expect.arrayContaining(['hidden', 'md:flex', 'min-h-ticker-height', 'min-[835px]:h-ticker-height', 'min-[835px]:flex-nowrap', 'gap-2', 'bg-canvas', 'px-3', 'sm:px-4']));
    expect(container.querySelectorAll('[role="radio"]')).toHaveLength(5);
  } else if (caller === 'home') {
    expect(container.firstElementChild?.className).toContain('min-h-0');
    expect(container.querySelector('[role="radiogroup"]')?.getAttribute('aria-label')).toBe('Leaderboard');
    if (state === 'unavailable') {
      const notice = container.querySelector('[data-state="unavailable"]')!;
      expect(notice.className.split(/\s+/)).toEqual(expect.arrayContaining(['bg-surface-2', 'text-text-muted', 'text-xs', 'min-h-6', 'flex-wrap', 'mx-2', 'my-1.5']));
    }
  } else {
    const frame = container.querySelector('section')!;
    expect(frame.className).toContain('bg-surface-1');
    expect(frame.className).toContain('rounded-lg');
    expect(frame.className).toContain(caller === 'traders' ? 'sm:p-4' : 'p-2');
  }
  if (state === 'empty' || state === 'unavailable') {
    const padding = panelPadding[caller][state];
    if (padding !== null) {
      const panel = container.querySelector(state === 'unavailable' ? '[class*="@container/state"][role="status"]' : '[class*="@container/state"]')!;
      expect(panel).not.toBeNull();
      expect(panel.className.split(/\s+/)).toEqual(expect.arrayContaining(['w-full', 'min-w-0', 'gap-3', ...padding]));
      expect(panel.querySelector('p')?.className).toContain('text-text-muted');
    }
  }
}

afterEach(() => {
  cleanup?.(); cleanup = null;
  for (const response of held.keys()) response.destroy();
  held.clear(); requests.length = 0;
  answer = 'pending'; retryAfterMs = 1_000; retryable = true;
  marketRetryPace.clear();
  vi.useRealTimers();
  window.localStorage.clear();
  document.documentElement.style.removeProperty('color-scheme');
  focusManager.setFocused(undefined); onlineManager.setOnline(true);
});

afterAll(async () => {
  server.closeAllConnections();
  await new Promise<void>(resolve => server.close(() => resolve()));
  for (const [name, value] of previousOrigins) {
    if (value === undefined) delete process.env[name];
    else process.env[name] = value;
  }
  dom.window.close();
});

for (const caller of callers) {
  for (const width of [390, 1440]) {
    for (const scheme of ['light', 'dark'] as const) {
      describe(`${caller}, ${width}px, ${scheme}`, () => {
        async function leaderboardPending() {
          const { container } = mount(element(caller), width, scheme);
          expect(placeholder(container)).toBe(true);
          await waitFor(() => requests.length === 1);
          expect(held.size).toBe(1);
          expect(placeholder(container)).toBe(true);
          expect(requests[0].path).toBe(`/v2/leaderboards/trader?window=24h&limit=${limits[caller]}`);
          expect(container.querySelector('a[href^="/profile/"]')).toBeNull();
          expectTokens(container, caller, 'pending', width, scheme);
        }

        async function leaderboardRows() {
          const { container, queryClient } = mount(element(caller), width, scheme);
          await waitFor(() => held.size === 1);
          release('rows');
          await waitFor(() => container.querySelector(`a[href="/profile/${wallet}"]`) !== null && !placeholder(container));
          expect(container.querySelectorAll(`a[href="/profile/${wallet}"]`)).toHaveLength(1);
          expect(container.textContent).toContain('2.50');
          expect(container.textContent).not.toContain('not available yet');
          expect(requests).toHaveLength(1);
          expectTokens(container, caller, 'rows', width, scheme);
          answer = 'pending';
          act(() => { void queryClient.invalidateQueries({ queryKey: queryKeys.leaderboard('trader', '24h', limits[caller]) }); });
          await waitFor(() => held.size === 1);
          expect(placeholder(container)).toBe(false);
          expect(container.querySelector(`a[href="/profile/${wallet}"]`)).not.toBeNull();
          release('rows');
          await waitFor(() => queryClient.isFetching() === 0);
        }

        async function leaderboardEmpty() {
          const { container, queryClient } = mount(element(caller), width, scheme);
          await waitFor(() => held.size === 1);
          release('empty');
          await waitFor(() => queryClient.getQueryState(queryKeys.leaderboard('trader', '24h', limits[caller]))?.status === 'success');
          await waitFor(() => container.textContent?.includes(emptyCopy[caller]) === true);
          expect(placeholder(container)).toBe(false);
          expect(container.querySelector('a[href^="/profile/"]')).toBeNull();
          expect(container.querySelector('[role="alert"]')).toBeNull();
          expect(container.textContent).not.toContain('not available yet');
          expect(requests).toHaveLength(1);
          if (caller === 'home') expect(container.textContent).toContain('Nothing in the last 24h yet');
          if (caller === 'traders') expect(container.querySelector('a[href="/explore"]')?.textContent).toBe('Browse coins');
          if (caller === 'profile') expect(container.textContent).toContain('Rankings show up once people start trading');
          expectTokens(container, caller, 'empty', width, scheme);
        }

        async function leaderboardUnavailable() {
          vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] });
          const { container, queryClient } = mount(element(caller), width, scheme);
          await waitFor(() => held.size === 1);
          release('unavailable');
          const key = queryKeys.leaderboard('trader', '24h', limits[caller]);
          await waitFor(() => queryClient.getQueryState(key)?.fetchFailureCount === 1);
          expect(requests).toHaveLength(1);
          expect(queryClient.getQueryState(key)?.fetchStatus).toBe('fetching');
          expect(placeholder(container)).toBe(true);
          await advance(999);
          expect(requests).toHaveLength(1);
          await advance(1);
          await waitFor(() => queryClient.getQueryState(key)?.fetchFailureCount === 2);
          expect(requests).toHaveLength(2);
          expect(placeholder(container)).toBe(true);
          for (const [index, delay] of [2_000, 4_000, 8_000].entries()) {
            await advance(delay);
            await waitFor(() => queryClient.getQueryState(key)?.fetchFailureCount === index + 3);
          }
          await waitFor(() => unavailable(container, caller));
          expect(requests).toHaveLength(5);
          expect(placeholder(container)).toBe(false);
          expect(container.textContent).not.toMatch(/DEPENDENCY_UNAVAILABLE|IF_LEADERBOARDS|503/);
          expect(container.querySelector('a[href^="/profile/"]')).toBeNull();
          if (caller !== 'ticker') expect(retryButton(container).disabled).toBe(false);
          expectTokens(container, caller, 'unavailable', width, scheme);
        }

        it(leaderboardPending.name, leaderboardPending);
        it(leaderboardRows.name, leaderboardRows);
        it(leaderboardEmpty.name, leaderboardEmpty);
        it(leaderboardUnavailable.name, leaderboardUnavailable);
      });
    }
  }
}

describe.each(callers)('%s retry budget', caller => {
  async function leaderboardRequestsStayBoundedForSixtySeconds() {
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] });
    answer = 'unavailable';
    const started = Date.now();
    const { container, queryClient } = mount(element(caller));
    const key = queryKeys.leaderboard('trader', '24h', limits[caller]);
    await waitFor(() => queryClient.getQueryState(key)?.fetchFailureCount === 1);
    expect(placeholder(container)).toBe(true);
    for (const [index, delay] of [1_000, 2_000, 4_000, 8_000].entries()) {
      await advance(delay);
      await waitFor(() => queryClient.getQueryState(key)?.fetchFailureCount === index + 2);
      expect(requests).toHaveLength(index + 2);
      expect(placeholder(container)).toBe(index < 3);
    }
    expect(queryClient.getQueryState(key)?.status).toBe('error');
    expect(requests.map(request => request.at - started)).toEqual([0, 1_000, 3_000, 7_000, 15_000]);
    await advance(30_000);
    await waitFor(() => requests.length === 6 && queryClient.getQueryState(key)?.fetchFailureCount === 1);
    expect(requests[5].at - started).toBe(45_000);
    expect(unavailable(container, caller)).toBe(true);
    expect(placeholder(container)).toBe(false);
    await advance(60_000);
    await waitFor(() => requests.length === 7 && queryClient.getQueryState(key)?.fetchFailureCount === 1);
    expect(requests[6].at - started).toBe(105_000);
    await advance(15_000);
    expect(requests).toHaveLength(7);
    expect(unavailable(container, caller)).toBe(true);
    expect(placeholder(container)).toBe(false);
    if (caller === 'ticker') return;

    answer = 'pending';
    act(() => retryButton(container).click());
    await waitFor(() => held.size === 1);
    expect(requests).toHaveLength(8);
    expect(requests[7].at - started).toBe(120_000);
    expect(placeholder(container)).toBe(false);
    expect(unavailable(container, caller)).toBe(true);
    release('rows');
    await waitFor(() => container.querySelector(`a[href="/profile/${wallet}"]`) !== null && !placeholder(container));
    expect(requests).toHaveLength(8);
  }

  it(leaderboardRequestsStayBoundedForSixtySeconds.name, leaderboardRequestsStayBoundedForSixtySeconds);
});

async function leaderboardRetryDelayUsesTheCacheCap() {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] });
  answer = 'unavailable'; retryAfterMs = 60_000;
  const { queryClient } = mount(<LeaderboardPanel initialWindow="24h" />);
  const key = queryKeys.leaderboard('trader', '24h', 25);
  await waitFor(() => queryClient.getQueryState(key)?.fetchFailureCount === 1);
  await advance(14_999);
  expect(requests).toHaveLength(1);
  await advance(1);
  await waitFor(() => queryClient.getQueryState(key)?.status === 'error');
  expect(requests).toHaveLength(2);
  expect(requests[1].at - requests[0].at).toBe(15_000);
  await advance(14_999);
  expect(requests).toHaveLength(2);
  await advance(1);
  await waitFor(() => requests.length === 3);
  expect(requests[2].at - requests[0].at).toBe(30_000);
}

async function leaderboardNonRetryableUnavailableAnswersOnce() {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] });
  answer = 'unavailable'; retryable = false;
  const { container, queryClient } = mount(<LeaderboardPanel initialWindow="24h" />);
  await waitFor(() => queryClient.getQueryState(queryKeys.leaderboard('trader', '24h', 25))?.status === 'error');
  expect(unavailable(container, 'profile')).toBe(true);
  await advance(60_000);
  expect(requests).toHaveLength(1);
}

async function leaderboardCallersShareMatchingRequests() {
  const { container, queryClient } = mount(<>
    <TickerStrip />
    <LeaderboardWidget {...props} />
    <TradersBoard period="1d" />
    <LeaderboardPanel initialWindow="24h" limit={50} />
  </>);
  await waitFor(() => held.size === 3);
  expect(requests.map(request => request.limit).sort((a, b) => a - b)).toEqual([20, 50, 100]);
  release('rows');
  await waitFor(() => container.querySelectorAll(`a[href="/profile/${wallet}"]`).length === 4 && queryClient.isFetching() === 0);
  expect(requests).toHaveLength(3);
}

async function leaderboardRequestKeysSeparateCategoryWindowAndLimit() {
  const { queryClient } = mount(<>
    <LeaderboardPanel initialWindow="24h" limit={20} />
    <LeaderboardPanel initialWindow="24h" limit={20} />
    <LeaderboardPanel initialWindow="7d" limit={20} />
    <LeaderboardPanel initialWindow="24h" limit={25} />
    <LeaderboardPanel initialWindow="24h" initialMetric="volume" limit={20} />
  </>);
  await waitFor(() => held.size === 4);
  expect(requests.map(request => request.path).sort()).toEqual([
    '/v2/leaderboards/trader?window=24h&limit=20', '/v2/leaderboards/trader?window=24h&limit=25',
    '/v2/leaderboards/trader?window=7d&limit=20', '/v2/leaderboards/volume?window=24h&limit=20',
  ]);
  release('empty');
  await waitFor(() => queryClient.isFetching() === 0);
  expect(requests).toHaveLength(4);
}

async function leaderboardChangingWindowStartsItsOwnPendingState() {
  answer = 'unavailable'; retryable = false;
  const { container } = mount(<LeaderboardPanel initialWindow="24h" />);
  await waitFor(() => unavailable(container, 'profile'));
  answer = 'pending';
  const select = container.querySelector<HTMLSelectElement>('select[aria-label="Window"]')!;
  act(() => { select.value = '7d'; select.dispatchEvent(new window.Event('change', { bubbles: true })); });
  await waitFor(() => held.size === 1);
  expect(placeholder(container)).toBe(true);
  expect(requests[1].period).toBe('7d');
  release('empty');
  await waitFor(() => container.textContent?.includes(emptyCopy.profile) === true);
  expect(placeholder(container)).toBe(false);
}

async function leaderboardTickerKeepsItsOtherFeeds() {
  answer = 'unavailable'; retryable = false;
  const { container, queryClient } = mount(<TickerStrip />);
  await waitFor(() => queryClient.getQueryState(queryKeys.leaderboard('trader', '24h', 20))?.status === 'error');
  expect(container.querySelector('a[href^="/profile/"]')).toBeNull();
  expect(container.textContent).not.toMatch(/not available yet|DEPENDENCY_UNAVAILABLE|Loading/);
  expect(container.querySelectorAll('[role="radio"]')).toHaveLength(5);
  const feed = container.querySelector('section[aria-label="Market ticker"] > div:last-child')!;
  await waitFor(() => feed.childElementCount === 0);
  expect(feed.innerHTML).toBe('');
  const recent = container.querySelector<HTMLButtonElement>('button[aria-label="Recently viewed"]')!;
  act(() => recent.click());
  expect(container.textContent).toContain('Coins you open show up here');
  expect(requests).toHaveLength(1);
}

it(leaderboardRetryDelayUsesTheCacheCap.name, leaderboardRetryDelayUsesTheCacheCap);
it(leaderboardNonRetryableUnavailableAnswersOnce.name, leaderboardNonRetryableUnavailableAnswersOnce);
it(leaderboardCallersShareMatchingRequests.name, leaderboardCallersShareMatchingRequests);
it(leaderboardRequestKeysSeparateCategoryWindowAndLimit.name, leaderboardRequestKeysSeparateCategoryWindowAndLimit);
it(leaderboardChangingWindowStartsItsOwnPendingState.name, leaderboardChangingWindowStartsItsOwnPendingState);
it(leaderboardTickerKeepsItsOtherFeeds.name, leaderboardTickerKeepsItsOtherFeeds);
