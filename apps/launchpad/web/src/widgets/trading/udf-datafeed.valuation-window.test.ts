import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import { setTimeout as pause } from 'node:timers/promises';
import {
  browserStreamTransportV2,
  decodeV2Contract,
  SdkClient,
  type V2ContractTypes,
} from '@kindle/sdk';
import { expect, test } from 'vitest';
import {
  canonicalCandleToDisplayBar,
  createReplayUdfDatafeed,
  registerChartReplayTransport,
  resolutionToSeconds,
  resolutionToWsTimeframe,
  type ChartFreshness,
  type ValuationState,
} from './udf-datafeed';

const fixtures = new URL('../../core/test-fixtures/market-production/', import.meta.url);
const core = JSON.parse(readFileSync(new URL('core-api-wire.json', fixtures), 'utf8'));
const charts = JSON.parse(readFileSync(new URL('charts-wire.json', fixtures), 'utf8'));
const candlePage = decodeV2Contract('CandlesResponse', core.candles.body);
const snapshotPage = decodeV2Contract('TopicSnapshotReadResponse', core.snapshot.body);
const tradesPage = decodeV2Contract('TradesResponse', core.trades.body);
const valuationPage = JSON.parse(charts.valuationHistory.bodyBytes);
const candleTemplate = candlePage.data.items[0];
const pool = candleTemplate.poolAddress;
const day = 86400;
const bound = 366 * day;
const intervals = ['1', '5', '15', '60', '240', '1D', '1W'] as const;
type Candle = V2ContractTypes['Candle'];
type Feed = ReturnType<typeof createReplayUdfDatafeed>;
type Read = { resolution: string; from: number; to: number; status: number };
type Bar = ReturnType<typeof canonicalCandleToDisplayBar>;
interface PeerSocket {
  send(frame: string): void;
  terminate(): void;
  on(event: 'message', receive: (bytes: Buffer) => void): void;
}
const { WebSocketServer } = createRequire(import.meta.url)('ws') as {
  WebSocketServer: new (options: { server: ReturnType<typeof createServer> }) => {
    clients: Set<PeerSocket>;
    on(event: 'connection', receive: (socket: PeerSocket) => void): void;
    close(callback: () => void): void;
  };
};

function candleAt(resolution: string, start: number, revision = candleTemplate.revision): Candle {
  return decodeV2Contract('Candle', {
    ...candleTemplate,
    interval: resolutionToWsTimeframe(resolution),
    start: new Date(start * 1000).toISOString(),
    revision,
  });
}
function expectedBar(candle: Candle): Bar {
  const source = valuationPage.valuations[0].supply[0];
  return canonicalCandleToDisplayBar(candle, 'mcap', {
    raw: source.totalSupplyRaw,
    decimals: source.decimals,
  });
}
function history(feed: Feed, resolution: string, from: number, to: number) {
  return new Promise<{ bars: Bar[]; noData?: boolean }>((resolve, reject) => {
    feed.getBars(
      {},
      resolution,
      { from, to },
      (bars, meta) => resolve({ bars, ...meta }),
      (reason) => reject(new Error(reason))
    );
  });
}
async function until(check: () => boolean) {
  const deadline = performance.now() + 5000;
  while (!check()) {
    if (performance.now() >= deadline) throw new Error('Chart valuation state did not settle');
    await pause(10);
  }
}
function spans(reads: Read[]) {
  return reads.map((read) => ({ ...read, spanSeconds: read.to - read.from }));
}

