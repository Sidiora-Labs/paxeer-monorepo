import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { createServer, request } from 'node:http';
import { performance as wallClock } from 'node:perf_hooks';
import type { Duplex } from 'node:stream';
import { setTimeout as wait } from 'node:timers/promises';
import { JSDOM } from 'jsdom';
import { expect, test, vi } from 'vitest';
import { getAddress } from 'viem';
import { browserStreamTransportV2, SdkClient, type StreamTransportV2 } from '@kindle/sdk';
import { marketRetryPace } from '@/core/retry-pace';
import {
  canonicalCandleToDisplayBar,
  createReplayUdfDatafeed,
  registerChartReplayTransport,
  type ChartAccessibleView,
  type ChartFreshness,
} from './udf-datafeed';

function object(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value))
    throw new Error('Expected chart service configuration');
  return Object.fromEntries(Object.entries(value));
}

function text(value: unknown): string {
  if (typeof value !== 'string' || !value) throw new Error('Missing chart service field');
  return value;
}

function services() {
  const bytes = readFileSync(text(process.env.JOINED_CONFIG_FILE));
  expect(createHash('sha256').update(bytes).digest('hex')).toBe(process.env.JOINED_CONFIG_SHA256);
  const config = object(JSON.parse(bytes.toString()));
  const chain = object(JSON.parse(readFileSync(text(process.env.JOINED_CHAIN_FILE), 'utf8')));
  expect(chain.realm).toBe('private-anvil125');
  expect(chain.productionActivation).toBe(false);
  expect(process.env.JOINED_ALLOW_CHAIN_MUTATION).toBe('1');
  if (!Array.isArray(config.processes)) throw new Error('Missing chart services');
  const processes = config.processes.map(object);
  const origin = (role: string) => {
    const service = processes.find(entry => entry.role === role);
    if (!service) throw new Error('Missing chart service');
    return new URL(text(service.ready_url)).origin;
  };
  const roles = object(object(chain.manifest).roles);
  return {
    api: new URL(text(config.api_origin)).origin,
    charts: origin('trading-charts'),
    sockets: origin('api').replace(/^http/, 'ws'),
    application: text(config.stream_application_origin),
    rpc: text(config.rpc_url),
    pool: getAddress(text(chain.pool)),
    account: getAddress(text(chain.account)),
    router: getAddress(text(object(roles.Router).address)),
  };
}

type Services = ReturnType<typeof services>;

async function chartOriginBridge(actual: Services) {
  const sockets = new Set<Duplex>();
  const headers = (incoming: import('node:http').IncomingMessage, target: URL) => ({
    ...incoming.headers, host: target.host, origin: actual.application,
  });
  const server = createServer((incoming, outgoing) => {
    const target = new URL(incoming.url ?? '/', actual.api);
    const upstream = request(target, { method: incoming.method, headers: headers(incoming, target) }, reply => {
      if (reply.statusCode === undefined) { outgoing.destroy(); return; }
      outgoing.writeHead(reply.statusCode, reply.headers);
      reply.pipe(outgoing);
    });
    upstream.on('error', () => outgoing.destroy());
    outgoing.on('close', () => upstream.destroy());
    incoming.pipe(upstream);
  });
  server.on('connection', socket => {
    sockets.add(socket);
    socket.on('close', () => sockets.delete(socket));
  });
  server.on('upgrade', (incoming, socket, head) => {
    const target = new URL(incoming.url ?? '/v2/stream', actual.sockets.replace(/^ws/, 'http'));
    const upstream = request(target, { headers: headers(incoming, target) });
    upstream.on('upgrade', (reply, peer, upstreamHead) => {
      sockets.add(peer);
      socket.on('close', () => peer.destroy());
      peer.on('close', () => { sockets.delete(peer); socket.destroy(); });
      socket.on('error', () => peer.destroy());
      peer.on('error', () => socket.destroy());
      socket.write(`HTTP/1.1 ${reply.statusCode} ${reply.statusMessage}\r\n${Object.entries(reply.headers)
        .map(([key, value]) => `${key}: ${Array.isArray(value) ? value.join(', ') : value}`)
        .join('\r\n')}\r\n\r\n`);
      if (upstreamHead.length) socket.write(upstreamHead);
      if (head.length) peer.write(head);
      socket.pipe(peer);
      peer.pipe(socket);
    });
    upstream.on('response', reply => { reply.resume(); socket.destroy(); });
    upstream.on('error', () => socket.destroy());
    upstream.end();
  });
  server.listen(0, new URL(actual.api).hostname);
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Chart stream listener required');
  const origin = new URL(actual.api);
  origin.port = String(address.port);
  return {
    origin: origin.origin,
    close: async () => {
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    },
  };
}

