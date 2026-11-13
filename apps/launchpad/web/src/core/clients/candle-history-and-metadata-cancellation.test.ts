import { createServer, type Server, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { readFileSync } from 'node:fs';
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'vitest';
import { CandleHistoryError, fetchCandleHistory, fetchCandleHistoryOutcome, fetchCandleStats } from './candle-stats';
import { fetchTokenMetadata } from './metadata';
import type { TokenMetadata } from '@/widgets/home/types';

// Exact ProductionHistoryHandler responses, including the original retryable 503 windows.
type Recorded = { path: string; status: number; headers: Record<string, string>; bodyBytes: string };
const charts = JSON.parse(readFileSync(new URL('../test-fixtures/market-production/charts-wire.json', import.meta.url), 'utf8')) as {
  now: number; history: Recorded; statsDaily: Recorded; statsHourly: Recorded; statsFiveMinute: Recorded;
};
const pool = '0xce20da50f92d21d8bee497910c782e9aa25b6263';
const anchor = 1_800_057_600; // UTC day boundary.
const day = 86400;
type Bar = { time: number; open: number; high: number; low: number; close: number; volume: number };
const bar = (time: number, close: number, high = close, low = close): Bar => ({ time, open: close, high, low, close, volume: 1 });
// Committed in-memory history store. The loopback peer applies the producer's half-open bounds.
const historyStore: Record<string, Bar[]> = {
  '1D': [bar(anchor - 400 * day, 20, 9999, 0.01), bar(anchor - 2 * day, 80), bar(anchor - day, 100), bar(anchor, 120, 150, 3)],
  '60': [bar(anchor - 7200, 50), bar(anchor - 3600, 100), bar(anchor, 120)],
  '5': [bar(anchor - 600, 50), bar(anchor - 300, 100), bar(anchor, 110)],
};
const token = (id: number) => `0x${id.toString(16).padStart(40, '0')}`;
const metadataStore = new Map<string, TokenMetadata>([1, 2, 3, 4, 5, 6, 7, 8].map(id => [token(id), {
  token_address: token(id), name: `Metadata ${id}`, symbol: `M${id}`,
}]));

type Mode = 'recorded' | 'store' | 'no_data' | 'malformed' | 'bad_schema' | 'http_error' | 'disconnect' | 'held' | 'missing_baseline' | 'flat' | 'zero';
let mode: Mode;
let server: Server;
let origin: string;
let requests: URL[];
let parked: Array<{ response: ServerResponse; release: () => void }>;
let closedRequests: number;
let originalMetadataOrigin: string | undefined;

beforeAll(async () => {
  originalMetadataOrigin = process.env.NEXT_PUBLIC_METADATA_API_URL;
  server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://loopback.invalid');
    requests.push(url);
    const send = (status: number, body: unknown, headers: Record<string, string> = {}) => {
      response.writeHead(status, { 'Content-Type': 'application/json', ...headers });
      response.end(JSON.stringify(body));
    };
    const hold = (release: () => void) => {
      response.on('close', () => { if (!response.writableEnded) closedRequests++; });
      parked.push({ response, release });
    };
    if (url.pathname.startsWith('/metadata/')) {
      const address = url.pathname.slice('/metadata/'.length);
      const value = metadataStore.get(address);
      hold(() => send(value ? 200 : 404, value ?? null));
      return;
    }
    if (url.pathname !== '/v2/udf/history') { send(404, {}); return; }
    const recorded = [charts.history, charts.statsDaily, charts.statsHourly, charts.statsFiveMinute].find(entry => entry.path === request.url);
    if (mode === 'recorded' && recorded) {
      response.writeHead(recorded.status, recorded.headers);
      response.end(recorded.bodyBytes);
      return;
    }
    if (mode === 'disconnect') { request.socket.destroy(); return; }
    if (mode === 'held') {
      hold(() => send(200, { s: 'no_data' }, { 'X-Schema-Version': '2' }));
      return;
    }
    if (mode === 'http_error') {
      send(400, { code: 'INVALID_ARGUMENT', message: 'invalid history window', retryable: false, retryAfterMs: null, requestId: 'history-error' });
      return;
    }
    const headers = { 'X-Schema-Version': mode === 'bad_schema' ? '1' : '2' };
    if (mode === 'no_data') { send(200, { s: 'no_data', nextTime: anchor - day }, headers); return; }
    if (mode === 'malformed') { send(200, { s: 'ok', mode: 'price', t: [anchor], c: [] }, headers); return; }
    const from = Number(url.searchParams.get('from'));
    const to = Number(url.searchParams.get('to'));
    const resolution = url.searchParams.get('resolution') ?? '';
    const interval = resolution === '1D' ? day : resolution === '60' ? 3600 : 300;
    let rows = (historyStore[resolution] ?? []).filter(row => row.time >= from && row.time < to);
    if (mode === 'missing_baseline') rows = rows.filter(row => row.time !== anchor - interval);
    if (mode === 'flat') rows = rows.map(row => bar(row.time, 100));
    if (mode === 'zero') rows = rows.map(row => bar(row.time, 0));
    send(200, {
      s: rows.length ? 'ok' : 'no_data', mode: 'price',
      t: rows.map(row => row.time), o: rows.map(row => row.open), h: rows.map(row => row.high),
      l: rows.map(row => row.low), c: rows.map(row => row.close), v: rows.map(row => row.volume),
    }, headers);
  });
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_METADATA_API_URL = origin;
});