/** The HTTP and WebSocket peer serves canonical records and the charts service's bounded valuation contract. */
async function chartsPeer() {
  let candles: Candle[] = [];
  let snapshotCandle = candleTemplate;
  let sequence = 1;
  let failNext = false;
  let held = false;
  let corrupt: 'snapshot' | 'identity' | 'supply' | undefined;
  const reads: Read[] = [];
  const parked: Array<() => void> = [];
  const subscriptions = new Map<PeerSocket, V2ContractTypes['Subscribe']>();
  const server = createServer((request, response) => {
    const url = new URL(request.url!, 'http://localhost');
    response.setHeader('Content-Type', 'application/json');
    response.setHeader('X-Schema-Version', '2');
    const json = (body: unknown) => response.end(JSON.stringify(body));
    if (url.pathname.endsWith('/candles')) {
      const from = Date.parse(url.searchParams.get('from')!) / 1000;
      const to = Date.parse(url.searchParams.get('to')!) / 1000;
      const matching = candles.filter(
        (candle) =>
          candle.interval === url.searchParams.get('interval') &&
          Date.parse(candle.start) / 1000 >= from &&
          Date.parse(candle.start) / 1000 < to
      );
      const offset = Number(url.searchParams.get('cursor') ?? 0);
      const limit = Number(url.searchParams.get('limit'));
      json(
        decodeV2Contract('CandlesResponse', {
          ...candlePage,
          data: { ...candlePage.data, items: matching.slice(offset, offset + limit) },
          nextCursor: offset + limit < matching.length ? String(offset + limit) : null,
        })
      );
      return;
    }
    if (url.pathname === '/v2/udf/history') {
      const from = Number(url.searchParams.get('from'));
      const to = Number(url.searchParams.get('to'));
      const resolution = url.searchParams.get('resolution')!;
      const status = to - from > bound || failNext ? 422 : 200;
      failNext = false;
      reads.push({ resolution, from, to, status });
      const matching = candles.filter(
        (candle) =>
          candle.interval === resolutionToWsTimeframe(resolution) &&
          Date.parse(candle.start) / 1000 >= from &&
          Date.parse(candle.start) / 1000 < to
      );
      const answer = () => {
        if (response.destroyed) return;
        response.statusCode = status;
        if (status !== 200) {
          json({
            code: 'FIELD_INVALID',
            message: 'invalid versioned history request',
            retryable: false,
            retryAfterMs: null,
          });
          return;
        }
        const body = {
          ...valuationPage,
          snapshotId:
            corrupt === 'snapshot'
              ? snapshotPage.data.snapshot.snapshotId
              : candlePage.data.snapshotId,
          t: matching.map(
            (candle) => Date.parse(candle.start) / 1000 + (corrupt === 'identity' ? 1 : 0)
          ),
          valuations: matching.map(() => structuredClone(valuationPage.valuations[0])),
        };
        if (corrupt === 'supply' && body.valuations.length)
          body.valuations[0].supply[0].supplySourceBlock = '1';
        json(body);
      };
      if (held) parked.push(answer);
      else answer();
      return;
    }
    if (url.pathname === '/v2/snapshots') {
      const topic = url.searchParams.get('topics')!;
      json(
        decodeV2Contract('TopicSnapshotReadResponse', {
          ...snapshotPage,
          data: {
            snapshot: {
              ...snapshotPage.data.snapshot,
              topics: [
                {
                  topic,
                  payload: snapshotCandle,
                  revision: snapshotCandle.revision,
                  sequence: String(sequence),
                  watermark: candlePage.watermark,
                },
              ],
            },
          },
        })
      );
      return;
    }
    if (url.pathname.endsWith('/trades')) {
      json(tradesPage);
      return;
    }
    response.writeHead(404);
    response.end('{}');
  });
  const sockets = new WebSocketServer({ server });
  const sendControl = (
    socket: PeerSocket,
    subscriptionId: string,
    op: 'subscribed' | 'checkpoint' | 'heartbeat'
  ) => {
    const recorded =
      op === 'heartbeat'
        ? JSON.parse(core.heartbeat.frameBytes)
        : core.replayFrames.find((frame: { op: string }) => frame.op === op);
    socket.send(
      JSON.stringify(
        decodeV2Contract('StreamControl', {
          ...recorded,
          subscriptionId,
          ...(op === 'subscribed'
            ? {
                topics: subscriptions
                  .get(socket)!
                  .topics.map((topic) => ({ topic, accepted: true, code: '' })),
              }
            : {}),
        })
      )
    );
  };
  sockets.on('connection', (socket) =>
    socket.on('message', (bytes) => {
      const frame = JSON.parse(bytes.toString());
      if (frame.op === 'ping') {
        sendControl(socket, subscriptions.get(socket)!.subscriptionId, 'heartbeat');
        return;
      }
      const subscribe = decodeV2Contract('Subscribe', frame);
      subscriptions.set(socket, subscribe);
      sendControl(socket, subscribe.subscriptionId, 'subscribed');
      sendControl(socket, subscribe.subscriptionId, 'checkpoint');
    })
  );
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  return {
    origin,
    reads,
    setCandles: (values: Candle[]) => {
      candles = values;
    },
    setSnapshot: (candle: Candle) => {
      snapshotCandle = candle;
    },
    fail: () => {
      failNext = true;
    },
    corrupt: (value: typeof corrupt) => {
      corrupt = value;
    },
    hold: () => {
      held = true;
    },
    release: () => {
      held = false;
      parked.splice(0).forEach((answer) => answer());
    },
    update: (candle: Candle) => {
      snapshotCandle = candle;
      candles = [
        ...candles.filter((old) => old.interval !== candle.interval || old.start !== candle.start),
        candle,
      ].sort((a, b) => Date.parse(a.start) - Date.parse(b.start));
      sequence++;
      for (const [socket, subscription] of subscriptions) {
        const topic = `market.candles.${pool}.${candle.interval}`;
        if (!subscription.topics.includes(topic)) continue;
        socket.send(
          JSON.stringify(
            decodeV2Contract('CandleEvent', {
              ...core.liveFrames[0],
              topic,
              payload: candle,
              revision: candle.revision,
              sequence: String(sequence),
              watermark: candlePage.watermark,
            })
          )
        );
      }
    },
    close: async () => {
      for (const socket of sockets.clients) socket.terminate();
      await new Promise<void>((resolve) => sockets.close(resolve));
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve()))
      );
    },
  };
}
function bind(peer: Awaited<ReturnType<typeof chartsPeer>>) {
  const streamOrigin = peer.origin.replace(/^http/, 'ws');
  const client = new SdkClient({
    apiOrigin: peer.origin,
    identityOrigin: peer.origin,
    mediaOrigin: peer.origin,
    streamOrigin,
    approvedStreamOrigins: [streamOrigin],
    production: false,
  });
  const authorization = new AbortController();
  const release = registerChartReplayTransport({
    transport: browserStreamTransportV2(
      client,
      { applicationOrigin: peer.origin, snapshotPath: '/v2/snapshots' },
      false
    ),
    signal: authorization.signal,
  });
  return () => {
    authorization.abort();
    release();
  };
}