async function eventually(predicate: () => boolean | Promise<boolean>) {
  const end = wallClock.now() + 45000;
  while (!(await predicate())) {
    if (wallClock.now() >= end) throw new Error('Chart delivery timed out');
    await wait(10);
  }
}

class ReconnectingTransport implements StreamTransportV2 {
  starts: number[] = [];
  topics: string[][] = [];
  refusedStarts = 0;
  closeFromFarSide?: () => void;

  constructor(private readonly source: StreamTransportV2) {}

  snapshot: StreamTransportV2['snapshot'] = (topics, signal) =>
    this.source.snapshot(topics, signal);

  connect: StreamTransportV2['connect'] = async input => {
    this.starts.push(Date.now());
    if (this.refusedStarts > 0) {
      this.refusedStarts--;
      throw new Error('Chart stream connection unavailable');
    }
    const connection = await this.source.connect(input);
    this.closeFromFarSide = () => {
      connection.close();
      input.disconnected();
    };
    return {
      close: () => connection.close(),
      send: message => {
        if (message.op === 'subscribe') this.topics.push(message.topics);
        connection.send(message);
      },
    };
  };
}

async function reconnectingChart() {
  const actual = services();
  const bridge = await chartOriginBridge(actual);
  const dom = new JSDOM('', { url: actual.api, pretendToBeVisual: true });
  const previousDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
  let visibility: DocumentVisibilityState = 'visible';
  Object.defineProperty(dom.window.document, 'visibilityState', { get: () => visibility });
  Object.defineProperty(globalThis, 'document', { configurable: true, value: dom.window.document });
  Object.defineProperty(globalThis, 'window', { configurable: true, value: dom.window });
  marketRetryPace.clear();
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
  const client = new SdkClient({
    apiOrigin: bridge.origin,
    identityOrigin: bridge.origin,
    mediaOrigin: bridge.origin,
    streamOrigin: bridge.origin.replace(/^http/, 'ws'),
    approvedStreamOrigins: [bridge.origin.replace(/^http/, 'ws')],
    production: false,
  });
  const transport = new ReconnectingTransport(browserStreamTransportV2(client, {
    applicationOrigin: actual.application,
    snapshotPath: '/v2/snapshots',
  }, false));
  const authorization = new AbortController();
  const release = registerChartReplayTransport({ transport, signal: authorization.signal });
  const states: ChartFreshness[] = [];
  const ticks: ReturnType<typeof canonicalCandleToDisplayBar>[] = [];
  const views: ChartAccessibleView[] = [];
  let resets = 0;
  let loads = 0;
  const feed = createReplayUdfDatafeed(actual.pool, 'price', value => states.push(value), undefined, {
    apiOrigin: actual.api,
    valuationOrigin: actual.charts,
  }, value => views.push(value));
  const state = () => feed.replayStatus()[0]?.freshness;
  return {
    feed,
    transport,
    states,
    ticks,
    views,
    state,
    resets: () => resets,
    loads: () => loads,
    topic: `market.candles.${actual.pool.toLowerCase()}.1d`,
    loadChartData: async () => {
      loads++;
      const now = Math.floor(Date.now() / 1000);
      await new Promise<void>((resolve, reject) => feed.getBars({}, '1D', {
        from: now - 3 * 86400,
        to: now + 60,
      }, bars => { ticks.push(...bars); resolve(); }, reason => reject(new Error(reason))));
      feed.subscribeBars({}, '1D', bar => ticks.push(bar), 'chart', () => {
        resets++;
        feed.unsubscribeBars('chart');
      });
    },
    live: () => eventually(() => state()?.connectionStatus === 'live'),
    visibility: (next: DocumentVisibilityState) => {
      visibility = next;
      dom.window.document.dispatchEvent(new dom.window.Event('visibilitychange'));
    },
    online: () => dom.window.dispatchEvent(new dom.window.Event('online')),
    close: async () => {
      feed.dispose();
      authorization.abort();
      release();
      marketRetryPace.clear();
      vi.useRealTimers();
      if (previousDocument) Object.defineProperty(globalThis, 'document', previousDocument);
      else Reflect.deleteProperty(globalThis, 'document');
      if (previousWindow) Object.defineProperty(globalThis, 'window', previousWindow);
      else Reflect.deleteProperty(globalThis, 'window');
      dom.window.close();
      await bridge.close();
    },
  };
}

