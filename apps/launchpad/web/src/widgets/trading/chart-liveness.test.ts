import { createHash } from 'node:crypto';
import { once } from 'node:events';
import { readFileSync } from 'node:fs';
import { createServer, request } from 'node:http';
import { performance as wallClock } from 'node:perf_hooks';
import type { Duplex } from 'node:stream';
import { setTimeout as wait } from 'node:timers/promises';
import { JSDOM } from 'jsdom';
import { expect, test, vi } from 'vitest';
import { encodeFunctionData, getAddress, parseAbi } from 'viem';
import {
  browserStreamTransportV2,
  decodeV2Contract,
  SdkClient,
  type StreamTransportV2,
  type V2ContractTypes,
} from '@kindle/sdk';
import { marketRetryPace } from '@/core/retry-pace';
import { HEARTBEAT_LIVENESS_MS } from '@/page-modules/token/market-presentation';
import {
  canonicalCandleToDisplayBar,
  createReplayUdfDatafeed,
  registerChartReplayTransport,
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

async function rpc(actual: Services, method: string, params: unknown[]): Promise<unknown> {
  const response = await fetch(actual.rpc, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });
  expect(response.ok).toBe(true);
  const result = object(await response.json());
  expect(result.error).toBeUndefined();
  return result.result;
}

async function buy(actual: Services) {
  expect(await rpc(actual, 'eth_chainId', [])).toBe('0x7d');
  const block = object(await rpc(actual, 'eth_getBlockByNumber', ['latest', false]));
  const data = encodeFunctionData({
    abi: parseAbi([
      'function buy(address pool,uint256 usdlAmountIn,uint256 minTokensOut,uint256 deadline) returns (uint256)',
    ]),
    functionName: 'buy',
    args: [actual.pool, 1000000n, 1n, BigInt(text(block.timestamp)) + 600n],
  });
  const call = { from: actual.account, to: actual.router, data };
  const estimatedGas = BigInt(text(await rpc(actual, 'eth_estimateGas', [call])));
  const transaction = text(await rpc(actual, 'eth_sendTransaction', [{
    ...call,
    gas: '0x' + ((estimatedGas * 5n + 3n) / 4n).toString(16),
  }]));
  await eventually(async () => {
    const receipt = await rpc(actual, 'eth_getTransactionReceipt', [transaction]);
    if (receipt === null) return false;
    expect(object(receipt).status).toBe('0x1');
    return true;
  });
  return transaction;
}

class DeliveryWindow implements StreamTransportV2 {
  connections = 0;
  heartbeats = 0;
  events: V2ContractTypes['CandleEvent'][] = [];
  snapshots: V2ContractTypes['TopicSnapshot'][] = [];
  heldCheckpoints: Array<() => void> = [];
  holdCheckpoints = false;
  silent = false;
  blockHeartbeats = false;
  ageSnapshot = false;
  corruptEvent?: (event: V2ContractTypes['CandleEvent']) => V2ContractTypes['CandleEvent'];

  constructor(private readonly source: StreamTransportV2) {}

  snapshot: StreamTransportV2['snapshot'] = async (topics, signal) => {
    const snapshot = decodeV2Contract('TopicSnapshot', await this.source.snapshot(topics, signal));
    this.snapshots.push(snapshot);
    if (this.ageSnapshot) {
      const watermark = snapshot.topics[0].watermark;
      if (!watermark) throw new Error('Candle source watermark required');
      vi.setSystemTime(Date.parse(watermark.observedAt) + 10 * 3600000);
    }
    return snapshot;
  };

  connect: StreamTransportV2['connect'] = async input => {
    this.connections++;
    return this.source.connect({
      ...input,
      receive: value => {
        if (value !== null && typeof value === 'object' && 'op' in value) {
          const control = decodeV2Contract('StreamControl', value);
          if (control.op === 'heartbeat') {
            this.heartbeats++;
            if (this.blockHeartbeats) return;
          }
          if (this.silent) return;
          if (control.op === 'checkpoint' && this.holdCheckpoints) {
            this.heldCheckpoints.push(() => input.receive(value));
            return;
          }
          input.receive(value);
          return;
        }
        const event = decodeV2Contract('CandleEvent', value);
        this.events.push(event);
        if (!this.silent) input.receive(this.corruptEvent ? this.corruptEvent(event) : event);
      },
    });
  };