async function marketCapHistoryStaysInsideTheServiceBound() {
  const peer = await chartsPeer();
  const states: ValuationState[] = [];
  const feed = createReplayUdfDatafeed(
    pool,
    'mcap',
    () => {},
    undefined,
    { apiOrigin: peer.origin, valuationOrigin: peer.origin },
    () => {},
    (state) => states.push(state)
  );
  const start = 1_600_128_000;
  const from = start - 1504 * day;
  const to = from + 2304 * day;
  try {
    const short = Array.from({ length: 10 }, (_, index) => candleAt('1W', start + index * 7 * day));
    peer.setCandles(short);
    expect((await history(feed, '1W', from, to)).bars).toEqual(short.map(expectedBar));
    expect(peer.reads).toEqual([
      { resolution: '1W', from: start, to: start + 70 * day, status: 200 },
    ]);
    const long = [0, 364, 700, 793].map((offset) => candleAt('1W', start + offset * day));
    peer.setCandles(long);
    expect((await history(feed, '1W', from, to)).bars).toEqual(long.map(expectedBar));
    expect(peer.reads.slice(1)).toEqual([
      { resolution: '1W', from: start, to: start + 366 * day, status: 200 },
      { resolution: '1W', from: start + 366 * day, to: start + 732 * day, status: 200 },
      { resolution: '1W', from: start + 732 * day, to: start + 800 * day, status: 200 },
    ]);
    expect(peer.reads.every((read) => read.to - read.from <= bound)).toBe(true);
    expect(states.every((state) => state.state === 'ready')).toBe(true);
    console.info('Weekly valuation windows', spans(peer.reads));
  } finally {
    feed.dispose();
    await peer.close();
  }
}
test(marketCapHistoryStaysInsideTheServiceBound, marketCapHistoryStaysInsideTheServiceBound);

