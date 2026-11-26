import { createRequire } from 'node:module';
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import type { QueryClient as QueryClientType } from '@tanstack/react-query';
import { afterAll, afterEach, beforeEach, expect, test, vi } from 'vitest';
import { decodeV2Contract, SdkClient } from '@kindle/sdk';
import { fetchCandleHistoryOutcome, type CandleHistoryOutcome } from '@/core/clients/candle-stats';
import { v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import { queryKeys } from '@/core/query-keys';
import type { PoolSnapshot, PoolStats } from '@/core/market/pool-resolver';

vi.mock('@/core/clients/candle-stats', async original => ({
  ...await original<typeof import('@/core/clients/candle-stats')>(),
  fetchCandleHistoryOutcome: vi.fn(),
}));

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

const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { MarketClientContext } = await import('@/core/market/client');
const { useAllTimeHigh } = await import('@/core/market/use-all-time-high');
const McAthCard = (await import('./mc-ath-card')).default;

const day = 86_400;
const now = Date.parse('2026-11-26T12:34:56Z');
const createdAt = new Date(now - 40 * day * 1000).toISOString();
const from = Math.floor((now / 1000 - 40 * day) / day) * day;
const detail = decodeV2Contract('PoolDetailResponse', v2Reads.responses.poolDetailReal.body).data;
const pool = detail.pool.poolAddress.toLowerCase();
const stats: PoolStats = { ...detail.stats, totalSupplyRaw: '1000000000000000', totalSupplyValuationUsdlRaw: '100000000000' };
const read = vi.mocked(fetchCandleHistoryOutcome);
const candles: CandleHistoryOutcome = { status: 'ok', history: {
  t: [from, from + 20 * day, Math.floor(now / 1000 / day) * day],
  o: [0.0001, 0.00012, 0.0001], h: [0.00012, 0.0002, 0.00015],
  l: [0.00009, 0.0001, 0.00009], c: [0.0001, 0.00015, 0.0001], v: [10, 20, 30],
} };
const sdk = new SdkClient({ apiOrigin: 'http://localhost:3000', identityOrigin: 'http://localhost:3000', mediaOrigin: 'http://localhost:3000', streamOrigin: 'ws://localhost:3000', approvedStreamOrigins: ['ws://localhost:3000'], production: false });
const mounted: Array<{ root: Root; host: HTMLDivElement; client: QueryClientType }> = [];

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(now);
  read.mockReset();
  read.mockResolvedValue(candles);
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});
afterEach(() => {
  for (const view of mounted.splice(0)) {
    act(() => view.root.unmount());
    view.host.remove();
    view.client.clear();
  }
  vi.useRealTimers();
});
afterAll(() => dom.window.close());

function mount(creation: string | null = createdAt, client = new QueryClient()) {
  const snapshot: PoolSnapshot = {
    pool: { chainId: '125', poolAddress: pool, tokenAddress: detail.token.address },
    token: { ...detail.token, decimals: 6, decimalsQuality: 'verified' }, metadata: null, stats,
    creator: detail.pool.creator, createdAt: creation, generatedAt: new Date(now).toISOString(),
    resolvedBy: 'detail', unavailable: { stats: null, metadata: null, registry: null },
  };
  client.setQueryData(queryKeys.poolSnapshot(pool), snapshot);
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  const view = { root, host, client };
  mounted.push(view);
  const render = (liveStats: PoolStats | null = null, address = pool) => act(() => root.render(
    <QueryClientProvider client={client}><MarketClientContext.Provider value={sdk}>
      <McAthCard poolAddress={address} liveStats={liveStats} />
    </MarketClientContext.Provider></QueryClientProvider>,
  ));
  render();
  return { ...view, render };
}
async function advance(milliseconds = 5) {
  await act(async () => { await vi.advanceTimersByTimeAsync(milliseconds); });
}
const athValue = (host: HTMLElement) => Array.from(host.querySelectorAll('dt')).find(node => node.textContent === 'ATH')?.nextElementSibling;

