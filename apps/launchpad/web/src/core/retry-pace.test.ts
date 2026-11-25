import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import React, { act, createElement } from 'react';
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest';
import type { UseQueryResult } from '@tanstack/react-query';
import { SdkClient, decodeV2Contract } from '@kindle/sdk';
import { ResourceRetryPace, marketRetryPace, marketResourceKey } from './retry-pace';
import { MarketClientContext } from './market/client';
import { createReplayUdfDatafeed } from '@/widgets/trading/udf-datafeed';
import { ErrorState, UnavailableState } from '@/ui/primitives';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as typeof import('jsdom');
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'http://localhost:3000/search?q=coin', pretendToBeVisual: true });
const globals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  globals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
globals.window = dom.window;
globals.document = dom.window.document;
globals.IS_REACT_ACT_ENVIRONMENT = true;
Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
window.matchMedia = query => ({ matches: false, media: query, onchange: null, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => false });
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { useCandles } = await import('./market/use-candles');
const { sparklineWindow, useSparkline } = await import('./market/use-sparkline');
const { useMarketTrades } = await import('@/page-modules/token/market-session-context');
const { createRoot } = await import('react-dom/client');
const { AppRouterContext } = await import('next/dist/shared/lib/app-router-context.shared-runtime');
const { adaptForAppRouterInstance } = await import('next/dist/shared/lib/router/adapters');
const { default: singletonRouter } = await import('next/router');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { defineChain } = await import('viem');
const { SearchView } = await import('@/page-modules/search');
const { LeaderboardView } = await import('@/widgets/leaderboard/leaderboard-view');
const core = JSON.parse(readFileSync(new URL('./test-fixtures/market-production/core-api-wire.json', import.meta.url), 'utf8'));
const candlePage = decodeV2Contract('CandlesResponse', core.candles.body);
const tradesPage = decodeV2Contract('TradesResponse', core.trades.body);
const pool = candlePage.data.items[0].poolAddress;
const origin = 'http://127.0.0.1:18081';
const config = { apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: 'ws://127.0.0.1:18081', approvedStreamOrigins: ['ws://127.0.0.1:18081'], production: false };
const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [origin] } } });
const wagmi = createConfig({ chains: [chain], connectors: [], transports: { 125: http(origin, { retryCount: 0 }) }, ssr: true, multiInjectedProviderDiscovery: false });
const nativeFetch = globalThis.fetch;
const requests: Array<{ path: string; at: number }> = [];
let succeeds = false;
let candlesSucceed = false;
let networkFailure = false;
let retryAfterMs = 1000;
const errorEnvelope = (retryable: boolean) => ({ code: 'DEPENDENCY_UNAVAILABLE', message: 'DEPENDENCY_UNAVAILABLE', requestId: candlePage.requestId, retryable, retryAfterMs: retryable ? retryAfterMs : null, fieldErrors: [], details: null });
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', 'X-Schema-Version': '2' } });
const countingFetch: typeof fetch = async input => {
  const url = new URL(input instanceof Request ? input.url : String(input));
  requests.push({ path: url.pathname + url.search, at: Date.now() });
  if (url.pathname.endsWith('/trades') || url.pathname.endsWith('/candles')) {
    if (networkFailure) throw new TypeError('Failed to fetch');
    return (succeeds || candlesSucceed && url.pathname.endsWith('/candles')) ? json(url.pathname.endsWith('/trades') ? tradesPage : url.pathname.includes(pool) && url.searchParams.get('interval') === '1m' ? candlePage : { ...candlePage, data: { ...candlePage.data, items: [] } }) : json(errorEnvelope(true), 503);
  }
  if (url.pathname === '/v2/search') {
    if (url.searchParams.get('type') === 'profile') return json(errorEnvelope(false), 412);
    return json({ ...tradesPage, data: { items: [] } });
  }
  return json(errorEnvelope(false), 501);
};
const cleanups: Array<() => void> = [];
function mount(node: React.ReactNode, sdk = new SdkClient(config, countingFetch)) {
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const render = (content: React.ReactNode) => act(() => root.render(createElement(QueryClientProvider, { client: cache }, createElement(MarketClientContext.Provider, { value: sdk }, content))));
  render(node);
  cleanups.push(() => { act(() => root.unmount()); cache.clear(); container.remove(); });
  return { container, cache, sdk, render };
}
async function advance(ms: number) {
  let remaining = ms;
  do {
    const step = Math.min(1000, remaining);
    await act(async () => { await vi.advanceTimersByTimeAsync(step); });
    remaining -= step;
  } while (remaining > 0);
}
function resourceRequests(path: 'trades' | 'candles', address = pool) { return requests.filter(row => row.path.startsWith(`/v2/pools/${address}/${path}?`)); }
async function bounds(path: 'trades' | 'candles', addresses = [pool]) {
  await advance(0);
  await advance(15_000);
  for (const address of addresses) {
    expect(resourceRequests(path, address).length).toBeGreaterThan(0);
    expect(resourceRequests(path, address).length).toBeLessThanOrEqual(5);
  }
  await advance(105_000);
  for (const address of addresses) expect(resourceRequests(path, address).length).toBeLessThanOrEqual(7);
}
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-10-13T09:45:00Z'));
  marketRetryPace.clear();
  requests.length = 0; succeeds = false; candlesSucceed = false; networkFailure = false; retryAfterMs = 1000;
  globalThis.fetch = countingFetch;
});
afterEach(() => {
  cleanups.splice(0).reverse().forEach(cleanup => cleanup());
  marketRetryPace.clear();
  globalThis.fetch = nativeFetch;
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
  vi.useRealTimers();
});
afterAll(() => dom.window.close());

