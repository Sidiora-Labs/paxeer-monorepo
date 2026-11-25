import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { createRequire } from 'node:module';
import { readFileSync } from 'node:fs';
import { setTimeout as pause } from 'node:timers/promises';
import React, { act, useSyncExternalStore } from 'react';
import { afterAll, afterEach, expect, test, vi } from 'vitest';
import { HttpError, SdkClient, decodeV2Contract, type StreamTransportV2, type V2ContractTypes } from '@kindle/sdk';
import { MarketSession } from './market-session';

interface BrowserDom { window: Window & typeof globalThis; reconfigure(options: { url: string }): void }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
browserGlobals.Event = dom.window.Event;
browserGlobals.CustomEvent = dom.window.CustomEvent;
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

type TopicSnapshot = V2ContractTypes['TopicSnapshot'];
type Channel = Parameters<StreamTransportV2['connect']>[0];
const wire = JSON.parse(readFileSync(new URL('../../core/test-fixtures/market-production/core-api-wire.json', import.meta.url), 'utf8')) as {
  snapshot: { body: V2ContractTypes['TopicSnapshotReadResponse'] };
  candles: { body: V2ContractTypes['CandlesResponse'] };
  trades: { body: V2ContractTypes['TradesResponse'] };
  v2Reads: { responses: { poolDetailReal: { body: V2ContractTypes['PoolDetailResponse'] } } };
};
const recorded = decodeV2Contract('TopicSnapshot', wire.snapshot.body.data.snapshot);
const statsEntry = recorded.topics.find(entry => entry.topic.startsWith('market.stats.'))!;
const tradedStats = decodeV2Contract('PoolStats', statsEntry.payload);
const pool = tradedStats.poolAddress;
const topics = ['trades', 'price', 'stats'].map(kind => `market.${kind}.${pool}`);
const noTradeStats = decodeV2Contract('PoolStats', { ...tradedStats, buys24h: '0', sells24h: '0', txCount24h: '0', uniqueTraders24h: '0', volume24hRaw: '0', realUsdlBalanceRaw: '0' });
const statsSnapshot = decodeV2Contract('TopicSnapshot', { ...recorded, topics: [{ ...statsEntry, payload: noTradeStats }] });
const completeSnapshot = decodeV2Contract('TopicSnapshot', {
  ...recorded,
  topics: recorded.topics.map(entry => entry.topic === statsEntry.topic ? { ...entry, revision: (BigInt(entry.revision) + 1n).toString() } : entry),
});
const candle = decodeV2Contract('Candle', wire.candles.body.data.items[0]);
let candleAvailable = false;
const httpReads: { path: string; at: number; status: number }[] = [];
const server = createServer((request, response) => {
  const path = request.url ?? '/';
  response.setHeader('Content-Type', 'application/json');
  response.setHeader('X-Schema-Version', '2');
  if (path.startsWith(`/v2/pools/${pool}/candles`)) {
    httpReads.push({ path, at: Date.now(), status: 200 });
    response.end(JSON.stringify(candleAvailable ? wire.candles.body : { ...wire.candles.body, watermark: null, revision: null, nextCursor: null, data: { ...wire.candles.body.data, items: [] } }));
  } else if (path.startsWith(`/v2/pools/${pool}/trades`)) {
    httpReads.push({ path, at: Date.now(), status: 200 });
    response.end(JSON.stringify(candleAvailable ? wire.trades.body : { ...wire.trades.body, watermark: null, revision: null, nextCursor: null, data: { items: [] } }));
  } else {
    httpReads.push({ path, at: Date.now(), status: 404 });
    response.writeHead(404); response.end();
  }
});
const origin = await new Promise<string>(resolve => server.listen(0, '127.0.0.1', () => resolve(`http://127.0.0.1:${(server.address() as AddressInfo).port}`)));
dom.reconfigure({ url: origin });

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { createConfig, http, WagmiProvider } = await import('wagmi');
const { paxeer } = await import('@/core/wagmi-config');
const { MarketClientProvider } = await import('./market-session-context');
const { MarketStats } = await import('./market-metrics');
const { MarketTradeFeed } = await import('./market-trade-feed');
const { default: TokenHeader } = await import('@/widgets/coin/token-header');
const { FreshnessNotice } = await import('@/widgets/coin/freshness-notice');
const { default: TvChart } = await import('@/widgets/trading/tv-chart');
const { createReplayUdfDatafeed, registerChartReplayTransport, chartReplayListenerCount } = await import('@/widgets/trading/udf-datafeed');
const { queryKeys } = await import('@/core/query-keys');
const { statsSpotPrice, formatSpotPrice, noTradePriceReason } = await import('@/core/market/pool-stats-display');
const { defaultChartSettings, updateChartSettings } = await import('@/widgets/coin/use-chart-settings');