  releaseCheckpoints() {
    this.holdCheckpoints = false;
    this.heldCheckpoints.splice(0).forEach(deliver => deliver());
  }
}

async function chart(actual: Services) {
  const bridge = await chartOriginBridge(actual);
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
  const delivery = new DeliveryWindow(browserStreamTransportV2(client, {
    applicationOrigin: actual.application,
    snapshotPath: '/v2/snapshots',
  }, false));
  const authorization = new AbortController();
  const release = registerChartReplayTransport({ transport: delivery, signal: authorization.signal });
  const states: ChartFreshness[] = [];
  const ticks: ReturnType<typeof canonicalCandleToDisplayBar>[] = [];
  let resets = 0;
  const feed = createReplayUdfDatafeed(actual.pool, 'price', value => states.push(value), undefined, {
    apiOrigin: actual.api,
    valuationOrigin: actual.charts,
  });
  return {
    delivery,
    states,
    ticks,
    feed,
    resets: () => resets,
    state: () => feed.replayStatus()[0].freshness,
    start: () => feed.subscribeBars({}, '1D', bar => ticks.push(bar), 'liveness', () => resets++),
    live: () => eventually(() => feed.replayStatus()[0]?.freshness.connectionStatus === 'live'),
    close: async () => {
      feed.dispose();
      authorization.abort();
      release();
      marketRetryPace.clear();
      vi.useRealTimers();
      await bridge.close();
    },
  };
}

async function idleCandleRemainsLiveWithHeartbeats() {
  const current = await chart(services());
  try {
    current.delivery.ageSnapshot = true;
    current.start();
    await current.live();
    const watermark = current.delivery.snapshots[0].topics[0].watermark;
    if (!watermark) throw new Error('Candle source watermark required');
    expect(Date.now() - Date.parse(watermark.observedAt)).toBe(10 * 3600000);
    expect(current.state()).toMatchObject({
      connectionStatus: 'live', lagReason: null,
      sourceObservedAt: watermark.observedAt, lastSourceHeight: watermark.height,
    });
    const candle = decodeV2Contract('Candle', current.delivery.snapshots[0].topics[0].payload);
    expect(current.ticks.at(-1)).toEqual(canonicalCandleToDisplayBar(candle));
    const reported = current.states.length;
    for (let elapsed = 0; elapsed < 60000; elapsed += 4000) {
      const heartbeats = current.delivery.heartbeats;
      await vi.advanceTimersByTimeAsync(4000);
      await eventually(() => current.delivery.heartbeats > heartbeats);
      expect(current.state().connectionStatus).toBe('live');
      expect(current.state().lagReason).toBeNull();
      expect(current.state().lastReceivedAt).toBe(Date.now());
      expect(current.feed.resourceCounts().timers).toBe(1);
    }
    expect(current.delivery.events).toHaveLength(0);
    expect(current.delivery.connections).toBe(1);
    expect(current.states.slice(reported).every(value => value.connectionStatus === 'live')).toBe(true);
    expect(current.state().sourceObservedAt).toBe(watermark.observedAt);
    expect(current.state().lastSourceHeight).toBe(watermark.height);
  } finally { await current.close(); }
}
test(idleCandleRemainsLiveWithHeartbeats, idleCandleRemainsLiveWithHeartbeats, 120000);

async function candleDeliveryRenewsLivenessWithoutHeartbeats() {
  const actual = services();
  const current = await chart(actual);
  try {
    current.delivery.blockHeartbeats = true;
    current.start();
    await current.live();
    const height = BigInt(text(current.state().lastSourceHeight));
    await vi.advanceTimersByTimeAsync(9000);
    const transaction = await buy(actual);
    await eventually(() => current.delivery.events.some(event =>
      event.payload.lastPosition.transactionHash.toLowerCase() === transaction.toLowerCase()));
    expect(BigInt(text(current.state().lastSourceHeight))).toBeGreaterThan(height);
    expect(current.state().lastReceivedAt).toBe(Date.now());
    await vi.advanceTimersByTimeAsync(9000);
    expect(current.state().connectionStatus).toBe('live');
    expect(current.delivery.connections).toBe(1);
  } finally { await current.close(); }
}
test(candleDeliveryRenewsLivenessWithoutHeartbeats, candleDeliveryRenewsLivenessWithoutHeartbeats, 120000);

