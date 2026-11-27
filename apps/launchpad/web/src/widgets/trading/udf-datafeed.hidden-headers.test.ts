import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { setTimeout as wait } from 'node:timers/promises';
import { expect, test } from 'vitest';
import { fetchCandleHistoryOutcome } from '@/core/clients/candle-stats';
import {
  canonicalCandleToDisplayBar,
  createReplayUdfDatafeed,
  fetchCanonicalCandlePage,
  type ChartAccessibleView,
  type TradingViewChartMark,
} from './udf-datafeed';

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Chart configuration required');
  return Object.fromEntries(Object.entries(value));
}

function text(value: unknown): string {
  if (typeof value !== 'string' || !value) throw new Error('Chart configuration value required');
  return value;
}

function origin(value: unknown): string {
  const url = new URL(text(value));
  if (!['http:', 'https:'].includes(url.protocol) || !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname) || url.username || url.password) throw new Error('Private chart authority required');
  return url.origin;
}

function chart() {
  const bytes = readFileSync(text(process.env.JOINED_CONFIG_FILE));
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.JOINED_CONFIG_SHA256);
  const config = object(JSON.parse(bytes.toString()));
  const chain = object(JSON.parse(readFileSync(text(process.env.JOINED_CHAIN_FILE), 'utf8')));
  expect(chain.realm).toBe('private-anvil125');
  expect(chain.productionActivation).toBe(false);
  const pool = text(chain.pool);
  expect(pool).toMatch(/^0x[0-9a-f]{40}$/i);
  if (!Array.isArray(config.processes)) throw new Error('Chart services required');
  const charts = config.processes.map(object).find(process => process.role === 'trading-charts');
  if (!charts) throw new Error('Price history service required');
  const now = Math.floor(Date.now() / 1000);
  return { pool, api: origin(config.api_origin), charts: origin(charts.ready_url), period: { from: now - 3 * 86400, to: now + 60 } };
}

function browserHeaders(incompatiblePath?: string) {
  const fetchResponse = globalThis.fetch;
  const reads = new Map<string, number>();
  const fetchWithBrowserHeaders: typeof fetch = async (input, init) => {
    const url = new URL(input instanceof Request ? input.url : input.toString());
    reads.set(url.pathname, (reads.get(url.pathname) ?? 0) + 1);
    const response = await fetchResponse(input, init);
    const headers = new Headers();
    for (const name of ['cache-control', 'content-type']) {
      const value = response.headers.get(name);
      if (value !== null) headers.set(name, value);
    }
    if (url.pathname === incompatiblePath) headers.set('X-Schema-Version', '1');
    return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
  };
  globalThis.fetch = fetchWithBrowserHeaders;
  return { reads: (path: string) => reads.get(path) ?? 0, restore: () => { globalThis.fetch = fetchResponse; } };
}

type Feed = ReturnType<typeof createReplayUdfDatafeed>;
function history(feed: Feed, period: { from: number; to: number }, resolution = '1D') {
  return new Promise<Array<{ time: number; open: number; close: number }>>((resolve, reject) =>
    feed.getBars({}, resolution, period, resolve, reason => reject(new Error(reason)))
  );
}
function marks(feed: Feed, period: { from: number; to: number }) {
  return new Promise<TradingViewChartMark[]>(resolve => feed.getMarks({}, period.from, period.to, resolve));
}

async function markersLoadWhenTheBrowserHidesAnswerHeaders() {
  const actual = chart();
  const transport = browserHeaders();
  const views: ChartAccessibleView[] = [];
  const feed = createReplayUdfDatafeed(actual.pool, 'price', () => {}, undefined, { apiOrigin: actual.api, valuationOrigin: actual.charts }, view => views.push(view));
  const path = `/v2/pools/${actual.pool}/trades`.toLowerCase();
  try {
    await history(feed, actual.period);
    const loaded = await marks(feed, actual.period);
    expect(loaded.length).toBeGreaterThan(0);
    expect(views.at(-1)?.annotationReason).toBe('Only fills with canonical timestamp and execution-price provenance are annotated');
    expect(views.at(-1)?.annotationState).toBe('partial');
    expect(transport.reads(path)).toBe(1);
    await wait(2200);
    expect(await marks(feed, actual.period)).toEqual(loaded);
    expect(transport.reads(path)).toBe(1);
  } finally { feed.dispose(); transport.restore(); }
}
test(markersLoadWhenTheBrowserHidesAnswerHeaders, markersLoadWhenTheBrowserHidesAnswerHeaders, 60_000);