function client() {
  const stream = origin.replace(/^http/, 'ws');
  return new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: stream, approvedStreamOrigins: [stream], production: false });
}

function marketTransport(read: (call: number) => unknown | Promise<unknown>) {
  const reads: { topics: string[]; at: number }[] = [];
  const channels: Channel[] = [];
  const subscriptions: V2ContractTypes['Subscribe'][] = [];
  let closes = 0;
  const transport: StreamTransportV2 = {
    snapshot: async requested => { reads.push({ topics: [...requested], at: performance.now() }); return read(reads.length); },
    connect: async input => {
      channels.push(input);
      return { send: frame => subscriptions.push(frame), close: () => { closes++; } };
    },
  };
  const control = (op: V2ContractTypes['StreamControl']['op'], cursor: string | null = null) => {
    channels.at(-1)!.receive(decodeV2Contract('StreamControl', { op, subscriptionId: subscriptions.at(-1)!.subscriptionId, cursor, code: op === 'resnapshot_required' ? 'RESUME_INVALID' : null, snapshotUrl: op === 'resnapshot_required' ? '/v2/snapshots' : null, watermark: null }));
  };
  const live = () => { control('subscribed'); control('checkpoint', 'market-checkpoint.cursor'); };
  return { transport, reads, channels, subscriptions, control, live, get closes() { return closes; } };
}

async function waitFor(check: () => boolean, timeoutMs = 1000) {
  const deadline = performance.now() + timeoutMs;
  while (!check()) {
    if (performance.now() >= deadline) throw new Error('Market state did not arrive in time');
    await pause(5);
    if (vi.isFakeTimers()) await vi.advanceTimersByTimeAsync(0);
  }
}

afterEach(() => {
  candleAvailable = false;
  httpReads.length = 0;
  updateChartSettings(defaultChartSettings);
  vi.useRealTimers();
});
afterAll(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
});

async function marketFirstStateInstallsStatsWhenTradeOrPriceHasNotStarted() {
  for (const missing of [[topics[0], topics[1]], [topics[0]], [topics[1]]]) {
    const snapshot = { ...completeSnapshot, topics: completeSnapshot.topics.filter(entry => !missing.includes(entry.topic)) };
    const transport = marketTransport(() => snapshot);
    const session = new MarketSession(client(), pool, transport.transport);
    try {
      await session.start();
      const state = session.getSnapshot();
      expect(state.noTradesYet).toBe(true);
      expect(state.stats).toEqual(tradedStats);
      expect(state.rows).toEqual([]);
      expect(state.price).toBeNull();
      expect(state.priceSequence).toBeNull();
      expect(state.priceRevision).toBeNull();
      expect(state.watermark).toEqual(statsEntry.watermark);
      expect(state.generatedAt).toBe(snapshot.generatedAt);
      expect(state.statsGeneratedAt).toBe(snapshot.generatedAt);
      expect(state.revision).toBe(snapshot.topics.find(entry => entry.topic === statsEntry.topic)!.revision);
      expect(state.sequence).toBe(statsEntry.sequence);
      expect(transport.subscriptions[0].topics).toEqual(topics);
      expect(transport.subscriptions[0].afterCursor).toBe(snapshot.resumeCursor);
      transport.live();
      expect(session.getSnapshot().stream).toBe('live');
      expect(session.getSnapshot().error).toBeNull();
    } finally { session.close(); }
  }
}
test(marketFirstStateInstallsStatsWhenTradeOrPriceHasNotStarted, marketFirstStateInstallsStatsWhenTradeOrPriceHasNotStarted);