async function idempotentTransportRepeatsTradesAndCandlesBeforeQueryRetries() {
  for (const [name, response] of [['trades', 'TradesResponse'], ['candles', 'CandlesResponse']] as const) {
    const client = new SdkClient(config, countingFetch);
    const result = client.requestV2(`/v2/pools/${pool}/${name}?limit=100`, response, { idempotentRead: true }).catch(error => error);
    await advance(1000);
    expect((await result).status).toBe(503);
    expect(resourceRequests(name)).toHaveLength(3);
    expect(resourceRequests(name)[2].at - resourceRequests(name)[0].at).toBeLessThan(1000);
  }
}
test(idempotentTransportRepeatsTradesAndCandlesBeforeQueryRetries.name, idempotentTransportRepeatsTradesAndCandlesBeforeQueryRetries);

async function tradeListSharesPaceAcrossMountsAndRestoresSuccessfulReads() {
  let result!: ReturnType<typeof useMarketTrades>;
  function Trades() { result = useMarketTrades(pool); return createElement('p', null, result.isPending ? 'Waiting' : result.isError ? 'Unavailable' : 'Ready'); }
  const view = mount(createElement(Trades));
  await bounds('trades');
  expect(view.container.textContent).toBe('Unavailable');
  const before = resourceRequests('trades').length;
  view.render(createElement(Trades, { key: 'remounted' }));
  await advance(0);
  expect(resourceRequests('trades')).toHaveLength(before);
  succeeds = true;
  await act(async () => { await result.refetch(); });
  expect(resourceRequests('trades')).toHaveLength(before + 1);
  await advance(0);
  expect(view.container.textContent).toBe('Ready');
  await act(async () => { await view.cache.invalidateQueries(); });
  expect(resourceRequests('trades')).toHaveLength(before + 2);
  succeeds = false;
  await act(async () => { void result.refetch(); });
  await advance(15_000);
  expect(result!.data).toEqual(tradesPage);
  expect(result!.isError).toBe(true);
}
test(tradeListSharesPaceAcrossMountsAndRestoresSuccessfulReads.name, tradeListSharesPaceAcrossMountsAndRestoresSuccessfulReads);