async function theBarNamesBothValuationsAndTheirShare() {
  const { host } = mount();
  await advance();
  expect(read).toHaveBeenCalledExactlyOnceWith(pool, '1D', from, now / 1000, { signal: expect.any(AbortSignal) });
  expect(host.textContent).toContain('MC100K USDL');
  expect(athValue(host)?.textContent).toBe('200K USDL');
  const bar = host.querySelector('[role="progressbar"]');
  expect(bar?.getAttribute('aria-label')).toBe('Market cap 100K USDL, all-time high 200K USDL; 50 percent of all-time high');
  expect(bar?.getAttribute('aria-valuenow')).toBe('50');
  expect(bar?.getAttribute('aria-valuemin')).toBe('0');
  expect(bar?.getAttribute('aria-valuemax')).toBe('100');
  expect((bar?.firstElementChild as HTMLElement).style.width).toBe('50%');
  expect(host.textContent).not.toContain('IF_POOL_STATS');
}
test(theBarNamesBothValuationsAndTheirShare.name, theBarNamesBothValuationsAndTheirShare);

async function aLiveHighAppearsImmediatelyWithoutACandleRead() {
  const view = mount();
  await advance();
  view.render({ ...stats, totalSupplyValuationUsdlRaw: '300000000000' });
  expect(athValue(view.host)?.textContent).toBe('300K USDL');
  expect(view.host.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('100');
  expect((view.host.querySelector('[role="progressbar"]')?.firstElementChild as HTMLElement).style.width).toBe('100%');
  view.render(stats);
  expect(athValue(view.host)?.textContent).toBe('300K USDL');
  await advance();
  expect(read).toHaveBeenCalledTimes(1);
}
test(aLiveHighAppearsImmediatelyWithoutACandleRead.name, aLiveHighAppearsImmediatelyWithoutACandleRead);

async function uncoveredPoolsShowMarketCapAlone() {
  for (const creation of [null, new Date(now - 365 * day * 1000).toISOString(), new Date(now - 400 * day * 1000).toISOString()]) {
    const { host } = mount(creation);
    await advance();
    expect(host.textContent).toBe('Market capMC100K USDL');
    expect(host.querySelector('[role="progressbar"], .bg-track, [aria-busy], [role="status"]')).toBeNull();
    expect(athValue(host)).toBeUndefined();
  }
  expect(read).not.toHaveBeenCalled();
}
test(uncoveredPoolsShowMarketCapAlone.name, uncoveredPoolsShowMarketCapAlone);

async function failedReadsShowMarketCapAlone() {
  read.mockResolvedValue({ status: 'unavailable', httpStatus: 503, code: 'DEPENDENCY_UNAVAILABLE', message: 'Candle history unavailable', retryable: false, retryAfterMs: null, requestId: null });
  const { host } = mount();
  await advance();
  expect(host.textContent).toBe('Market capMC100K USDL');
  expect(host.querySelector('[role="progressbar"], .bg-track, [aria-busy], [role="status"]')).toBeNull();
  expect(athValue(host)).toBeUndefined();
  expect(read).toHaveBeenCalledTimes(1);
}
test(failedReadsShowMarketCapAlone.name, failedReadsShowMarketCapAlone);

async function theFigureAndBarStayLoadingUntilTheReadSettles() {
  let resolveRead!: (outcome: CandleHistoryOutcome) => void;
  read.mockReturnValue(new Promise(resolve => { resolveRead = resolve; }));
  const { host } = mount();
  expect(host.textContent).toContain('MC100K USDL');
  expect(athValue(host)?.getAttribute('aria-busy')).toBe('true');
  expect(athValue(host)?.querySelector('.sl-skeleton')).not.toBeNull();
  expect(host.querySelector('[aria-label="Loading all-time high"]')?.getAttribute('aria-busy')).toBe('true');
  expect(host.querySelector('[role="progressbar"]')).toBeNull();
  await act(async () => resolveRead(candles));
  await advance();
  expect(athValue(host)?.textContent).toBe('200K USDL');
  expect(host.querySelector('[aria-busy]')).toBeNull();
}
test(theFigureAndBarStayLoadingUntilTheReadSettles.name, theFigureAndBarStayLoadingUntilTheReadSettles);

async function noCandlesUseTheMarketCapOfTheMoment() {
  read.mockResolvedValue({ status: 'no_data', nextTime: null });
  const view = mount();
  await advance();
  expect(athValue(view.host)?.textContent).toBe('100K USDL');
  expect(view.host.querySelector('[role="progressbar"]')?.getAttribute('aria-valuenow')).toBe('100');
  view.render({ ...stats, totalSupplyValuationUsdlRaw: '80000000000' });
  expect(athValue(view.host)?.textContent).toBe('80K USDL');
  expect(read).toHaveBeenCalledTimes(1);
}
test(noCandlesUseTheMarketCapOfTheMoment.name, noCandlesUseTheMarketCapOfTheMoment);

async function theTrackKeepsASliverForSmallAndZeroShares() {
  const view = mount();
  await advance();
  view.render({ ...stats, totalSupplyValuationUsdlRaw: '1000000' });
  const bar = view.host.querySelector('[role="progressbar"]');
  expect(bar?.getAttribute('aria-label')).toBe('Market cap 1.00 USDL, all-time high 200K USDL; less than 0.01 percent of all-time high');
  expect(Number(bar?.getAttribute('aria-valuenow'))).toBeCloseTo(0.0005);
  expect((bar?.firstElementChild as HTMLElement).style.width).toBe('2%');
  view.render({ ...stats, totalSupplyValuationUsdlRaw: '0' });
  expect(bar?.getAttribute('aria-valuenow')).toBe('0');
  expect(bar?.getAttribute('aria-label')).toContain('0 percent');
  expect((bar?.firstElementChild as HTMLElement).style.width).toBe('2%');
}
test(theTrackKeepsASliverForSmallAndZeroShares.name, theTrackKeepsASliverForSmallAndZeroShares);

async function candleHistoryIsCachedPerPoolAndRefreshedOnceAMinute() {
  const view = mount();
  await advance();
  const another = mount(createdAt, view.client);
  await advance();
  expect(athValue(another.host)?.textContent).toBe('200K USDL');
  expect(read).toHaveBeenCalledTimes(1);
  await advance(59_000);
  expect(read).toHaveBeenCalledTimes(1);
  await advance(1_010);
  expect(read).toHaveBeenCalledTimes(2);
  await advance(60_000);
  expect(read).toHaveBeenCalledTimes(3);
  const otherPool = '0x' + 'b'.repeat(40);
  const snapshot = view.client.getQueryData<PoolSnapshot>(queryKeys.poolSnapshot(pool))!;
  view.client.setQueryData(queryKeys.poolSnapshot(otherPool), { ...snapshot, pool: { ...snapshot.pool, poolAddress: otherPool }, stats: { ...stats, poolAddress: otherPool } });
  view.render(null, otherPool);
  await advance();
  expect(read).toHaveBeenCalledTimes(4);
  expect(read).toHaveBeenLastCalledWith(otherPool, '1D', from, Math.floor(Date.now() / 1000), { signal: expect.any(AbortSignal) });
}
test(candleHistoryIsCachedPerPoolAndRefreshedOnceAMinute.name, candleHistoryIsCachedPerPoolAndRefreshedOnceAMinute);

function HistoryReader({ inView }: { inView: boolean }) {
  const history = useAllTimeHigh(pool, createdAt, inView);
  return <output>{history.data?.status}</output>;
}

async function historyRefreshPausesOutsideTheView() {
  const client = new QueryClient();
  const host = document.createElement('div');
  document.body.append(host);
  const root = createRoot(host);
  mounted.push({ client, host, root });
  const render = (inView: boolean) => act(() => root.render(<QueryClientProvider client={client}><HistoryReader inView={inView} /></QueryClientProvider>));
  render(false);
  await advance(60_000);
  expect(read).not.toHaveBeenCalled();
  render(true);
  await advance();
  expect(read).toHaveBeenCalledTimes(1);
  expect(host.textContent).toBe('candles');
  render(false);
  await advance(120_000);
  expect(read).toHaveBeenCalledTimes(1);
  render(true);
  await advance();
  expect(read).toHaveBeenCalledTimes(2);
}
test(historyRefreshPausesOutsideTheView.name, historyRefreshPausesOutsideTheView);