async function marketFirstStateRefusesSnapshotWithoutStats() {
  const transport = marketTransport(() => ({ ...completeSnapshot, topics: completeSnapshot.topics.filter(entry => entry.topic !== statsEntry.topic) }));
  const session = new MarketSession(client(), pool, transport.transport);
  try {
    await session.start();
    expect(session.getSnapshot().noTradesYet).toBe(false);
    expect(session.getSnapshot().stats).toBeNull();
    expect(session.getSnapshot().rows).toEqual([]);
    expect(session.getSnapshot().price).toBeNull();
    expect(session.getSnapshot().error).toBe('Incomplete market snapshot');
    expect(transport.subscriptions).toEqual([]);
  } finally { session.close(); }
}
test(marketFirstStateRefusesSnapshotWithoutStats, marketFirstStateRefusesSnapshotWithoutStats);

async function marketFirstStateTurnsLiveWithTheFirstTradeWithoutAnIntermediateFailure() {
  let release: ((snapshot: TopicSnapshot) => void) | undefined;
  const transport = marketTransport(call => call === 1 ? statsSnapshot : new Promise<TopicSnapshot>(resolve => { release = resolve; }));
  const session = new MarketSession(client(), pool, transport.transport);
  const states: ReturnType<MarketSession['getSnapshot']>[] = [];
  const unsubscribe = session.subscribe(() => states.push(session.getSnapshot()));
  try {
    await session.start(); transport.live();
    const first = session.getSnapshot();
    states.length = 0;
    const startedAt = performance.now();
    transport.control('resnapshot_required');
    await waitFor(() => transport.reads.length === 2);
    expect(transport.reads[1].at - startedAt).toBeLessThan(1000);
    expect(session.getSnapshot()).toBe(first);
    expect(transport.closes).toBe(1);
    release!(completeSnapshot);
    await waitFor(() => transport.subscriptions.length === 2);
    transport.live();
    expect(session.getSnapshot()).toMatchObject({ noTradesYet: false, stream: 'live', error: null });
    expect(session.getSnapshot().rows).toEqual([decodeV2Contract('Trade', completeSnapshot.topics.find(entry => entry.topic === topics[0])!.payload)]);
    expect(session.getSnapshot().price).toEqual(decodeV2Contract('Price', completeSnapshot.topics.find(entry => entry.topic === topics[1])!.payload));
    expect(states.map(state => state.stream)).not.toContain('resync-required');
    expect(states.every(state => state.stream === 'live' && state.error === null)).toBe(true);
    expect(states.some(state => state.noTradesYet)).toBe(false);
    expect(session.getDiagnostics().timers).toBe(0);
  } finally { unsubscribe(); session.close(); }
}
test(marketFirstStateTurnsLiveWithTheFirstTradeWithoutAnIntermediateFailure, marketFirstStateTurnsLiveWithTheFirstTradeWithoutAnIntermediateFailure);

