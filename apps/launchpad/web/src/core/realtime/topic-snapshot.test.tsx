// @vitest-environment jsdom
import { readFileSync } from 'node:fs';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import type { Duplex } from 'node:stream';
import { createRequire } from 'node:module';
import { URL as NodeURL } from 'node:url';
import { setTimeout as networkTick } from 'node:timers/promises';
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, test, vi } from 'vitest';
import { SdkClient, browserStreamTransportV2, decodeV2Contract, type ReconnectStatus, type V2TopicSnapshot } from '@kindle/sdk';
import { MarketSession } from '@/page-modules/token/market-session';
import { MarketTradeFeed } from '@/page-modules/token/market-trade-feed';
import { FreshnessNotice } from '@/widgets/coin/freshness-notice';
import { createReplayUdfDatafeed, registerChartReplayTransport, type ChartFreshness } from '@/widgets/trading/udf-datafeed';
import { CanonicalDataStream } from './data-stream';

interface ServerSocket {
  send(data: string): void;
  on(event: 'message', listener: (data: unknown) => void): void;
  terminate(): void;
}
interface SocketServer {
  clients: Set<ServerSocket>;
  handleUpgrade(request: IncomingMessage, socket: Duplex, head: Buffer, done: (socket: ServerSocket) => void): void;
  close(): void;
}
const { WebSocketServer } = createRequire(import.meta.url)('ws') as {
  WebSocketServer: new (options: { noServer: true }) => SocketServer;
};
const wire = JSON.parse(readFileSync(new NodeURL('../test-fixtures/market-production/core-api-wire.json', import.meta.url), 'utf8'));
const snapshotResponse = decodeV2Contract('TopicSnapshotReadResponse', wire.snapshot.body);
const candleResponse = decodeV2Contract('CandlesResponse', wire.candles.body);
const chartsWire = JSON.parse(readFileSync(new NodeURL('../test-fixtures/market-production/charts-wire.json', import.meta.url), 'utf8'));
const notFound = decodeV2Contract('Error', JSON.parse(chartsWire.unknownSymbol.bodyBytes));
const candle = candleResponse.data.items[0];
const pool = candle.poolAddress;
const topics = snapshotResponse.data.snapshot.topics.map(entry => entry.topic);
const clients = ['market', 'canonical', 'chart'] as const;
type ClientKind = typeof clients[number];

function visibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: state });
  document.dispatchEvent(new Event('visibilitychange'));
}

async function eventually(predicate: () => boolean, describe = () => 'Stream state') {
  for (let turn = 0; !predicate(); turn++) {
    if (turn === 400) throw new Error(`${describe()} did not settle`);
    await networkTick(5);
  }
  await networkTick(20);
}

async function snapshotOrigin() {
  const requests: number[] = [];
  let ready = false;
  let failure = { status: 404, code: 'TOPIC_NOT_STARTED', retryAfterMs: 8000 };
  const json = (response: ServerResponse, status: number, body: unknown) => {
    response.writeHead(status, { 'Content-Type': 'application/json', 'X-Schema-Version': '2', 'Cache-Control': 'no-store' });
    response.end(JSON.stringify(body));
  };
  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    if (url.pathname === '/v2/snapshots') {
      requests.push(Date.now());
      if (!ready) {
        json(response, failure.status, decodeV2Contract('Error', {
          code: failure.code, message: failure.code, requestId: snapshotResponse.requestId,
          retryable: failure.status === 404, retryAfterMs: failure.retryAfterMs, fieldErrors: [], details: null,
        }));
        return;
      }
      const requested = (url.searchParams.get('topics') ?? '').split(',');
      const snapshot = snapshotResponse.data.snapshot;
      const entries = requested[0]?.startsWith('market.candles.')
        ? [{ topic: requested[0], payload: candle, sequence: '1', revision: candle.revision, watermark: candleResponse.watermark }]
        : snapshot.topics.filter(entry => requested.includes(entry.topic));
      json(response, 200, { ...snapshotResponse, data: { snapshot: { ...snapshot, topics: entries } } });
      return;
    }
    if (url.pathname === `/v2/pools/${pool}/candles`) {
      json(response, 200, { ...candleResponse, data: { ...candleResponse.data, items: ready ? [candle] : [] } });
      return;
    }
    if (url.pathname === `/v2/pools/${pool}/trades`) { json(response, 200, wire.tradesFirstPage.body); return; }
    json(response, 404, notFound);
  });
  const sockets = new WebSocketServer({ noServer: true });
  server.on('upgrade', (request, socket, head) => sockets.handleUpgrade(request, socket, head, connection => {
    connection.on('message', raw => {
      const subscribe = decodeV2Contract('Subscribe', JSON.parse(String(raw)));
      for (const control of wire.replayFrames) connection.send(JSON.stringify({ ...control, subscriptionId: subscribe.subscriptionId }));
    });
  }));
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const streamOrigin = origin.replace(/^http/, 'ws');
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false });
  return {
    origin, client, requests,
    publish: () => { ready = true; },
    answer: (value: typeof failure) => { failure = value; },
    close: async () => {
      for (const socket of sockets.clients) socket.terminate();
      sockets.close();
      server.closeAllConnections();
      await new Promise<void>(resolve => server.close(() => resolve()));
    },
  };
}