async function heartbeatWaitsForReplayCheckpoint() {
  const current = await chart(services());
  try {
    current.delivery.holdCheckpoints = true;
    current.start();
    await eventually(() => current.delivery.heldCheckpoints.length > 0);
    for (let count = 0; count < 3; count++) {
      const heartbeats = current.delivery.heartbeats;
      await vi.advanceTimersByTimeAsync(4000);
      await eventually(() => current.delivery.heartbeats > heartbeats);
      expect(current.state().connectionStatus).toBe('replaying');
      expect(current.feed.resourceCounts().timers).toBe(0);
      expect(current.states.some(value => value.connectionStatus === 'live')).toBe(false);
    }
    current.delivery.releaseCheckpoints();
    await current.live();
    expect(current.state().lagReason).toBeNull();
    expect(current.feed.resourceCounts().timers).toBe(1);
  } finally { await current.close(); }
}
test(heartbeatWaitsForReplayCheckpoint, heartbeatWaitsForReplayCheckpoint, 120000);

async function missingFramesReconnectAndReachCheckpoint() {
  const current = await chart(services());
  try {
    current.start();
    await current.live();
    current.delivery.silent = true;
    await vi.advanceTimersByTimeAsync(HEARTBEAT_LIVENESS_MS - 1000);
    expect(current.state().connectionStatus).toBe('live');
    await vi.advanceTimersByTimeAsync(1000);
    expect(current.state()).toMatchObject({
      connectionStatus: 'replaying', lagReason: 'Live updates are reconnecting',
    });
    expect(current.resets()).toBe(0);
    expect(current.feed.resourceCounts().sockets).toBe(0);
    current.delivery.silent = false;
    current.delivery.holdCheckpoints = true;
    await vi.advanceTimersByTimeAsync(1000);
    await eventually(() => current.delivery.connections === 2 && current.delivery.heldCheckpoints.length > 0);
    expect(current.state().connectionStatus).toBe('replaying');
    current.delivery.releaseCheckpoints();
    await current.live();
    expect(current.state().lagReason).toBeNull();
    expect(current.feed.resourceCounts().timers).toBe(1);
  } finally { await current.close(); }
}
test(missingFramesReconnectAndReachCheckpoint, missingFramesReconnectAndReachCheckpoint, 120000);

async function invalidCandleWatermarksRequireRecovery() {
  const actual = services();
  for (const integrity of ['healthy', 'quarantined'] as const) {
    const current = await chart(actual);
    try {
      current.start();
      await current.live();
      const heldHeight = BigInt(text(current.state().lastSourceHeight));
      current.delivery.corruptEvent = event => {
        if (!event.watermark) throw new Error('Candle event watermark required');
        return { ...event, watermark: {
          ...event.watermark,
          height: integrity === 'healthy' ? (heldHeight - 1n).toString() : event.watermark.height,
          integrity,
        } };
      };
      const transaction = await buy(actual);
      await eventually(() => current.delivery.events.some(event =>
        event.payload.lastPosition.transactionHash.toLowerCase() === transaction.toLowerCase()));
      expect(current.resets()).toBe(0);
      expect(current.state().connectionStatus).toBe('replaying');
      expect(current.feed.resourceCounts().sockets).toBe(0);
      expect(current.states.some(value => value.connectionStatus === 'resync_required')).toBe(true);
    } finally { await current.close(); }
  }
}
test(invalidCandleWatermarksRequireRecovery, invalidCandleWatermarksRequireRecovery, 120000);