async function marketFirstStateRefreshesBeyondThreeWaitingTopicHandoffs() {
  const transport = marketTransport(call => call <= 6 ? statsSnapshot : completeSnapshot);
  const session = new MarketSession(client(), pool, transport.transport);
  const states: string[] = [];
  const unsubscribe = session.subscribe(() => states.push(session.getSnapshot().stream));
  try {
    await session.start(); transport.live(); states.length = 0;
    for (let handoff = 1; handoff <= 6; handoff++) {
      const startedAt = performance.now();
      transport.control('resnapshot_required');
      await waitFor(() => transport.subscriptions.length === handoff + 1);
      expect(transport.reads[handoff].at - startedAt).toBeLessThan(1000);
      transport.live();
      expect(session.getSnapshot().noTradesYet).toBe(handoff < 6);
      expect(session.getSnapshot().stream).toBe('live');
      expect(session.getDiagnostics().timers).toBe(0);
    }
    expect(states.every(state => state === 'live')).toBe(true);
    expect(transport.reads).toHaveLength(7);
  } finally { unsubscribe(); session.close(); }
}
test(marketFirstStateRefreshesBeyondThreeWaitingTopicHandoffs, marketFirstStateRefreshesBeyondThreeWaitingTopicHandoffs);

async function marketFirstStateKeepsNormalResyncDelayForTradedPools() {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  const transport = marketTransport(() => completeSnapshot);
  const session = new MarketSession(client(), pool, transport.transport);
  try {
    await session.start(); transport.live();
    transport.control('resnapshot_required');
    expect(session.getSnapshot().stream).toBe('resync-required');
    expect(transport.reads).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(499);
    expect(transport.reads).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(transport.reads).toHaveLength(2);
    transport.live();
    expect(session.getSnapshot().stream).toBe('live');
  } finally { session.close(); }
}
test(marketFirstStateKeepsNormalResyncDelayForTradedPools, marketFirstStateKeepsNormalResyncDelayForTradedPools);

async function marketFirstStateRefusesForeignHandoffsAndIgnoresClosedChannels() {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  const transport = marketTransport(() => statsSnapshot);
  const session = new MarketSession(client(), pool, transport.transport);
  try {
    await session.start(); transport.live();
    transport.channels[0].receive({ op: 'resnapshot_required', subscriptionId: 'another-market', cursor: null, code: 'RESUME_INVALID', snapshotUrl: '/v2/snapshots', watermark: null });
    expect(session.getSnapshot().stream).toBe('resync-required');
    expect(session.getSnapshot().error).toBe('Foreign subscription control');
    expect(transport.reads).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(500);
    transport.live();
    const state = session.getSnapshot();
    const reads = transport.reads.length;
    transport.channels[0].receive({ op: 'resnapshot_required', subscriptionId: transport.subscriptions[0].subscriptionId, cursor: null, code: 'RESUME_INVALID', snapshotUrl: '/v2/snapshots', watermark: null });
    expect(session.getSnapshot()).toBe(state);
    expect(transport.reads).toHaveLength(reads);
  } finally { session.close(); }
}
test(marketFirstStateRefusesForeignHandoffsAndIgnoresClosedChannels, marketFirstStateRefusesForeignHandoffsAndIgnoresClosedChannels);

async function marketFirstStateUsesPoolDetailDuringLegacyTopicWait() {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
  const transport = marketTransport(call => {
    if (call === 1) throw new HttpError(404, { code: 'TOPIC_NOT_STARTED', message: 'Topic has no first frame', requestId: '11111111-1111-4111-8111-111111111111', retryable: true, retryAfterMs: 7000, fieldErrors: [], details: null });
    return statsSnapshot;
  });
  const session = new MarketSession(client(), pool, transport.transport);
  session.setPoolDetail(noTradeStats, statsSnapshot.generatedAt);
  try {
    const started = session.start();
    await waitFor(() => session.getSnapshot().noTradesYet);
    expect(session.getSnapshot()).toMatchObject({ stats: noTradeStats, rows: [], price: null, error: null, watermark: noTradeStats.watermark, generatedAt: statsSnapshot.generatedAt });
    expect(transport.reads).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(6999);
    expect(transport.reads).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    await started;
    expect(transport.reads).toHaveLength(2);
    expect(transport.subscriptions[0].topics).toEqual(topics);
    expect(session.getSnapshot().noTradesYet).toBe(true);
  } finally { session.close(); }
}
test(marketFirstStateUsesPoolDetailDuringLegacyTopicWait, marketFirstStateUsesPoolDetailDuringLegacyTopicWait);

