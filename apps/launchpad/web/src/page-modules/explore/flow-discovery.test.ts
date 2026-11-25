import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { act, createElement, type ReactNode } from 'react';
import { afterAll, afterEach, expect, test } from 'vitest';
import { SdkClient, decodeV2Contract } from '@kindle/sdk';
import { serveRecorded, v2Reads } from '@/core/test-fixtures/market-production/v2-reads';
import type { ExploreRowsState } from '@/widgets/explore/use-explore-rows';
import type { WatchlistRows } from '@/core/market/use-watchlist-rows';
import type { TokenMetadata } from '@/widgets/home/types';
import metadataRecording from '@/widgets/my-token/creator-metadata.recording.json';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000/explore', pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const configurationPath = process.env.JOINED_CONFIG_FILE;
if (!configurationPath) throw new Error('Discovery services are unavailable');
const joined = JSON.parse(readFileSync(configurationPath, 'utf8')) as {
  api_origin: string; rpc_url: string; processes: { role: string; ready_url: string }[];
};
const gateway = joined.processes.find(service => service.role === 'gateway');
if (!gateway) throw new Error('The watchlist service is unavailable');
process.env.NEXT_PUBLIC_DATA_API_URL = joined.api_origin;
process.env.NEXT_PUBLIC_GATEWAY_URL = new URL(gateway.ready_url).origin;
process.env.NEXT_PUBLIC_METADATA_API_URL = new URL(gateway.ready_url).origin;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { WagmiProvider, createConfig, http } = await import('wagmi');
const { paxeer } = await import('@/core/wagmi-config');
const { MarketClientContext } = await import('@/core/market/client');
const { useExploreRows, buildExploreRow } = await import('@/widgets/explore/use-explore-rows');
const { useWatchlistRows } = await import('@/core/market/use-watchlist-rows');
const { useWatchlist } = await import('@/hooks/ui/use-watchlist');
const { queryKeys } = await import('@/core/query-keys');
const { SourceNotice } = await import('@/ui/common/source-notice');
const { WatchlistNotice } = await import('@/ui/common/watchlist-notice');
const { default: WatchlistModule } = await import('@/page-modules/watchlist');
const { WatchlistWidget } = await import('@/widgets/home/watchlist-widget');
const { TrendingWidget, trendingMetadata, toTrendingRow } = await import('@/widgets/home/trending-widget');
const { ScreenerWidget } = await import('@/widgets/home/screener-widget');
const { refreshWatchlist } = await import('@/widgets/profile/watchlist-state');

const wallet = createConfig({ chains: [paxeer], connectors: [], storage: null, transports: { [paxeer.id]: http(joined.rpc_url) }, multiInjectedProviderDiscovery: false });
const cleanup: Array<() => Promise<void>> = [];
const { real, stateless } = v2Reads.pools;

function marketClient(origin: string) {
  const stream = origin.replace(/^http/, 'ws');
  return new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: stream, approvedStreamOrigins: [stream], production: false });
}

function mount(node: ReactNode, origin = joined.api_origin) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => root.render(createElement(WagmiProvider, { config: wallet },
    createElement(QueryClientProvider, { client: queryClient },
      createElement(MarketClientContext.Provider, { value: marketClient(origin) }, node)))));
  cleanup.push(async () => {
    act(() => root.unmount());
    await queryClient.cancelQueries();
    queryClient.clear();
    container.remove();
  });
  return { container, queryClient };
}

async function waitFor(predicate: () => boolean): Promise<void> {
  const deadline = Date.now() + 10000;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error(`Discovery did not settle: ${document.body.textContent}`);
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
  }
}

function button(container: HTMLElement, label: string): HTMLButtonElement {
  const result = Array.from(container.querySelectorAll('button')).find(item => item.textContent === label);
  if (!result) throw new Error(`Missing action: ${label}`);
  return result;
}

function ExploreProbe({ category = 'trending', observe }: { category?: 'trending' | 'new'; observe: (state: ExploreRowsState) => void }) {
  const state = useExploreRows(category);
  observe(state);
  return createElement('div', { 'aria-busy': state.loading || state.enrichmentPending },
    ...state.rows.map(row => createElement('a', { key: row.poolAddress, href: row.href }, row.name)),
    createElement(SourceNotice, { label: 'Pool details', state: state.listings, onRetry: state.retrySources }),
    createElement(SourceNotice, { label: 'Coin stats', state: state.stats, onRetry: state.retrySources }));
}