function streamClient(kind: ClientKind, fixture: Awaited<ReturnType<typeof snapshotOrigin>>) {
  if (kind === 'market') {
    const session = new MarketSession(fixture.client, pool);
    void session.start();
    return {
      waiting: () => {
        expect(session.getSnapshot()).toMatchObject({ rows: [], stream: 'loading', error: null, watermark: null });
      },
      answered: () => session.getSnapshot().stream === 'live',
      failed: () => session.getSnapshot().stream === 'stale' && session.getSnapshot().error !== null,
      retry: () => { void session.start(); },
      close: () => session.close(),
      snapshot: () => session.getSnapshot(),
    };
  }
  if (kind === 'canonical') {
    const stream = new CanonicalDataStream(fixture.client, { applicationOrigin: window.location.origin, snapshotPath: '/v2/snapshots' }, false, '');
    const snapshots: V2TopicSnapshot[] = [];
    const statuses: ReconnectStatus[] = [];
    stream.subscribe({ topics, installSnapshot: value => { snapshots.push(value); }, onEvent: () => {}, onStatus: value => statuses.push(value) });
    return {
      waiting: () => {
        expect(snapshots).toEqual([]);
        expect(stream.status).toEqual({ state: 'loading', circuitOpen: false, authenticationRequired: false });
        expect(statuses.some(status => status.state === 'stale' || status.state === 'resync-required')).toBe(false);
      },
      answered: () => snapshots.length === 1 && stream.status.state === 'live',
      failed: () => stream.status.state === 'stale' && stream.status.reason !== undefined,
      retry: () => { void stream.retry(); },
      close: () => stream.dispose(),
    };
  }
  const authorization = new AbortController();
  const release = registerChartReplayTransport({
    transport: browserStreamTransportV2(fixture.client, { applicationOrigin: window.location.origin, snapshotPath: '/v2/snapshots' }, false), signal: authorization.signal,
  });
  const states: ChartFreshness[] = [];
  const feed = createReplayUdfDatafeed(pool, 'price', value => states.push(value), undefined, { apiOrigin: fixture.origin, valuationOrigin: fixture.origin });
  const ticks: unknown[] = [];
  feed.subscribeBars({}, '1', value => ticks.push(value), 'chart');
  return {
    waiting: () => {
      expect(ticks).toEqual([]);
      expect(states.at(-1)).toMatchObject({ connectionStatus: 'snapshotting', lagReason: null });
      expect(states.every(state => state.lagReason === null)).toBe(true);
      expect(feed.resourceCounts()).toMatchObject({ sockets: 0, timers: 0 });
    },
    answered: () => feed.replayStatus()[0]?.replayComplete === true && ticks.length > 0,
    failed: () => states.some(state => state.connectionStatus === 'resync_required' && state.lagReason !== null),
    retry: () => feed.retry(),
    close: () => { feed.dispose(); authorization.abort(); release(); },
  };
}

afterEach(() => {
  visibility('visible');
  vi.useRealTimers();
});

async function topicNotStartedWaitsForHintThenStartsEachStream() {
  for (const kind of clients) {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] });
    vi.setSystemTime(Date.parse(candle.start) + 90_000);
    visibility('visible');
    const fixture = await snapshotOrigin();
    const consumer = streamClient(kind, fixture);
    try {
      await eventually(() => fixture.requests.length === 1, () => `${kind}: snapshot requests=${fixture.requests.length}`);
      consumer.waiting();
      await vi.advanceTimersByTimeAsync(7999);
      await networkTick(20);
      expect(fixture.requests).toHaveLength(1);
      consumer.retry();
      await networkTick(20);
      expect(fixture.requests).toHaveLength(1);
      consumer.waiting();
      fixture.publish();
      await vi.advanceTimersByTimeAsync(1);
      await eventually(consumer.answered, () => `${kind}: snapshot requests=${fixture.requests.length}, stream awaiting live state`);
      expect(fixture.requests).toHaveLength(2);
      expect(fixture.requests[1] - fixture.requests[0]).toBe(8000);
    } finally { consumer.close(); await fixture.close(); vi.useRealTimers(); }
  }
}
test(topicNotStartedWaitsForHintThenStartsEachStream, topicNotStartedWaitsForHintThenStartsEachStream);