async function twoCandlePanelsAndSparklineShareOneResourceQuery() {
  const range = { ...sparklineWindow(), limit: 24 };
  function Panels() {
    const first = useCandles(pool, '1h', range);
    const second = useCandles(pool, '1h', range);
    const small = useSparkline(pool);
    return createElement('p', null, `${first.status}:${second.status}:${small.status}`);
  }
  const view = mount(createElement(Panels));
  expect(view.cache.getQueryCache().getAll()).toHaveLength(1);
  await bounds('candles');
  succeeds = true;
  await advance(60_000);
  await advance(0);
  expect(view.container.textContent).toBe('success:success:success');
  const before = resourceRequests('candles').length;
  await act(async () => { await view.cache.invalidateQueries(); });
  expect(resourceRequests('candles')).toHaveLength(before + 1);
}
test(twoCandlePanelsAndSparklineShareOneResourceQuery.name, twoCandlePanelsAndSparklineShareOneResourceQuery);

async function ninePoolSparklinesKeepIndependentBoundedPaces() {
  const addresses = Array.from({ length: 9 }, (_, index) => `0x${(index + 1).toString(16).padStart(40, '0')}`);
  function Small({ address }: { address: string }) { const query = useSparkline(address); return createElement('span', null, query.status); }
  const view = mount(createElement(React.Fragment, null, ...addresses.map(address => createElement(Small, { key: address, address }))));
  await bounds('candles', addresses);
  expect(new Set(requests.map(request => request.path.split('?')[0])).size).toBe(9);
  const before = addresses.map(address => resourceRequests('candles', address).length);
  succeeds = true;
  await advance(60_000);
  expect(view.cache.getQueryCache().getAll().every(query => query.state.status === 'success')).toBe(true);
  await act(async () => { await view.cache.invalidateQueries(); });
  addresses.forEach((address, index) => expect(resourceRequests('candles', address)).toHaveLength(before[index] + 2));
}
test(ninePoolSparklinesKeepIndependentBoundedPaces.name, ninePoolSparklinesKeepIndependentBoundedPaces);

async function injectedClockBoundsConnectionFailuresAndServerHints() {
  let now = Date.now();
  const pace = new ResourceRetryPace(() => now);
  const client = new SdkClient(config, countingFetch);
  const path = `/v2/pools/${pool}/trades?limit=100`;
  const key = marketResourceKey(new URL(path, origin));
  const read = () => pace.read(key, signal => client.requestV2(path, 'TradesResponse', { signal, idempotentRead: false })).catch(error => error);
  try {
    networkFailure = true;
    for (let second = 0; second <= 120; second++) {
      now += second ? 1000 : 0;
      await read();
      if (second === 15) expect(resourceRequests('trades').length).toBeLessThanOrEqual(5);
    }
    expect(resourceRequests('trades').length).toBeLessThanOrEqual(7);
    const before = resourceRequests('trades').length;
    networkFailure = false; succeeds = true;
    const error = await read();
    pace.retryNow(error);
    expect(await read()).toEqual(tradesPage);
    expect(await read()).toEqual(tradesPage);
    expect(resourceRequests('trades')).toHaveLength(before + 2);
  } finally { pace.clear(); }
}
test(injectedClockBoundsConnectionFailuresAndServerHints.name, injectedClockBoundsConnectionFailuresAndServerHints);

async function chartHistoryAndPollShareBoundedReadsAndResumeOnSuccess() {
  const candle = candlePage.data.items[0];
  const from = Date.parse(candle.start) / 1000;
  const feed = createReplayUdfDatafeed(pool, 'price', () => {}, undefined, { apiOrigin: origin, valuationOrigin: origin });
  const read = () => new Promise<void>(resolve => feed.getBars({}, '1', { from, to: from + 60 }, () => resolve(), () => resolve()));
  try {
    feed.subscribeBars({}, '1', () => {}, 'price');
    await advance(0);
    for (let second = 0; second <= 120; second++) {
      await read();
      if (second === 15) {
        expect(resourceRequests('candles').length).toBeLessThanOrEqual(5);
      }
      if (second < 120) await advance(1000);
    }
    expect(resourceRequests('candles').length).toBeLessThanOrEqual(7);
    feed.unsubscribeBars('price');
    succeeds = true;
    feed.retry();
    let bars = 0;
    await new Promise<void>((resolve, reject) => feed.getBars({}, '1', { from, to: from + 60 }, values => { bars = values.length; resolve(); }, reason => reject(new Error(reason))));
    expect(bars).toBe(candlePage.data.items.length);
    const before = resourceRequests('candles').length;
    await read();
    expect(resourceRequests('candles')).toHaveLength(before + 1);
  } finally { feed.dispose(); }
}
test(chartHistoryAndPollShareBoundedReadsAndResumeOnSuccess.name, chartHistoryAndPollShareBoundedReadsAndResumeOnSuccess);