function FirstMarketView({ session }: { session: MarketSession }) {
  const state = useSyncExternalStore(session.subscribe, session.getSnapshot, session.getSnapshot);
  const detail = wire.v2Reads.responses.poolDetailReal.body.data;
  return React.createElement(React.Fragment, null,
    React.createElement(TokenHeader, { poolAddress: pool, liveStats: state.stats, livePrice: null, now: Date.now(), notice: React.createElement(FreshnessNotice, { freshness: session.status(Date.now()), stream: state.stream, error: state.error, noTradesYet: state.noTradesYet, onReconnect: () => { void session.start(); } }) }),
    React.createElement(MarketStats, { poolAddress: pool, stats: state.stats }),
    React.createElement(TvChart, { poolAddress: pool, market: session }),
    React.createElement(MarketTradeFeed, { pool: detail.pool, token: detail.token, rows: state.rows, watermark: state.watermark, contiguous: state.stream === 'live' }),
  );
}

async function marketFirstStateRendersStatsSpotPriceAndNeutralEmptyViews() {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
  vi.setSystemTime(Date.parse(candle.start) + 30000);
  updateChartSettings({ interval: '1', mode: 'price' });
  const queries = new QueryClient();
  const detail = wire.v2Reads.responses.poolDetailReal.body;
  queries.setQueryData(queryKeys.poolSnapshot(pool), { pool: { chainId: '125', poolAddress: pool, tokenAddress: detail.data.token.address }, token: detail.data.token, stats: noTradeStats, metadata: null, creator: detail.data.pool.creator, createdAt: null, generatedAt: detail.generatedAt, resolvedBy: 'detail', unavailable: { stats: null, metadata: null, registry: null } });
  queries.setQueryData(queryKeys.tokenMetadata(detail.data.token.address), null);
  queries.setQueryData(queryKeys.trades(origin, pool, null), { ...wire.trades.body, watermark: null, revision: null, nextCursor: null, data: { items: [] } });
  const transport = marketTransport(() => statsSnapshot);
  const session = new MarketSession(client(), pool, transport.transport);
  const config = createConfig({ chains: [paxeer], transports: { 125: http(origin) }, ssr: true });
  const container = document.createElement('div'); document.body.append(container);
  const root = createRoot(container);
  try {
    await session.start(); transport.live();
    await act(async () => root.render(React.createElement(WagmiProvider, { config }, React.createElement(QueryClientProvider, { client: queries }, React.createElement(MarketClientProvider, { client: client(), children: React.createElement(FirstMarketView, { session }) })))));
    await act(async () => waitFor(() => httpReads.some(read => read.path.includes('/candles'))));
    expect(container.querySelector('[aria-label="Market statistics"]')?.textContent).toContain('Volume · USDL · 24h0');
    expect(container.querySelector('[aria-label="Trade history"]')?.textContent).toContain('No trades yet');
    expect(container.querySelector('[aria-label="Price chart"]')?.textContent).toContain('No trades yet');
    expect(container.querySelector('[data-freshness-chip]')?.textContent).toBe('Waiting for trades');
    const marketCap = container.querySelector<HTMLButtonElement>('button[aria-label^="Market cap"]');
    expect(marketCap).not.toBeNull();
    await act(async () => marketCap!.click());
    expect(container.querySelector('[data-headline="price"]')?.textContent).toBe(formatSpotPrice(statsSpotPrice(noTradeStats).value));
    expect(container.textContent).not.toMatch(/Syncing|Try again|Out of date|Prices may be out of date/);
    expect(container.querySelector('[role="alert"], .lucide-circle-alert, .lucide-triangle-alert, .text-warning, .bg-danger')).toBeNull();
    expect(httpReads.every(read => read.status === 200)).toBe(true);
    expect(httpReads.some(read => read.path.includes('/snapshots'))).toBe(false);
    const withoutPrice = decodeV2Contract('TopicSnapshot', { ...statsSnapshot, topics: [{ ...statsSnapshot.topics[0], payload: { ...noTradeStats, spotPriceWad: null } }] });
    const noPriceTransport = marketTransport(() => withoutPrice);
    const noPriceSession = new MarketSession(client(), pool, noPriceTransport.transport);
    try {
      await noPriceSession.start(); noPriceTransport.live();
      await act(async () => root.render(React.createElement(WagmiProvider, { config }, React.createElement(QueryClientProvider, { client: queries }, React.createElement(MarketClientProvider, { client: client(), children: React.createElement(FirstMarketView, { session: noPriceSession }) })))));
      expect(container.querySelector('[data-headline="price"]')?.textContent).toBe('—');
      expect(container.querySelector('button[aria-label^="Price"]')?.getAttribute('title')).toBe(noTradePriceReason);
      expect(container.textContent).not.toMatch(/Syncing|Try again/);
    } finally { noPriceSession.close(); }
  } finally {
    await act(async () => root.unmount());
    container.remove(); queries.clear(); session.close();
  }
}
test(marketFirstStateRendersStatsSpotPriceAndNeutralEmptyViews, marketFirstStateRendersStatsSpotPriceAndNeutralEmptyViews);