async function topicNotStartedRepeatsWithFiveSecondMinimum() {
  for (const kind of clients) {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] });
    const fixture = await snapshotOrigin();
    fixture.answer({ status: 404, code: 'TOPIC_NOT_STARTED', retryAfterMs: 1000 });
    const consumer = streamClient(kind, fixture);
    try {
      await eventually(() => fixture.requests.length === 1, () => `${kind}: snapshot requests=${fixture.requests.length}`);
      for (let repeat = 1; repeat <= 2; repeat++) {
        consumer.waiting();
        await vi.advanceTimersByTimeAsync(4999);
        await networkTick(20);
        expect(fixture.requests).toHaveLength(repeat);
        await vi.advanceTimersByTimeAsync(1);
        await eventually(() => fixture.requests.length === repeat + 1, () => `${kind}: snapshot requests=${fixture.requests.length}`);
        consumer.waiting();
        expect(fixture.requests[repeat] - fixture.requests[repeat - 1]).toBe(5000);
      }
    } finally { consumer.close(); await fixture.close(); vi.useRealTimers(); }
  }
}
test(topicNotStartedRepeatsWithFiveSecondMinimum, topicNotStartedRepeatsWithFiveSecondMinimum);

async function topicNotStartedStopsRepeatsWhileHiddenAndAfterClose() {
  for (const kind of clients) {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] });
    const fixture = await snapshotOrigin();
    const consumer = streamClient(kind, fixture);
    try {
      await eventually(() => fixture.requests.length === 1, () => `${kind}: snapshot requests=${fixture.requests.length}`);
      consumer.waiting();
      visibility('hidden');
      await vi.advanceTimersByTimeAsync(1000);
      visibility('visible');
      await networkTick(20);
      expect(fixture.requests).toHaveLength(1);
      visibility('hidden');
      await vi.advanceTimersByTimeAsync(30_000);
      await networkTick(20);
      expect(fixture.requests).toHaveLength(1);
      visibility('visible');
      await eventually(() => fixture.requests.length === 2, () => `${kind}: snapshot requests=${fixture.requests.length}`);
      consumer.close();
      await vi.advanceTimersByTimeAsync(30_000);
      visibility('hidden');
      visibility('visible');
      await networkTick(20);
      expect(fixture.requests).toHaveLength(2);
    } finally { consumer.close(); await fixture.close(); vi.useRealTimers(); }
  }
}
test(topicNotStartedStopsRepeatsWhileHiddenAndAfterClose, topicNotStartedStopsRepeatsWhileHiddenAndAfterClose);

async function otherSnapshotFailuresKeepEachStreamsErrorState() {
  for (const kind of clients) {
    for (const failure of [
      { status: 404, code: 'NOT_FOUND', retryAfterMs: 5000 },
      { status: 503, code: 'DEPENDENCY_UNAVAILABLE', retryAfterMs: 5000 },
      { status: 401, code: 'AUTH_REQUIRED', retryAfterMs: 5000 },
      { status: 503, code: 'TOPIC_NOT_STARTED', retryAfterMs: 5000 },
      { status: 200, code: 'INVALID_SNAPSHOT', retryAfterMs: 5000 },
    ]) {
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
      const fixture = await snapshotOrigin();
      fixture.answer(failure);
      const consumer = streamClient(kind, fixture);
      try { await eventually(consumer.failed); expect(fixture.requests).toHaveLength(1); }
      finally { consumer.close(); await fixture.close(); vi.useRealTimers(); }
    }
  }
}
test(otherSnapshotFailuresKeepEachStreamsErrorState, otherSnapshotFailuresKeepEachStreamsErrorState);

async function topicNotStartedShowsEmptyTradesAndWaitingUpdates() {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  const fixture = await snapshotOrigin();
  const session = new MarketSession(fixture.client, pool);
  void session.start();
  const container = document.createElement('div');
  const root = createRoot(container);
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  try {
    await eventually(() => fixture.requests.length === 1, () => `market view: snapshot requests=${fixture.requests.length}`);
    const state = session.getSnapshot();
    const trade = decodeV2Contract('Trade', wire.trades.body.data.items[0]);
    await act(async () => root.render(<>
      <MarketTradeFeed pool={{ chainId: '125', poolAddress: pool, tokenAddress: trade.emittingContract }} token={{ address: trade.emittingContract, decimals: 6, decimalsQuality: 'verified' }} rows={state.rows} watermark={state.watermark} contiguous={false} />
      <FreshnessNotice freshness={session.status(Date.now())} stream={state.stream} error={state.error} onReconnect={() => { void session.start(); }} />
    </>));
    expect(container.textContent).toContain('No trades yet');
    expect(container.textContent).toContain('Waiting for trades');
    expect(container.textContent).not.toMatch(/TOPIC_NOT_STARTED|Updates off|Live updates are off/);
    expect(container.querySelector('[role="alert"]')).toBeNull();
    expect(container.querySelector('[data-freshness-chip="waiting"]')).not.toBeNull();
  } finally { await act(async () => root.unmount()); session.close(); await fixture.close(); }
}
test(topicNotStartedShowsEmptyTradesAndWaitingUpdates, topicNotStartedShowsEmptyTradesAndWaitingUpdates);