async function failedReadWaitsFifteenSecondsAndKeepsRetryDuringBackgroundReads() {
  let result!: UseQueryResult<unknown>;
  function Panel() {
    result = useMarketTrades(pool);
    return result.isPending ? createElement('p', { 'aria-busy': true }, 'Waiting') : result.isError ? createElement(ErrorState, { title: createElement('span', null, 'Trades unavailable'), onRetry: () => { void result.refetch(); } }) : createElement('p', null, 'Trades');
  }
  const view = mount(createElement(Panel));
  await advance(14_999);
  expect(view.container.textContent).toBe('Waiting');
  await advance(1);
  expect(view.container.textContent).toContain('Trades unavailable');
  expect(view.container.querySelector('button')?.textContent).toBe('Try again');
  const before = resourceRequests('trades').length;
  await act(async () => view.container.querySelector('button')!.click());
  await advance(0);
  expect(resourceRequests('trades')).toHaveLength(before + 1);
  expect(view.container.textContent).toContain('Trades unavailable');
  await advance(1000);
  expect(view.container.textContent).toContain('Trades unavailable');
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' });
  await act(async () => document.dispatchEvent(new document.defaultView!.Event('visibilitychange', { bubbles: true })));
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
  const hiddenCount = resourceRequests('trades').length;
  await act(async () => document.dispatchEvent(new document.defaultView!.Event('visibilitychange', { bubbles: true })));
  await advance(0);
  expect(resourceRequests('trades')).toHaveLength(hiddenCount + 1);
}
test(failedReadWaitsFifteenSecondsAndKeepsRetryDuringBackgroundReads.name, failedReadWaitsFifteenSecondsAndKeepsRetryDuringBackgroundReads);

async function profileSearchNotServedShowsItsTextWithoutRetry() {
  const view = mount(createElement(AppRouterContext.Provider, { value: adaptForAppRouterInstance(singletonRouter) }, createElement(SearchView, { initialQuery: 'coin' })));
  await advance(200);
  expect(requests.some(request => request.path.includes('type=profile'))).toBe(true);
  expect(view.container.textContent).toContain('User search is not available yet');
  expect(view.container.textContent).not.toContain('Try again');
}
test(profileSearchNotServedShowsItsTextWithoutRetry.name, profileSearchNotServedShowsItsTextWithoutRetry);

async function leadersWithoutCompetitionsShowTheirTextWithoutRetry() {
  const view = mount(createElement(WagmiProvider, { config: wagmi }, createElement(LeaderboardView)));
  await advance(0);
  const hero = view.container.querySelector('[data-cup-state="unsupported"]');
  expect(hero).not.toBeNull();
  expect(hero!.textContent).toContain('Competitions are not available yet');
  expect(hero!.textContent).toContain('No competition scheduled');
  expect(hero!.textContent).not.toContain('Try again');
}
test(leadersWithoutCompetitionsShowTheirTextWithoutRetry.name, leadersWithoutCompetitionsShowTheirTextWithoutRetry);

async function unservedStatesHideRetryWhileFailedReadsKeepIt() {
  const server = decodeV2Contract('Error', errorEnvelope(false));
  const { HttpError } = await import('@kindle/sdk');
  for (const variant of ['panel', 'inline', 'chip'] as const) {
    const view = mount(createElement(UnavailableState, { operation: 'IF_SEARCH', status: 412, notServed: true, title: 'User search', variant, onRetry: () => {} }));
    expect(view.container.textContent).toContain('User search is not available yet');
    expect(view.container.querySelector('button')).toBeNull();
    const unserved = mount(createElement(UnavailableState, { operation: 'IF_CANDLES', status: 503, error: new HttpError(503, server), variant, onRetry: () => {} }));
    expect(unserved.container.querySelector('button')?.textContent).toBe('Try again');
    const failed = mount(createElement(UnavailableState, { operation: 'IF_CANDLES', status: null, error: new TypeError('Failed to fetch'), variant, onRetry: () => {} }));
    expect(failed.container.querySelector('button')?.textContent).toBe('Try again');
  }
}
test(unservedStatesHideRetryWhileFailedReadsKeepIt.name, unservedStatesHideRetryWhileFailedReadsKeepIt);