function WatchlistProbe({ observe }: { observe: (state: WatchlistRows) => void }) {
  const state = useWatchlistRows();
  observe(state);
  return null;
}

function WatchActions() {
  const state = useWatchlist();
  return createElement(WatchlistNotice, { state });
}

afterEach(async () => {
  for (const close of cleanup.splice(0).reverse()) await close();
  localStorage.clear();
  await refreshWatchlist();
});
afterAll(() => dom.window.close());

async function discoveryRefreshKeepsAnUnchangedRankingActionable() {
  const replay = await serveRecorded([
    v2Reads.responses.rankingsTrending, v2Reads.responses.statsBatch,
    v2Reads.responses.poolListTrending, v2Reads.responses.poolDetailReal, v2Reads.responses.poolDetailFirst,
  ]);
  cleanup.push(() => replay.close());
  let current: ExploreRowsState | undefined;
  const view = mount(createElement(ExploreProbe, { observe: state => { current = state; } }), replay.origin);
  await waitFor(() => current?.rows.length === 2 && !current.loading && !current.enrichmentPending);
  const snapshot = current!.snapshot;
  const count = replay.requests.filter(path => path === 'GET /v2/rankings/trending').length;
  await act(async () => { current!.reload(); });
  await waitFor(() => replay.requests.filter(path => path === 'GET /v2/rankings/trending').length === count + 1 && view.queryClient.isFetching() === 0);
  expect(current!.snapshot).toBe(snapshot);
  expect(current!.loading).toBe(false);
  expect(current!.ranking.state).toBe('ready');
  expect(view.container.querySelectorAll('a')).toHaveLength(2);
  expect(current!.rows.find(row => row.poolAddress === real)?.mcapRaw).toBe('10098226981');
}

async function discoveryListsEverySavedPoolInBoundedStatsBatches() {
  const pools = [real, ...Array.from({ length: 50 }, (_, index) => `0x${(index + 1).toString(16).padStart(40, '0')}`)];
  localStorage.setItem('sidiora_watchlist', JSON.stringify(pools));
  let current: WatchlistRows | undefined;
  const view = mount(createElement('div', null,
    createElement(WatchlistProbe, { observe: state => { current = state; } }),
    createElement(WatchlistModule)));
  await waitFor(() => current?.rows.length === pools.length && current.stats.state !== 'loading' && current.listings.state !== 'loading');
  expect(current!.pools).toEqual(pools);
  expect(view.container.textContent).toContain('51 starred coins');
  for (const pool of pools) expect(view.container.querySelector(`a[href="/coin/${pool}"]`)).not.toBeNull();
  const batches = view.queryClient.getQueryCache().findAll().filter(query => query.queryKey.includes('stats-batch'));
  expect(batches).toHaveLength(2);
  expect(batches.map(query => query.queryKey.slice(3).length).sort((a, b) => a - b)).toEqual([1, 50]);
  expect(batches.every(query => query.state.fetchStatus === 'idle')).toBe(true);
}

async function discoveryWatchlistReadFailureHasAVisibleRecovery() {
  localStorage.setItem('sidiora_watchlist', '{');
  const view = mount(createElement(WatchlistModule));
  await waitFor(() => view.container.textContent?.includes("Couldn't load your saved watchlist on this device") === true);
  expect(view.container.textContent).not.toContain('Your watchlist is empty');
  localStorage.setItem('sidiora_watchlist', '[]');
  act(() => button(view.container, 'Refresh watchlist').click());
  await waitFor(() => view.container.textContent?.includes('Your watchlist is empty') === true);
  expect(view.container.textContent).not.toContain("Couldn't load your saved watchlist on this device");
  expect(view.container.querySelector('a[href="/explore"]')?.textContent).toBe('Browse coins');
}

async function discoveryHomeWatchlistReadFailureHasAVisibleRecovery() {
  localStorage.setItem('sidiora_watchlist', '{');
  const view = mount(createElement(WatchlistWidget));
  await waitFor(() => view.container.textContent?.includes("Couldn't load your saved watchlist on this device") === true);
  expect(view.container.textContent).not.toContain('Your watchlist is empty');
  localStorage.setItem('sidiora_watchlist', '[]');
  act(() => button(view.container, 'Refresh watchlist').click());
  await waitFor(() => view.container.textContent?.includes('Your watchlist is empty') === true);
}