async function marketFirstStateChartWaitsForHistoryAndStartsAfterTheFirstCandle() {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
  vi.setSystemTime(Date.parse(candle.start) + 30000);
  const transport = marketTransport(call => call === 1 ? statsSnapshot : completeSnapshot);
  const session = new MarketSession(client(), pool, transport.transport);
  await session.start(); transport.live();
  const candleTopic = `market.candles.${pool}.1m`;
  const candleSnapshot = decodeV2Contract('TopicSnapshot', { ...statsSnapshot, topics: [{ topic: candleTopic, sequence: '1', revision: candle.revision, watermark: wire.candles.body.watermark, payload: candle }] });
  const chartTransport = marketTransport(() => candleSnapshot);
  const authorization = new AbortController();
  const baselineListeners = chartReplayListenerCount();
  const release = registerChartReplayTransport({ transport: chartTransport.transport, signal: authorization.signal });
  const bars: { time: number }[] = [];
  const feed = createReplayUdfDatafeed(pool, 'price', () => {}, undefined, { apiOrigin: origin, valuationOrigin: origin }, () => {}, () => {}, () => {}, session);
  try {
    feed.subscribeBars({}, '1', bar => bars.push(bar), 'first-candle');
    await waitFor(() => httpReads.some(read => read.path.includes('/candles')) && feed.resourceCounts().timers > 0);
    expect(chartTransport.reads).toHaveLength(0);
    expect(feed.replayStatus()[0].freshness.connectionStatus).toBe('connecting');
    expect(feed.resourceCounts().sockets).toBe(0);
    const initialReads = httpReads.filter(read => read.path.includes('/candles')).length;
    await vi.advanceTimersByTimeAsync(4999);
    expect(httpReads.filter(read => read.path.includes('/candles'))).toHaveLength(initialReads);
    await vi.advanceTimersByTimeAsync(6001);
    await waitFor(() => httpReads.filter(read => read.path.includes('/candles')).length === initialReads + 1);
    expect(httpReads.filter(read => read.path.includes('/candles'))[1].at - httpReads[0].at).toBeGreaterThanOrEqual(5000);
    expect(chartTransport.reads).toHaveLength(0);
    const beforeTrade = httpReads.filter(read => read.path.includes('/candles')).length;
    transport.control('resnapshot_required');
    await waitFor(() => transport.subscriptions.length === 2);
    transport.live();
    await waitFor(() => httpReads.filter(read => read.path.includes('/candles')).length === beforeTrade + 1);
    await waitFor(() => feed.resourceCounts().timers === 1);
    expect(session.getSnapshot().noTradesYet).toBe(false);
    expect(chartTransport.reads).toHaveLength(0);
    candleAvailable = true;
    await vi.advanceTimersByTimeAsync(11000);
    await waitFor(() => chartTransport.subscriptions.length === 1);
    expect(chartTransport.reads[0].topics).toEqual([candleTopic]);
    expect(chartTransport.subscriptions[0].afterCursor).toBe(candleSnapshot.resumeCursor);
    chartTransport.live();
    expect(feed.replayStatus()[0].replayComplete).toBe(true);
    expect(bars.some(bar => bar.time === Date.parse(candle.start))).toBe(true);
    expect(httpReads.every(read => read.status === 200)).toBe(true);
  } finally {
    feed.dispose(); release(); authorization.abort(); session.close();
    expect(feed.resourceCounts()).toEqual({ subscriptions: 0, requests: 0, sockets: 0, timers: 0 });
    expect(chartReplayListenerCount()).toBe(baselineListeners);
  }
}
test(marketFirstStateChartWaitsForHistoryAndStartsAfterTheFirstCandle, marketFirstStateChartWaitsForHistoryAndStartsAfterTheFirstCandle);