async function marketCapIntervalChangeKeepsAnEmptyWindowLive() {
  const peer = await chartsPeer();
  const release = bind(peer);
  const states: ValuationState[] = [];
  const freshness: ChartFreshness[] = [];
  const ticks: Bar[] = [];
  const feed = createReplayUdfDatafeed(
    pool,
    'mcap',
    (value) => freshness.push(value),
    undefined,
    { apiOrigin: peer.origin, valuationOrigin: peer.origin },
    () => {},
    (state) => states.push(state)
  );
  const now = Math.floor(Date.now() / 1000);
  const weekly = candleAt('1W', now - 7 * day);
  const live = candleAt('5', Math.floor((now - day) / 300) * 300);
  try {
    peer.setCandles([weekly]);
    peer.fail();
    await expect(history(feed, '1W', now - 30 * day, now)).rejects.toThrow();
    expect(states.filter((state) => state.state === 'error')).toHaveLength(1);
    expect(peer.reads).toHaveLength(1);
    expect(peer.reads[0].status).toBe(422);
    peer.setCandles([live]);
    peer.setSnapshot(live);
    const empty = await history(feed, '5', now - 900, now + 60);
    expect(empty).toEqual({ bars: [], noData: true });
    expect(states.at(-1)).toEqual({ state: 'ready' });
    expect(peer.reads).toHaveLength(1);
    peer.hold();
    feed.subscribeBars({}, '5', (bar) => ticks.push(bar), 'chart');
    await until(() => feed.replayStatus()[0]?.replayComplete === true && peer.reads.length === 2);
    expect(freshness.at(-1)?.connectionStatus).toBe('live');
    expect(ticks).toEqual([]);
    expect(states.at(-1)).toEqual({ state: 'ready' });
    expect(peer.reads[1]).toEqual({
      resolution: '5',
      from: Date.parse(live.start) / 1000,
      to: Date.parse(live.start) / 1000 + 300,
      status: 200,
    });
    peer.release();
    await until(() => ticks.length === 1);
    expect(ticks).toEqual([expectedBar(live)]);
    expect(freshness.at(-1)?.connectionStatus).toBe('live');
    expect(
      freshness.some((value) => ['offline', 'resync_required'].includes(value.connectionStatus))
    ).toBe(false);
    console.info('Valuation interval recovery', spans(peer.reads));
  } finally {
    feed.dispose();
    release();
    await peer.close();
  }
}
test(
  marketCapIntervalChangeKeepsAnEmptyWindowLive,
  marketCapIntervalChangeKeepsAnEmptyWindowLive,
  15000
);

async function marketCapEmptyWindowsNeedNoValuation() {
  const peer = await chartsPeer();
  const now = Math.floor(Date.now() / 1000);
  try {
    for (const resolution of intervals) {
      const states: ValuationState[] = [];
      const feed = createReplayUdfDatafeed(
        pool,
        'mcap',
        () => {},
        undefined,
        { apiOrigin: peer.origin, valuationOrigin: peer.origin },
        () => {},
        (state) => states.push(state)
      );
      try {
        expect(
          await history(feed, resolution, now - 50 * resolutionToSeconds(resolution), now)
        ).toEqual({ bars: [], noData: true });
        expect(states).toEqual([]);
        expect(peer.reads).toEqual([]);
        console.info('Empty valuation window', { resolution, reads: 0 });
      } finally {
        feed.dispose();
      }
    }
  } finally {
    await peer.close();
  }
}
test(marketCapEmptyWindowsNeedNoValuation, marketCapEmptyWindowsNeedNoValuation);

async function marketCapHistoryValuesTheCandleSpanAtEveryInterval() {
  const peer = await chartsPeer();
  const start = 1_600_128_000;
  try {
    for (const resolution of intervals) {
      const seconds = resolutionToSeconds(resolution);
      const candles = [0, 2].map((offset) => candleAt(resolution, start + offset * seconds));
      peer.setCandles(candles);
      const feed = createReplayUdfDatafeed(pool, 'mcap', () => {}, undefined, {
        apiOrigin: peer.origin,
        valuationOrigin: peer.origin,
      });
      const before = peer.reads.length;
      try {
        expect(
          (await history(feed, resolution, start - 100 * seconds, start + 100 * seconds)).bars
        ).toEqual(candles.map(expectedBar));
        expect(peer.reads.slice(before)).toEqual([
          { resolution, from: start, to: start + 3 * seconds, status: 200 },
        ]);
        console.info('Candle valuation span', spans(peer.reads.slice(before)));
      } finally {
        feed.dispose();
      }
    }
  } finally {
    await peer.close();
  }
}
test(
  marketCapHistoryValuesTheCandleSpanAtEveryInterval,
  marketCapHistoryValuesTheCandleSpanAtEveryInterval
);