async function discoveryStarReportsStorageFailureAndKeepsTheChoiceVisible() {
  const view = mount(createElement(WatchActions));
  await act(async () => { await refreshWatchlist(); });
  localStorage.setItem('sidiora_watchlist', JSON.stringify(Array.from({ length: 500 }, (_, index) => `0x${(index + 1).toString(16).padStart(40, '0')}`)));
  const { mutateWatch } = await import('@/widgets/profile/watchlist-state');
  await act(async () => {
    await expect(mutateWatch(real, true)).rejects.toThrow('Frozen watchlist capacity500');
  });
  expect(view.container.textContent).toContain('Your watchlist is full (500 coins)');
  expect(JSON.parse(localStorage.getItem('sidiora_watchlist')!)).toHaveLength(500);
  // Removing a saved coin remains available at capacity.
  await act(async () => { await mutateWatch(`0x${'1'.padStart(40, '0')}`, false); });
  expect(JSON.parse(localStorage.getItem('sidiora_watchlist')!)).toHaveLength(499);
  expect(view.container.textContent).not.toContain('waiting for confirmation');
}

async function discoveryShowsPoolLookupFailureWithoutDroppingTheRankedCoins() {
  const replay = await serveRecorded([v2Reads.responses.rankingsTrending, v2Reads.responses.statsBatch]);
  cleanup.push(() => replay.close());
  let current: ExploreRowsState | undefined;
  const view = mount(createElement(ExploreProbe, { observe: state => { current = state; } }), replay.origin);
  await waitFor(() => current?.rows.length === 2 && current.listings.state === 'unsupported');
  expect(view.container.textContent).toContain('Pool details not available yet');
  expect(current!.loading).toBe(false);
  const before = replay.requests.length;
  expect(view.container.querySelector('button')).toBeNull();
  await act(async () => { await new Promise(resolve => setTimeout(resolve, 50)); });
  expect(replay.requests).toHaveLength(before);
  expect(view.container.querySelector(`a[href="/coin/${real}"]`)).not.toBeNull();
}

async function discoveryWatchlistStatsFailureKeepsRowsAndCanRetry() {
  localStorage.setItem('sidiora_watchlist', JSON.stringify([real, stateless]));
  const replay = await serveRecorded([
    v2Reads.responses.statsBatchUnavailable, v2Reads.responses.poolDetailReal, v2Reads.responses.poolDetailUnavailable,
  ]);
  cleanup.push(() => replay.close());
  const view = mount(createElement(WatchlistModule), replay.origin);
  await waitFor(() => view.container.textContent?.includes('Coin stats not available yet') === true);
  expect(view.container.querySelector(`a[href="/coin/${real}"]`)).not.toBeNull();
  expect(view.container.querySelector(`a[href="/coin/${stateless}"]`)).not.toBeNull();
  const before = replay.requests.filter(path => path === 'POST /v2/pools/stats:batch').length;
  act(() => button(view.container, 'Try again').click());
  await waitFor(() => replay.requests.filter(path => path === 'POST /v2/pools/stats:batch').length === before + 1 && view.queryClient.isFetching() === 0);
  expect(view.container.textContent).not.toContain('Your watchlist is empty');
}

async function discoveryStatsConnectionLossCanRecoverFromTheView() {
  localStorage.setItem('sidiora_watchlist', JSON.stringify([real]));
  const replay = await serveRecorded([v2Reads.responses.statsBatchReal, v2Reads.responses.poolDetailReal]);
  cleanup.push(() => replay.close());
  let current: WatchlistRows | undefined;
  const view = mount(createElement('div', null,
    createElement(WatchlistProbe, { observe: state => { current = state; } }),
    createElement(WatchlistModule)), replay.origin);
  await waitFor(() => current?.stats.state === 'ready' && current.listings.state === 'ready' && view.queryClient.isFetching() === 0);
  await replay.close();
  await act(async () => { await view.queryClient.resetQueries({ queryKey: queryKeys.statsBatch(replay.origin, [real]) }); });
  await waitFor(() => current?.stats.state === 'error');
  expect(view.container.textContent).toContain('Coin stats:');
  expect(view.container.querySelector(`a[href="/coin/${real}"]`)).not.toBeNull();
  const errorNotice = Array.from(view.container.querySelectorAll('[role="status"]')).find(element => element.textContent?.includes('Coin stats:'));
  expect(errorNotice?.querySelector('button')?.textContent).toBe('Try again');
  await new Promise<void>(resolve => replay.server.listen(Number(new URL(replay.origin).port), '127.0.0.1', resolve));
  act(() => errorNotice!.querySelector<HTMLButtonElement>('button')!.click());
  await waitFor(() => current?.stats.state === 'ready' && view.queryClient.isFetching() === 0);
  expect(current!.rows[0].stats?.totalSupplyValuationUsdlRaw).toBe('10098226981');
  expect(view.container.textContent).not.toContain('Coin stats:');
}