async function hiddenChartSuspendsLivenessAndReleasesTimers() {
  const actual = services();
  const dom = new JSDOM('', { url: actual.api, pretendToBeVisual: true });
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'document');
  let visibility: DocumentVisibilityState = 'visible';
  Object.defineProperty(dom.window.document, 'visibilityState', { get: () => visibility });
  Object.defineProperty(globalThis, 'document', { configurable: true, value: dom.window.document });
  const current = await chart(actual);
  const changeVisibility = (next: DocumentVisibilityState) => {
    visibility = next;
    dom.window.document.dispatchEvent(new dom.window.Event('visibilitychange'));
  };
  try {
    current.start();
    await current.live();
    expect(current.feed.resourceCounts().timers).toBe(1);
    changeVisibility('hidden');
    expect(current.feed.resourceCounts().timers).toBe(0);
    const heartbeats = current.delivery.heartbeats;
    await vi.advanceTimersByTimeAsync(4000);
    await eventually(() => current.delivery.heartbeats > heartbeats);
    expect(current.feed.resourceCounts().timers).toBe(0);
    current.delivery.silent = true;
    const reported = current.states.length;
    await vi.advanceTimersByTimeAsync(60000);
    expect(current.states.slice(reported)).toHaveLength(0);
    expect(current.state().connectionStatus).toBe('live');
    expect(current.delivery.connections).toBe(1);
    current.delivery.silent = false;
    changeVisibility('visible');
    await eventually(() => current.delivery.connections === 2);
    await current.live();
    expect(current.feed.resourceCounts().timers).toBe(1);
    current.feed.unsubscribeBars('liveness');
    expect(current.feed.resourceCounts().timers).toBe(0);
    expect(current.feed.resourceCounts().sockets).toBe(0);
    current.start();
    await current.live();
    expect(current.feed.resourceCounts().timers).toBe(1);
    current.feed.dispose();
    expect(current.feed.resourceCounts()).toEqual({ subscriptions: 0, requests: 0, sockets: 0, timers: 0 });
    await vi.advanceTimersByTimeAsync(60000);
    expect(current.delivery.connections).toBe(3);
  } finally {
    await current.close();
    if (previous) Object.defineProperty(globalThis, 'document', previous);
    else Reflect.deleteProperty(globalThis, 'document');
    dom.window.close();
  }
}
test(hiddenChartSuspendsLivenessAndReleasesTimers, hiddenChartSuspendsLivenessAndReleasesTimers, 120000);

async function successfulPollRestoresPausedUpdates() {
  const actual = services();
  const upstream = new URL(actual.api);
  let unavailable = true;
  let interrupted = 0;
  let successful = 0;
  const server = createServer((incoming, outgoing) => {
    const url = new URL(incoming.url ?? '/', upstream);
    if (unavailable && url.pathname.endsWith('/candles')) {
      interrupted++;
      incoming.socket.destroy();
      return;
    }
    const forward = request(url, { method: incoming.method, headers: { ...incoming.headers, host: upstream.host } }, reply => {
      if (url.pathname.endsWith('/candles') && reply.statusCode === 200) successful++;
      outgoing.writeHead(reply.statusCode ?? 502, reply.headers);
      reply.pipe(outgoing);
    });
    forward.on('error', () => outgoing.destroy());
    outgoing.on('close', () => forward.destroy());
    incoming.pipe(forward);
  });
  server.listen(0, upstream.hostname);
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Chart history listener required');
  const proxy = new URL(actual.api);
  proxy.port = String(address.port);
  marketRetryPace.clear();
  vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
  const states: ChartFreshness[] = [];
  const feed = createReplayUdfDatafeed(actual.pool, 'price', value => states.push(value), undefined, {
    apiOrigin: proxy.origin, valuationOrigin: actual.charts,
  });
  try {
    feed.subscribeBars({}, '1D', () => {}, 'polling');
    expect(feed.replayStatus()[0].freshness).toMatchObject({
      connectionStatus: 'degraded', lagReason: 'Live updates are paused',
    });
    await vi.advanceTimersByTimeAsync(0);
    await eventually(() => feed.replayStatus()[0].freshness.connectionStatus === 'offline');
    expect(interrupted).toBeGreaterThan(0);
    unavailable = false;
    await eventually(() => feed.resourceCounts().requests === 0);
    for (let count = 0; count < 6 && feed.replayStatus()[0].freshness.connectionStatus === 'offline'; count++) {
      await vi.advanceTimersByTimeAsync(5000);
      await eventually(() => feed.resourceCounts().requests === 0);
    }
    expect(successful).toBeGreaterThan(0);
    expect(feed.replayStatus()[0].freshness).toMatchObject({
      connectionStatus: 'degraded', lagReason: 'Live updates are paused',
    });
    expect(feed.replayStatus()[0].freshness.sourceObservedAt).not.toBeNull();
    expect(states.some(value => value.connectionStatus === 'live')).toBe(false);
    expect(feed.resourceCounts().timers).toBe(1);
  } finally {
    feed.dispose();
    expect(feed.resourceCounts().timers).toBe(0);
    marketRetryPace.clear();
    vi.useRealTimers();
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
}
test(successfulPollRestoresPausedUpdates, successfulPollRestoresPausedUpdates, 120000);