beforeEach(() => { mode = 'recorded'; requests = []; parked = []; closedRequests = 0; });
afterEach(() => { for (const entry of parked) if (!entry.response.destroyed && !entry.response.writableEnded) entry.release(); });
afterAll(async () => {
  server.closeAllConnections();
  await new Promise<void>(resolve => server.close(() => resolve()));
  if (originalMetadataOrigin === undefined) delete process.env.NEXT_PUBLIC_METADATA_API_URL;
  else process.env.NEXT_PUBLIC_METADATA_API_URL = originalMetadataOrigin;
});

async function candle_unavailable_zero_preserves_production_retry_metadata() {
  const stats = await fetchCandleStats(pool, { chartsOrigin: origin, now: charts.now });
  expect(stats).toMatchObject({ change24h: null, change1h: null, change5m: null, rollingYearHigh: null, rollingYearLow: null });
  const recordings = [charts.statsDaily, charts.statsHourly, charts.statsFiveMinute];
  expect(requests.map(url => url.pathname + url.search).sort()).toEqual(recordings.map(entry => entry.path).sort());
  for (const [index, outcome] of Object.values(stats.history).entries()) {
    const error = JSON.parse(recordings[index].bodyBytes);
    expect(outcome).toEqual({ status: 'unavailable', httpStatus: 503, code: error.code, message: error.message, retryable: true, retryAfterMs: 1000, requestId: error.requestId });
  }
  await expect(fetchCandleHistory(pool, '60', charts.now - 7200, charts.now, { chartsOrigin: origin })).rejects.toMatchObject({
    name: 'CandleHistoryError', outcome: { status: 'unavailable', httpStatus: 503, retryable: true, retryAfterMs: 1000 },
  });
}
test(candle_unavailable_zero_preserves_production_retry_metadata.name, candle_unavailable_zero_preserves_production_retry_metadata);

async function candle_unavailable_zero_reads_real_price_history() {
  const url = new URL(charts.history.path, origin);
  const from = Number(url.searchParams.get('from'));
  const to = Number(url.searchParams.get('to'));
  const recorded = JSON.parse(charts.history.bodyBytes);
  const history = await fetchCandleHistory(pool, '1', from, to, { chartsOrigin: origin });
  const outcome = await fetchCandleHistoryOutcome(pool, '1', from, to, { chartsOrigin: origin });
  expect(history).toEqual({ t: recorded.t, o: recorded.o, h: recorded.h, l: recorded.l, c: recorded.c, v: recorded.v });
  expect(outcome).toEqual({ status: 'ok', history });
}
test(candle_unavailable_zero_reads_real_price_history.name, candle_unavailable_zero_reads_real_price_history);