async function markersRejectAnotherSchemaOncePerChartContext() {
  const actual = chart();
  const path = `/v2/pools/${actual.pool}/trades`.toLowerCase();
  const transport = browserHeaders(path);
  const views: ChartAccessibleView[] = [];
  const feed = createReplayUdfDatafeed(actual.pool, 'price', () => {}, undefined, { apiOrigin: actual.api, valuationOrigin: actual.charts }, view => views.push(view));
  try {
    await history(feed, actual.period);
    expect(await marks(feed, actual.period)).toEqual([]);
    expect(views.at(-1)).toMatchObject({ annotationState: 'partial', annotationReason: 'Confirmed trade schema negotiation failed' });
    expect(transport.reads(path)).toBe(1);
    await wait(2200);
    feed.setMarkPresentation({ enabled: false });
    feed.setMarkPresentation({ enabled: true });
    feed.retry();
    await history(feed, actual.period);
    expect(await marks(feed, actual.period)).toEqual([]);
    expect(transport.reads(path)).toBe(1);
    expect(views.at(-1)).toMatchObject({ annotationState: 'partial', annotationReason: 'Confirmed trade schema negotiation failed' });
    await history(feed, actual.period, '60');
    expect(await marks(feed, actual.period)).toEqual([]);
    expect(transport.reads(path)).toBe(2);
    await wait(2200);
    expect(transport.reads(path)).toBe(2);
  } finally { feed.dispose(); transport.restore(); }
}
test(markersRejectAnotherSchemaOncePerChartContext, markersRejectAnotherSchemaOncePerChartContext, 60_000);

async function candlesLoadWhenTheBrowserHidesAnswerHeaders() {
  const actual = chart();
  const page = await fetchCanonicalCandlePage(actual.pool, '1D', new Date(actual.period.from * 1000).toISOString(), new Date(actual.period.to * 1000).toISOString(), 50, undefined, undefined, actual.api);
  expect(page.data.items.length).toBeGreaterThan(0);
  expect(page.nextCursor).toBeNull();
  const transport = browserHeaders();
  const feed = createReplayUdfDatafeed(actual.pool, 'price', () => {}, undefined, { apiOrigin: actual.api, valuationOrigin: actual.charts });
  const path = `/v2/pools/${actual.pool}/candles`.toLowerCase();
  try {
    expect(await history(feed, actual.period)).toEqual(page.data.items.map(candle => canonicalCandleToDisplayBar(candle)));
    await marks(feed, actual.period);
    await wait(2200);
    expect(transport.reads(path)).toBe(1);
  } finally { feed.dispose(); transport.restore(); }
}
test(candlesLoadWhenTheBrowserHidesAnswerHeaders, candlesLoadWhenTheBrowserHidesAnswerHeaders, 60_000);

async function candlesRejectAnotherSchemaWithoutRepeatingTheRead() {
  const actual = chart();
  const path = `/v2/pools/${actual.pool}/candles`.toLowerCase();
  const transport = browserHeaders(path);
  const feed = createReplayUdfDatafeed(actual.pool, 'price', () => {}, undefined, { apiOrigin: actual.api, valuationOrigin: actual.charts });
  try {
    await expect(history(feed, actual.period)).rejects.toThrow('Chart can\'t load right now');
    await wait(2200);
    expect(transport.reads(path)).toBe(1);
    expect(transport.reads(`/v2/pools/${actual.pool}/trades`.toLowerCase())).toBe(0);
  } finally { feed.dispose(); transport.restore(); }
}
test(candlesRejectAnotherSchemaWithoutRepeatingTheRead, candlesRejectAnotherSchemaWithoutRepeatingTheRead, 60_000);

async function candleStatsDecodeHistoryWithoutASchemaHeader() {
  const actual = chart();
  const transport = browserHeaders();
  try {
    const outcome = await fetchCandleHistoryOutcome(actual.pool, '1D', actual.period.from, actual.period.to, { chartsOrigin: actual.charts });
    expect(outcome.status).toBe('ok');
    if (outcome.status !== 'ok') throw new Error('Price history required');
    expect(outcome.history.t.length).toBeGreaterThan(0);
    expect(outcome.history.c).toHaveLength(outcome.history.t.length);
    expect(transport.reads('/v2/udf/history')).toBe(1);
  } finally { transport.restore(); }
}
test(candleStatsDecodeHistoryWithoutASchemaHeader, candleStatsDecodeHistoryWithoutASchemaHeader, 60_000);

async function candleStatsRejectAnExplicitlyIncompatibleSchema() {
  const actual = chart();
  const transport = browserHeaders('/v2/udf/history');
  try {
    const outcome = await fetchCandleHistoryOutcome(actual.pool, '1D', actual.period.from, actual.period.to, { chartsOrigin: actual.charts });
    expect(outcome).toMatchObject({ status: 'error', httpStatus: 200, message: 'Invalid candle history envelope' });
    expect(transport.reads('/v2/udf/history')).toBe(1);
  } finally { transport.restore(); }
}
test(candleStatsRejectAnExplicitlyIncompatibleSchema, candleStatsRejectAnExplicitlyIncompatibleSchema, 60_000);