async function marketFirstStateChartStartsWithoutWaitingForRecentHistory() {
  for (const snapshot of [completeSnapshot, { ...completeSnapshot, topics: completeSnapshot.topics.filter(entry => entry.topic !== statsEntry.topic) }]) {
    const transport = marketTransport(() => snapshot);
    const session = new MarketSession(client(), pool, transport.transport);
    await session.start();
    expect(session.getSnapshot().firstStateReady).toBe(true);
    expect(session.getSnapshot().noTradesYet).toBe(false);
    const candleTopic = `market.candles.${pool}.1m`;
    const candleSnapshot = decodeV2Contract('TopicSnapshot', { ...statsSnapshot, topics: [{ topic: candleTopic, sequence: '1', revision: candle.revision, watermark: wire.candles.body.watermark, payload: candle }] });
    const chartTransport = marketTransport(() => candleSnapshot);
    const authorization = new AbortController();
    const release = registerChartReplayTransport({ transport: chartTransport.transport, signal: authorization.signal });
    const feed = createReplayUdfDatafeed(pool, 'price', () => {}, undefined, { apiOrigin: origin, valuationOrigin: origin }, () => {}, () => {}, () => {}, session);
    try {
      feed.subscribeBars({}, '1', () => {}, 'traded-pool');
      expect(chartTransport.reads).toHaveLength(1);
      await waitFor(() => chartTransport.subscriptions.length === 1);
      expect(httpReads.some(read => read.path.includes('/candles'))).toBe(true);
      expect(candleAvailable).toBe(false);
      chartTransport.live();
      expect(feed.replayStatus()[0].replayComplete).toBe(true);
    } finally { feed.dispose(); release(); authorization.abort(); session.close(); }
  }
}
test(marketFirstStateChartStartsWithoutWaitingForRecentHistory, marketFirstStateChartStartsWithoutWaitingForRecentHistory);