async function chartConfirmedTradeAnnotationsKeepTheirRetryPace() {
  candlesSucceed = true;
  const from = Date.parse(candlePage.data.items[0].start) / 1000;
  const feed = createReplayUdfDatafeed(pool, 'price', () => {}, undefined, { apiOrigin: origin, valuationOrigin: origin });
  try {
    await new Promise<void>((resolve, reject) => feed.getBars({}, '1', { from, to: from + 60 }, () => resolve(), error => reject(new Error(error))));
    await bounds('trades');
    expect(resourceRequests('trades').every(request => request.path.endsWith('?limit=50'))).toBe(true);
    const before = resourceRequests('trades').length;
    succeeds = true;
    feed.retry();
    await advance(0);
    expect(resourceRequests('trades')).toHaveLength(before + 1);
  } finally { feed.dispose(); }
}
test(chartConfirmedTradeAnnotationsKeepTheirRetryPace.name, chartConfirmedTradeAnnotationsKeepTheirRetryPace);

async function zeroDelayServerCannotRestartTheResourceBudget() {
  let now = Date.now();
  const pace = new ResourceRetryPace(() => now);
  const client = new SdkClient(config, countingFetch);
  const path = `/v2/pools/${pool}/trades?limit=100`;
  const key = marketResourceKey(new URL(path, origin));
  retryAfterMs = 0;
  try {
    for (let second = 0; second <= 120; second++) {
      now += second ? 1000 : 0;
      await pace.read(key, signal => client.requestV2(path, 'TradesResponse', { signal, idempotentRead: false })).catch(error => error);
      if (second === 15) expect(resourceRequests('trades').length).toBeLessThanOrEqual(5);
    }
    expect(resourceRequests('trades').length).toBeLessThanOrEqual(7);
  } finally { pace.clear(); }
}
test(zeroDelayServerCannotRestartTheResourceBudget.name, zeroDelayServerCannotRestartTheResourceBudget);

async function sameOriginHistoryAndMarketReadersUseTheSameResourceIdentity() {
  const path = `/v2/pools/${pool}/candles?interval=1m&limit=50&from=2026-10-13T09%3A00%3A00.000Z&to=2026-10-13T10%3A00%3A00.000Z`;
  expect(marketResourceKey(new URL(path, window.location.origin))).toBe(marketResourceKey(new URL(path, origin)));
}
test(sameOriginHistoryAndMarketReadersUseTheSameResourceIdentity.name, sameOriginHistoryAndMarketReadersUseTheSameResourceIdentity);

async function cancellingOneReaderKeepsTheSharedRequestForTheOther() {
  const pace = new ResourceRetryPace();
  const client = new SdkClient(config, countingFetch);
  const controller = new AbortController();
  let release!: () => void;
  let sharedSignal!: AbortSignal;
  const load = async (signal: AbortSignal) => {
    sharedSignal = signal;
    await new Promise<void>(resolve => { release = resolve; });
    return client.requestV2(`/v2/pools/${pool}/trades?limit=100`, 'TradesResponse', { signal, idempotentRead: false });
  };
  try {
    succeeds = true;
    const cancelled = pace.read('trades', load, controller.signal).catch(error => error);
    const retained = pace.read('trades', load);
    controller.abort();
    expect((await cancelled).name).toBe('AbortError');
    expect(sharedSignal.aborted).toBe(false);
    release();
    expect(await retained).toEqual(tradesPage);
    expect(resourceRequests('trades')).toHaveLength(1);
  } finally { pace.clear(); }
}
test(cancellingOneReaderKeepsTheSharedRequestForTheOther.name, cancellingOneReaderKeepsTheSharedRequestForTheOther);