async function candle_unavailable_zero_distinguishes_empty_flat_and_zero_markets() {
  mode = 'no_data';
  const empty = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1 });
  expect(empty).toMatchObject({ change24h: null, change1h: null, change5m: null, rollingYearHigh: null, rollingYearLow: null });
  for (const result of Object.values(empty.history)) expect(result).toEqual({ status: 'no_data', nextTime: anchor - day });
  expect(await fetchCandleHistory(pool, '5', anchor - 600, anchor + 1, { chartsOrigin: origin })).toBeNull();
  mode = 'flat';
  const flat = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1 });
  expect(flat).toMatchObject({ change24h: 0, change1h: 0, change5m: 0, rollingYearHigh: 100, rollingYearLow: 100 });
  mode = 'zero';
  const zero = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1 });
  expect(zero).toMatchObject({ change24h: null, change1h: null, change5m: null, rollingYearHigh: 0, rollingYearLow: null });
  for (const result of Object.values(zero.history)) expect(result.status).toBe('ok');
}
test(candle_unavailable_zero_distinguishes_empty_flat_and_zero_markets.name, candle_unavailable_zero_distinguishes_empty_flat_and_zero_markets);

async function candle_unavailable_zero_preserves_errors_transport_and_abort() {
  for (const invalid of ['malformed', 'bad_schema', 'http_error'] as const) {
    mode = invalid;
    const stats = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1 });
    expect(stats.change5m).toBeNull();
    expect(stats.history.fiveMinute).toMatchObject({ status: 'error', httpStatus: invalid === 'http_error' ? 400 : 200 });
    if (invalid === 'http_error') expect(stats.history.fiveMinute).toMatchObject({ code: 'INVALID_ARGUMENT', retryable: false, retryAfterMs: null, requestId: 'history-error' });
  }
  mode = 'disconnect';
  const disconnected = await fetchCandleHistoryOutcome(pool, '5', anchor - 600, anchor + 1, { chartsOrigin: origin });
  expect(disconnected).toMatchObject({ status: 'unavailable', httpStatus: null, retryable: null });
  mode = 'held';
  const controller = new AbortController();
  const reason = new Error('candle subscriber detached');
  const pending = fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1, signal: controller.signal });
  await expect.poll(() => parked.length).toBe(3);
  controller.abort(reason);
  const aborted = await pending;
  for (const result of Object.values(aborted.history)) expect(result).toEqual({ status: 'aborted', reason });
  expect(aborted).toMatchObject({ change24h: null, change1h: null, change5m: null, rollingYearHigh: null, rollingYearLow: null });
  const before = requests.length;
  await expect(fetchCandleHistory(pool, '5', anchor - 600, anchor + 1, { chartsOrigin: origin, signal: controller.signal })).rejects.toBeInstanceOf(CandleHistoryError);
  expect(requests).toHaveLength(before);
}
test(candle_unavailable_zero_preserves_errors_transport_and_abort.name, candle_unavailable_zero_preserves_errors_transport_and_abort);

async function candle_window_labels_selects_exact_timestamp_baselines() {
  mode = 'store';
  const stats = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1 });
  expect(stats.change24h).toBeCloseTo(20, 10);
  expect(stats.change1h).toBeCloseTo(20, 10);
  expect(stats.change5m).toBeCloseTo(10, 10);
  // A missing exact baseline must not silently substitute the older endpoint.
  mode = 'missing_baseline';
  const missing = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1 });
  expect(missing).toMatchObject({ change24h: null, change1h: null, change5m: null });
}
test(candle_window_labels_selects_exact_timestamp_baselines.name, candle_window_labels_selects_exact_timestamp_baselines);