async function marketCapLiveValuationFailureWaitsForRetry() {
  const peer = await chartsPeer();
  const release = bind(peer);
  const states: ValuationState[] = [];
  const ticks: Bar[] = [];
  const freshness: ChartFreshness[] = [];
  const now = Math.floor(Date.now() / 1000);
  const candle = candleAt('1', Math.floor((now - day) / 60) * 60);
  peer.setCandles([candle]);
  peer.setSnapshot(candle);
  const feed = createReplayUdfDatafeed(
    pool,
    'mcap',
    (value) => freshness.push(value),
    undefined,
    { apiOrigin: peer.origin, valuationOrigin: peer.origin },
    () => {},
    (state) => states.push(state)
  );
  try {
    await history(feed, '1', Date.parse(candle.start) / 1000, now);
    feed.subscribeBars({}, '1', (bar) => ticks.push(bar), 'chart');
    await until(() => feed.replayStatus()[0]?.replayComplete === true);
    expect(ticks).toEqual([expectedBar(candle)]);
    peer.hold();
    peer.fail();
    const correction = candleAt('1', Date.parse(candle.start) / 1000, '3');
    correction.volumeUsdlRaw = '2000000';
    peer.update(correction);
    await until(() => peer.reads.length === 2);
    expect(freshness.at(-1)?.connectionStatus).toBe('live');
    expect(ticks).toHaveLength(1);
    peer.release();
    await until(() => states.filter((state) => state.state === 'error').length === 1);
    await pause(12000);
    expect(peer.reads).toHaveLength(2);
    expect(peer.reads[1].to - peer.reads[1].from).toBe(60);
    expect(states.filter((state) => state.state === 'error')).toHaveLength(1);
    expect(freshness.at(-1)?.connectionStatus).toBe('live');
    expect(
      freshness.some((value) => ['offline', 'resync_required'].includes(value.connectionStatus))
    ).toBe(false);
    feed.retry();
    await until(() => ticks.at(-1)?.volume === 2);
    expect(peer.reads).toHaveLength(3);
    expect(peer.reads[2].to - peer.reads[2].from).toBe(60);
    expect(states.at(-1)).toEqual({ state: 'ready' });
    expect(ticks.at(-1)).toEqual(expectedBar(correction));
    console.info('Live candle valuation and retry', spans(peer.reads));
  } finally {
    feed.dispose();
    release();
    await peer.close();
  }
}
test(marketCapLiveValuationFailureWaitsForRetry, marketCapLiveValuationFailureWaitsForRetry, 25000);

async function marketCapWindowsKeepSnapshotIdentityAndSupplyChecks() {
  const peer = await chartsPeer();
  const start = 1_600_128_000;
  peer.setCandles([candleAt('1W', start)]);
  try {
    for (const corruption of ['snapshot', 'identity', 'supply'] as const) {
      peer.corrupt(corruption);
      const states: ValuationState[] = [];
      const feed = createReplayUdfDatafeed(
        pool,
        'mcap',
        () => {},
        undefined,
        { apiOrigin: peer.origin, valuationOrigin: peer.origin },
        () => {},
        (state) => states.push(state)
      );
      try {
        await expect(history(feed, '1W', start, start + 7 * day)).rejects.toThrow();
        expect(states.map((state) => state.state)).toEqual(['error']);
      } finally {
        feed.dispose();
      }
    }
  } finally {
    await peer.close();
  }
}
test(
  marketCapWindowsKeepSnapshotIdentityAndSupplyChecks,
  marketCapWindowsKeepSnapshotIdentityAndSupplyChecks
);