async function discoveryHomeSourcesExposeTheirRetryActions() {
  const replay = await serveRecorded([v2Reads.responses.rankingsTrending, v2Reads.responses.rankingsNew, v2Reads.responses.statsBatch]);
  cleanup.push(() => replay.close());
  for (const Widget of [TrendingWidget, ScreenerWidget]) {
    const view = mount(createElement(Widget, { columnId: 'discovery', filter: '', density: 'comfortable', view: 'list' }), replay.origin);
    if (Widget === ScreenerWidget) act(() => {
      const movers = Array.from(view.container.querySelectorAll('button')).find(item => item.textContent === 'Movers');
      expect(movers).toBeDefined();
    });
    await waitFor(() => view.container.textContent?.includes('Pool details not available yet') === true && view.queryClient.isFetching() === 0);
    const unavailable = Array.from(view.container.querySelectorAll('[role="status"]')).find(element => element.textContent?.includes('Pool details not available yet'));
    expect(unavailable).toBeDefined();
    expect(unavailable!.querySelector('button')).toBeNull();
    expect(view.container.querySelector(`a[href="/coin/${real}"]`)).not.toBeNull();
  }
}

function discoveryUsesTheTokenIdentityForTrendingMetadata() {
  const recorded = metadataRecording[0].body as TokenMetadata;
  const token = recorded.token_address!;
  const row = buildExploreRow({ poolAddress: recorded.pool_address!, rank: 1, stats: null, metadata: recorded, now: 1700001200000 });
  expect(trendingMetadata(row, { [token]: recorded })).toBe(recorded);
  expect(trendingMetadata({ tokenAddress: null }, { [token]: recorded })).toBeNull();
  expect(toTrendingRow(row, '24h', trendingMetadata(row, { [token]: recorded })).name).toBe(recorded.name);
}

function discoveryReadsMetadataCreationTimeInMilliseconds() {
  const recorded = metadataRecording[0].body as TokenMetadata;
  const row = buildExploreRow({ poolAddress: recorded.pool_address!, rank: 1, stats: null, metadata: recorded, now: 1700001200000 });
  expect(row.createdAtMs).toBe(1700000000000);
  expect(row.ageMs).toBe(1200000);
  expect(row.stage).toBe('new');
  expect(buildExploreRow({ poolAddress: row.poolAddress, rank: 1, stats: null, metadata: recorded, now: 1700172800000 }).stage).toBeNull();
}

async function discoveryJoinedReadsReachContentOrExplicitSourceStates() {
  let current: ExploreRowsState | undefined;
  const view = mount(createElement(ExploreProbe, { category: 'new', observe: state => { current = state; } }));
  await waitFor(() => current?.ranking.state !== 'loading' && current?.loading === false && !current.enrichmentPending);
  expect(current!.ranking.state).toBe('ready');
  expect(current!.rows.length).toBeGreaterThan(0);
  expect(current!.rows.every(row => row.href === `/coin/${row.poolAddress}`)).toBe(true);
  const response = await marketClient(joined.api_origin).requestV2('/v2/discovery/feed?order=chronological&limit=50', 'ConnectedDiscoveryFeedPage');
  expect(decodeV2Contract('ConnectedDiscoveryFeedPage', response).Entries).toEqual(response.Entries);
  expect(view.container.querySelector('[aria-busy]')?.getAttribute('aria-busy')).toBe('false');
}

for (const scenario of [
  discoveryRefreshKeepsAnUnchangedRankingActionable,
  discoveryListsEverySavedPoolInBoundedStatsBatches,
  discoveryWatchlistReadFailureHasAVisibleRecovery,
  discoveryHomeWatchlistReadFailureHasAVisibleRecovery,
  discoveryStarReportsStorageFailureAndKeepsTheChoiceVisible,
  discoveryShowsPoolLookupFailureWithoutDroppingTheRankedCoins,
  discoveryWatchlistStatsFailureKeepsRowsAndCanRetry,
  discoveryStatsConnectionLossCanRecoverFromTheView,
  discoveryHomeSourcesExposeTheirRetryActions,
  discoveryUsesTheTokenIdentityForTrendingMetadata,
  discoveryReadsMetadataCreationTimeInMilliseconds,
  discoveryJoinedReadsReachContentOrExplicitSourceStates,
]) test(scenario.name, scenario, 30000);