async function candle_window_labels_covers_boundary_and_history_older_than_one_year() {
  mode = 'store';
  const boundary = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor });
  expect(boundary.change24h).toBeCloseTo(25, 10);
  expect(boundary.change1h).toBeCloseTo(100, 10);
  expect(boundary.change5m).toBeCloseTo(100, 10);
  const stats = await fetchCandleStats(pool, { chartsOrigin: origin, now: anchor + 1 });
  expect(stats.extremesWindow).toEqual({ from: anchor + 1 - 365 * day, to: anchor + 1 });
  expect(stats.rollingYearHigh).toBe(150);
  expect(stats.rollingYearLow).toBe(3);
  expect(historyStore['1D'][0]).toMatchObject({ high: 9999, low: 0.01 });
  expect(stats).not.toHaveProperty('ath');
  expect(stats).not.toHaveProperty('atl');
}
test(candle_window_labels_covers_boundary_and_history_older_than_one_year.name, candle_window_labels_covers_boundary_and_history_older_than_one_year);

async function metadata_shared_abort_keeps_each_consumer_independent() {
  for (const abortedIndex of [0, 1]) {
    const address = token(abortedIndex + 1);
    const controllers = [new AbortController(), new AbortController()];
    const published: number[] = [];
    const consumers = controllers.map((controller, index) => fetchTokenMetadata(address, controller.signal).then(value => { published.push(index); return value; }));
    const reason = new Error(`consumer ${abortedIndex} detached`);
    const rejected = expect(consumers[abortedIndex]).rejects.toBe(reason);
    await expect.poll(() => parked.length).toBe(abortedIndex + 1);
    controllers[abortedIndex].abort(reason);
    await rejected;
    const held = parked[abortedIndex];
    expect(held.response.destroyed).toBe(false);
    held.release();
    expect(await consumers[1 - abortedIndex]).toEqual(metadataStore.get(address));
    expect(published).toEqual([1 - abortedIndex]);
    expect(requests.filter(url => url.pathname === `/metadata/${address}`)).toHaveLength(1);
  }
}
test(metadata_shared_abort_keeps_each_consumer_independent.name, metadata_shared_abort_keeps_each_consumer_independent);

async function metadata_shared_abort_detaches_last_consumer_and_allows_a_fresh_read() {
  const address = token(3);
  const controllers = [new AbortController(), new AbortController()];
  const consumers = controllers.map(controller => fetchTokenMetadata(address, controller.signal));
  const rejected = consumers.map(promise => expect(promise).rejects.toMatchObject({ name: 'AbortError' }));
  await expect.poll(() => parked.length).toBe(1);
  controllers[0].abort();
  controllers[1].abort();
  // Reopen before the cancelled transport settles, exercising inflight-entry cleanup races.
  const reopened = fetchTokenMetadata(address);
  await Promise.all(rejected);
  await expect.poll(() => parked.length).toBe(2);
  await expect.poll(() => closedRequests).toBe(1);
  const lateSubscriber = fetchTokenMetadata(address);
  parked[1].release();
  expect(await reopened).toEqual(metadataStore.get(address));
  expect(await lateSubscriber).toEqual(metadataStore.get(address));
  expect(requests.filter(url => url.pathname === `/metadata/${address}`)).toHaveLength(2);
}
test(metadata_shared_abort_detaches_last_consumer_and_allows_a_fresh_read.name, metadata_shared_abort_detaches_last_consumer_and_allows_a_fresh_read);

async function metadata_shared_abort_rejects_preaborted_subscribers_and_preserves_live_reads() {
  const address = token(4);
  const live = fetchTokenMetadata(address);
  await expect.poll(() => parked.length).toBe(1);
  const controller = new AbortController();
  const reason = new Error('already detached');
  controller.abort(reason);
  await expect(fetchTokenMetadata(address, controller.signal)).rejects.toBe(reason);
  parked[0].release();
  expect(await live).toEqual(metadataStore.get(address));
  expect(requests.filter(url => url.pathname === `/metadata/${address}`)).toHaveLength(1);
  const absent = fetchTokenMetadata(token(99));
  await expect.poll(() => parked.length).toBe(2);
  parked[1].release();
  expect(await absent).toBeNull();
}
test(metadata_shared_abort_rejects_preaborted_subscribers_and_preserves_live_reads.name, metadata_shared_abort_rejects_preaborted_subscribers_and_preserves_live_reads);