async function chartReturnsToLiveAfterACloseFromTheFarSide() {
  const chart = await reconnectingChart();
  try {
    await chart.loadChartData();
    await chart.live();
    const drawn = chart.ticks.slice();
    const retained = chart.views.at(-1)?.rows;
    expect(drawn.length).toBeGreaterThan(0);
    expect(chart.transport.topics).toEqual([[chart.topic]]);
    expect(chart.transport.closeFromFarSide).toBeTypeOf('function');
    chart.transport.closeFromFarSide!();
    expect(chart.state()?.connectionStatus).toBe('replaying');
    expect(chart.feed.resourceCounts().subscriptions).toBe(1);
    expect(chart.ticks).toEqual(drawn);
    expect(chart.views.at(-1)?.rows).toEqual(retained);
    await vi.advanceTimersByTimeAsync(1000);
    await chart.live();
    expect(chart.transport.starts).toHaveLength(2);
    expect(chart.transport.topics).toEqual([[chart.topic], [chart.topic]]);
    expect(chart.ticks.slice(0, drawn.length)).toEqual(drawn);
    expect(chart.views.at(-1)?.rows).toEqual(retained);
    expect(chart.resets()).toBe(0);
    expect(chart.loads()).toBe(1);
  } finally { await chart.close(); }
}
test(chartReturnsToLiveAfterACloseFromTheFarSide, chartReturnsToLiveAfterACloseFromTheFarSide, 120000);

async function chartKeepsTryingPastFiveFailedStarts() {
  const chart = await reconnectingChart();
  try {
    chart.transport.refusedStarts = 7;
    await chart.loadChartData();
    const waits = [1000, 2000, 4000, 8000, 16000, 30000, 30000];
    for (let failure = 1; failure <= 7; failure++) {
      await eventually(() => chart.transport.starts.length === failure &&
        chart.state()?.connectionStatus === 'replaying' && chart.state()?.lagReason !== null);
      expect(chart.resets()).toBe(0);
      expect(chart.feed.resourceCounts().subscriptions).toBe(1);
      await vi.advanceTimersByTimeAsync(waits[failure - 1] - 1);
      expect(chart.transport.starts).toHaveLength(failure);
      await vi.advanceTimersByTimeAsync(1);
    }
    await chart.live();
    expect(chart.transport.starts).toHaveLength(8);
    expect(chart.transport.starts.slice(1).map((start, index) =>
      start - chart.transport.starts[index])).toEqual(waits);
    expect(waits.every(delay => delay <= 30000)).toBe(true);
    expect(chart.states.some(value => ['closed', 'degraded', 'offline'].includes(value.connectionStatus))).toBe(false);
    expect(chart.transport.topics).toEqual([[chart.topic]]);
    expect(chart.loads()).toBe(1);
    chart.feed.unsubscribeBars('chart');
    expect(chart.feed.resourceCounts().subscriptions).toBe(0);
    expect(chart.feed.resourceCounts().timers).toBe(0);
    await vi.advanceTimersByTimeAsync(30000);
    expect(chart.transport.starts).toHaveLength(8);
  } finally { await chart.close(); }
}
test(chartKeepsTryingPastFiveFailedStarts, chartKeepsTryingPastFiveFailedStarts, 120000);

async function chartStartsAgainWhenThePageIsVisibleAgain() {
  const chart = await reconnectingChart();
  try {
    await chart.loadChartData();
    await chart.live();
    chart.visibility('hidden');
    chart.transport.closeFromFarSide!();
    expect(chart.state()?.connectionStatus).toBe('replaying');
    const closedAt = Date.now();
    chart.visibility('visible');
    await chart.live();
    expect(chart.transport.starts).toHaveLength(2);
    expect(chart.transport.starts[1]).toBe(closedAt);
    expect(chart.transport.topics).toEqual([[chart.topic], [chart.topic]]);
    chart.visibility('hidden');
    chart.transport.closeFromFarSide!();
    chart.online();
    await chart.live();
    expect(chart.transport.starts).toHaveLength(3);
    expect(chart.transport.starts[2]).toBe(closedAt);
    expect(chart.resets()).toBe(0);
    expect(chart.loads()).toBe(1);
  } finally { await chart.close(); }
}
test(chartStartsAgainWhenThePageIsVisibleAgain, chartStartsAgainWhenThePageIsVisibleAgain, 120000);