async function marketFirstStateChartWaitsForTheSessionBeforeChoosingItsStart() {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
  vi.setSystemTime(Date.parse(candle.start) + 30000);
  for (const snapshot of [statsSnapshot, completeSnapshot]) {
    let resolveSnapshot: ((snapshot: TopicSnapshot) => void) | undefined;
    const transport = marketTransport(() => new Promise<TopicSnapshot>(resolve => { resolveSnapshot = resolve; }));
    const session = new MarketSession(client(), pool, transport.transport);
    const started = session.start();
    const candleTopic = `market.candles.${pool}.1m`;
    const candleSnapshot = decodeV2Contract('TopicSnapshot', { ...statsSnapshot, topics: [{ topic: candleTopic, sequence: '1', revision: candle.revision, watermark: wire.candles.body.watermark, payload: candle }] });
    const chartTransport = marketTransport(() => candleSnapshot);
    const authorization = new AbortController();
    const release = registerChartReplayTransport({ transport: chartTransport.transport, signal: authorization.signal });
    const feed = createReplayUdfDatafeed(pool, 'price', () => {}, undefined, { apiOrigin: origin, valuationOrigin: origin }, () => {}, () => {}, () => {}, session);
    try {
      feed.subscribeBars({}, '1', () => {}, 'session-first');
      expect(session.getSnapshot().firstStateReady).toBe(false);
      await vi.advanceTimersByTimeAsync(11000);
      expect(chartTransport.reads).toHaveLength(0);
      expect(feed.resourceCounts()).toMatchObject({ sockets: 0, timers: 0 });
      resolveSnapshot!(snapshot);
      await started;
      expect(session.getSnapshot().firstStateReady).toBe(true);
      transport.live();
      if (snapshot === statsSnapshot) {
        await waitFor(() => feed.resourceCounts().timers === 1);
        expect(chartTransport.reads).toHaveLength(0);
        await vi.advanceTimersByTimeAsync(11000);
        await waitFor(() => feed.resourceCounts().requests === 0);
        expect(chartTransport.reads).toHaveLength(0);
        expect(feed.replayStatus()[0].freshness.connectionStatus).toBe('connecting');
      } else {
        expect(chartTransport.reads).toHaveLength(1);
        await waitFor(() => chartTransport.subscriptions.length === 1);
        chartTransport.live();
        expect(feed.replayStatus()[0].replayComplete).toBe(true);
      }
    } finally { feed.dispose(); release(); authorization.abort(); session.close(); }
  }
}
test(marketFirstStateChartWaitsForTheSessionBeforeChoosingItsStart, marketFirstStateChartWaitsForTheSessionBeforeChoosingItsStart);

async function marketFirstStateChartCancelsSnapshotRepeatsWhenTheWaitingMarkArrives() {
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
  vi.setSystemTime(Date.parse(candle.start) + 30000);
  const transport = marketTransport(call => {
    if (call === 1) throw new HttpError(503, { code: 'DEPENDENCY_UNAVAILABLE', message: 'Market snapshot is unavailable', requestId: '11111111-1111-4111-8111-111111111111', retryable: true, retryAfterMs: null, fieldErrors: [], details: null });
    return statsSnapshot;
  });
  const session = new MarketSession(client(), pool, transport.transport);
  await session.start();
  const chartTransport = marketTransport(() => {
    throw new HttpError(404, { code: 'TOPIC_NOT_STARTED', message: 'Topic has no first frame', requestId: '11111111-1111-4111-8111-111111111111', retryable: true, retryAfterMs: 5000, fieldErrors: [], details: null });
  });
  const authorization = new AbortController();
  const release = registerChartReplayTransport({ transport: chartTransport.transport, signal: authorization.signal });
  const feed = createReplayUdfDatafeed(pool, 'price', () => {}, undefined, { apiOrigin: origin, valuationOrigin: origin }, () => {}, () => {}, () => {}, session);
  try {
    feed.subscribeBars({}, '1', () => {}, 'waiting-mark');
    await waitFor(() => chartTransport.reads.length === 1);
    await session.start();
    await waitFor(() => feed.resourceCounts().timers === 1);
    expect(session.getSnapshot()).toMatchObject({ firstStateReady: true, noTradesYet: true });
    expect(feed.replayStatus()[0].freshness.connectionStatus).toBe('connecting');
    await vi.advanceTimersByTimeAsync(11000);
    await waitFor(() => feed.resourceCounts().requests === 0);
    expect(chartTransport.reads).toHaveLength(1);
    expect(chartTransport.subscriptions).toHaveLength(0);
    expect(feed.resourceCounts().sockets).toBe(0);
  } finally { feed.dispose(); release(); authorization.abort(); session.close(); }
}
test(marketFirstStateChartCancelsSnapshotRepeatsWhenTheWaitingMarkArrives, marketFirstStateChartCancelsSnapshotRepeatsWhenTheWaitingMarkArrives);
