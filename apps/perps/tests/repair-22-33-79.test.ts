import assert from "node:assert/strict";
import { EventEmitter, once } from "node:events";
import { createServer, type Server } from "node:https";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import type {
  Bar,
  LibrarySymbolInfo,
  ResolutionString,
} from "../public/static/charting_library/charting_library";
import type { PerpsDatafeed } from "../src/core/datafeed/perps";

// Public, test-only TLS identity for the loopback candle producer.
const fixtureKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgfXyO12I6hTrYLEUe
dUMD03lCVxHscy3PulVVSdEUxeShRANCAARANoFgwqy9Y1StNwE59nCpE5t84WRs
zASWK5x/h3b1aEex1vVYiioViKcqZz91aHzFPxDBU8YSC/ZBQNO2PCJX
-----END PRIVATE KEY-----`;
const fixtureCertificate = `-----BEGIN CERTIFICATE-----
MIIBnDCCAUGgAwIBAgIUS0SSc0ZdRVgGTDjo1zfu7JHwppMwCgYIKoZIzj0EAwIw
FDESMBAGA1UEAwwJbG9jYWxob3N0MCAXDTI2MTEwNDA0MTMxN1oYDzIxMjYxMDEx
MDQxMzE3WjAUMRIwEAYDVQQDDAlsb2NhbGhvc3QwWTATBgcqhkjOPQIBBggqhkjO
PQMBBwNCAARANoFgwqy9Y1StNwE59nCpE5t84WRszASWK5x/h3b1aEex1vVYiioV
iKcqZz91aHzFPxDBU8YSC/ZBQNO2PCJXo28wbTAdBgNVHQ4EFgQUmvneHEwLwK1Q
b2nuWVLN8nUob5EwHwYDVR0jBBgwFoAUmvneHEwLwK1Qb2nuWVLN8nUob5EwDwYD
VR0TAQH/BAUwAwEB/zAaBgNVHREEEzARhwR/AAABgglsb2NhbGhvc3QwCgYIKoZI
zj0EAwIDSQAwRgIhAP3AB6h7m32qCO+Pzp5YYLz4Yfz7jBtZ68P1KNlop5r8AiEA
wTF2Udc2AXX/RRM21zc34fpeD3ycEdppuwwKVpYnatc=
-----END CERTIFICATE-----`;

interface FixtureSocket extends EventEmitter {
  send(data: string): void;
  close(code: number, reason: string): void;
}
interface FixtureSocketServer extends EventEmitter {
  clients: Set<FixtureSocket>;
  close(callback: () => void): void;
}

const require = createRequire(import.meta.url);
const { WebSocketServer } = require("next/dist/compiled/ws") as {
  WebSocketServer: new (options: { server: Server }) => FixtureSocketServer;
};
const tls = require("node:tls") as {
  getCACertificates(): string[];
  setDefaultCACertificates(certificates: string[]): void;
};

const hour = 1735689600;
type Candle = [number, number, number, number, number, number, number];
const transcript: Record<string, Candle> = {
  minuteHistory: [hour + 120, 100, 104, 99, 103, 10, 0],
  minuteSnapshot: [hour + 60, 100, 102, 99, 101, 5, 1],
  hourlySnapshot: [hour, 90, 105, 85, 100, 1000, 0],
  minuteLive: [hour + 240, 103, 108, 102, 107, 15, 0],
  minuteStale: [hour + 180, 103, 106, 102, 105, 12, 1],
  minuteSameTime: [hour + 240, 103, 110, 102, 109, 20, 0],
  minuteBarrier: [hour + 300, 109, 112, 108, 111, 21, 0],
  backfill: [hour - 3600, 80, 90, 75, 85, 200, 1],
  hourlyBackfill: [hour - 7200, 70, 80, 65, 75, 300, 1],
  monthly: [hour - 86400, 50, 110, 45, 100, 10000, 0],
  ethSnapshot: [hour + 600, 200, 205, 199, 204, 50, 0],
  ethStale: [hour + 540, 199, 204, 198, 203, 49, 1],
  ethBarrier: [hour + 660, 204, 208, 203, 207, 51, 0],
};

function udf(candle: Candle) {
  const [time, open, high, low, close, volume] = candle;
  return { s: "ok", t: [time], o: [open], h: [high], l: [low], c: [close], v: [volume] };
}

function chartBar(candle: Candle): Bar {
  const [time, open, high, low, close, volume] = candle;
  return { time: time * 1000, open, high, low, close, volume };
}

async function until(predicate: () => boolean, message: string) {
  const deadline = Date.now() + 5000;
  while (!predicate() && Date.now() < deadline) await delay(5);
  assert.ok(predicate(), message);
}

function history(feed: PerpsDatafeed, symbol: LibrarySymbolInfo, resolution: string, from = hour) {
  return new Promise<Bar[]>((resolve, reject) => {
    feed.getBars(symbol, resolution as ResolutionString, {
      from, to: hour + 7200, countBack: 10, firstDataRequest: from === hour,
    }, (bars) => resolve(bars), (error) => reject(new Error(error)));
  });
}

// Task 22.33.79: chart-resolution-cache is proven by this real datafeed path.
// Baseline verification is not run: the bounded gate order requires full implementation first.
// No out-of-path wiring is required.
async function chartResolutionCachePreservesSeriesWatermarks() {
  const requests: URL[] = [];
  const server = createServer({ key: fixtureKey, cert: fixtureCertificate }, (request, response) => {
    const url = new URL(request.url!, "https://localhost");
    requests.push(url);
    response.setHeader("content-type", "application/json");
    if (url.pathname.endsWith("/symbols")) {
      response.end(JSON.stringify({ name: "BTC", ticker: "BTC" }));
      return;
    }
    if (!url.pathname.endsWith("/history")) {
      response.writeHead(404).end();
      return;
    }
    const resolution = url.searchParams.get("resolution");
    const backfill = Number(url.searchParams.get("from")) < hour;
    const candle = resolution === "1M" ? transcript.monthly
      : url.searchParams.get("symbol") === "ETH" ? transcript.ethSnapshot
      : backfill ? (resolution === "60" ? transcript.hourlyBackfill : transcript.backfill)
      : transcript.minuteHistory;
    response.end(JSON.stringify(udf(candle)));
  });
  const sockets = new WebSocketServer({ server });
  const connections = new Map<string, FixtureSocket[]>();
  const subscriptions: string[] = [];
  sockets.on("connection", (socket: FixtureSocket, request: { url: string }) => {
    const symbol = request.url.split("/").pop()!.toUpperCase();
    const previous = connections.get(symbol) ?? [];
    previous.push(socket);
    connections.set(symbol, previous);
    const reconnected = previous.length > 1;
    socket.on("message", (raw: Buffer) => {
      const frame = JSON.parse(raw.toString()) as { event: string; topic?: string };
      if (frame.event !== "subscribe" || !frame.topic?.includes("@kline_")) return;
      subscriptions.push(frame.topic);
      const candle = symbol === "ETH" ? transcript.ethSnapshot
        : frame.topic.endsWith("1h") ? (reconnected ? transcript.backfill : transcript.hourlySnapshot)
        : transcript.minuteSnapshot;
      socket.send(JSON.stringify({ topic: frame.topic, snapshot: true, data: { candles: [candle] } }));
    });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const origin = `https://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const envKeys = [
    "NEXT_PUBLIC_PERPS_GATEWAY_URL", "NEXT_PUBLIC_PAXSCAN_BASE_URL",
    "NEXT_PUBLIC_PAXSCAN_API_URL", "NEXT_PUBLIC_PAXSCAN_WS_URL", "NEXT_PUBLIC_MARKET_PROXY_HOST",
  ];
  const previousEnv = envKeys.map((key) => process.env[key]);
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("https:", "wss:");
  process.env.NEXT_PUBLIC_MARKET_PROXY_HOST = new URL(origin).host;
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  const previousDocument = Object.getOwnPropertyDescriptor(globalThis, "document");
  Object.defineProperty(globalThis, "window", { configurable: true, value: new globalThis.EventTarget() });
  Object.defineProperty(globalThis, "document", { configurable: true, value: new globalThis.EventTarget() });
  const certificates = tls.getCACertificates();
  tls.setDefaultCACertificates([...certificates, fixtureCertificate]);
  let feed: PerpsDatafeed | undefined;
  const ids: string[] = [];
  try {
    const { PerpsDatafeed: Datafeed } = await import("../src/core/datafeed/perps");
    feed = new Datafeed();
    const symbol = await new Promise<LibrarySymbolInfo>((resolve, reject) => {
      feed!.resolveSymbol("PERP_btc_USDC", resolve, (error) => reject(new Error(error)));
    });
    const alias = { ...symbol, ticker: "btc/USD" };
    assert.deepEqual(await history(feed, alias, "1"), [chartBar(transcript.minuteHistory)]);
    const minute: Bar[] = [], hourly: Bar[] = [], aliased: Bar[] = [], monthly: Bar[] = [], eth: Bar[] = [];
    const resets: Record<string, number> = {};
    function subscribe(info: LibrarySymbolInfo, resolution: string, id: string, bars: Bar[]) {
      ids.push(id);
      resets[id] = 0;
      feed!.subscribeBars(info, resolution as ResolutionString, (bar) => bars.push(bar), id, () => { resets[id]++; });
    }
    subscribe(symbol, "1", "minute", minute);
    subscribe(symbol, "60", "hourly", hourly);
    subscribe(alias, "1", "alias", aliased);
    await until(() => subscriptions.includes("BTC@kline_1m") && subscriptions.includes("BTC@kline_1h"), "both resolutions subscribe on the real socket");
    function send(symbolName: string, interval: string, candle: Candle) {
      const socket = connections.get(symbolName)!.at(-1)!;
      socket.send(JSON.stringify({ topic: `${symbolName}@kline_${interval}`, data: { candle } }));
    }
    send("BTC", "1m", transcript.minuteHistory);
    await until(() => minute.at(-1)?.time === (hour + 120) * 1000 && aliased.at(-1)?.time === (hour + 120) * 1000, "minute update reaches both symbol aliases");
    assert.deepEqual(minute, [chartBar(transcript.minuteHistory)], "history watermark rejects the older snapshot through a normalized alias");
    assert.deepEqual(aliased, minute);
    assert.deepEqual(hourly, [chartBar(transcript.hourlySnapshot)], "the minute series cannot suppress the hourly snapshot");

    send("BTC", "1m", transcript.minuteLive);
    await until(() => minute.at(-1)?.time === (hour + 240) * 1000, "live minute advances the watermark");
    assert.deepEqual(await history(feed, symbol, "1", hour - 7200), [chartBar(transcript.backfill)]);
    assert.deepEqual(await history(feed, alias, "60", hour - 10800), [chartBar(transcript.hourlyBackfill)]);
    send("BTC", "1m", transcript.minuteStale);
    send("BTC", "1h", transcript.backfill);
    send("BTC", "1m", transcript.minuteSameTime);
    send("BTC", "1m", transcript.minuteBarrier);
    await until(() => minute.at(-1)?.time === (hour + 300) * 1000 && aliased.at(-1)?.time === (hour + 300) * 1000, "ordered live barrier follows stale frames");
    assert.deepEqual(minute, [transcript.minuteHistory, transcript.minuteLive, transcript.minuteSameTime, transcript.minuteBarrier].map(chartBar), "backfill cannot lower the watermark, while equal-time live OHLC updates remain valid");
    assert.deepEqual(aliased, minute);
    assert.deepEqual(hourly, [chartBar(transcript.hourlySnapshot)], "hourly backfill also preserves its live watermark");

    subscribe(symbol, "1M", "monthly", monthly);
    await until(() => monthly.length > 0, "the real REST polling fallback delivers a separate series");
    assert.deepEqual(monthly, [chartBar(transcript.monthly)]);
    feed.unsubscribeBars("monthly");
    const ethSymbol = { ...symbol, ticker: "ETH" };
    assert.deepEqual(await history(feed, ethSymbol, "1"), [chartBar(transcript.ethSnapshot)]);
    subscribe(ethSymbol, "1", "eth", eth);
    await until(() => eth.length === 1, "unrelated symbol starts its own stream");

    connections.get("BTC")!.at(-1)!.close(1012, "fixture-resync");
    await until(() => minute.at(-1)?.time === (hour + 60) * 1000 && hourly.at(-1)?.time === (hour - 3600) * 1000 && aliased.at(-1)?.time === (hour + 60) * 1000, "reconnect clears the matching series and accepts authoritative snapshots");
    assert.deepEqual(resets, { minute: 1, hourly: 1, alias: 1, monthly: 0, eth: 0 });
    assert.deepEqual(minute.at(-1), chartBar(transcript.minuteSnapshot));
    assert.deepEqual(hourly.at(-1), chartBar(transcript.backfill));
    send("ETH", "1m", transcript.ethStale);
    send("ETH", "1m", transcript.ethBarrier);
    await until(() => eth.at(-1)?.time === (hour + 660) * 1000, "unrelated stream processes the stale frame and barrier");
    assert.deepEqual(eth, [transcript.ethSnapshot, transcript.ethBarrier].map(chartBar), "BTC resync cannot clear ETH's watermark");
    assert.ok(requests.every((url) => !url.searchParams.has("symbol") || ["BTC", "ETH"].includes(url.searchParams.get("symbol")!)), "the real URL producer normalizes every transport request");
  } finally {
    for (const id of ids) feed?.unsubscribeBars(id);
    // Production streams release their idle sockets after the normal 15-second linger.
    while (sockets.clients.size > 0) await delay(20);
    await new Promise<void>((resolve) => sockets.close(resolve));
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    tls.setDefaultCACertificates(certificates);
    if (previousWindow) Object.defineProperty(globalThis, "window", previousWindow);
    else Reflect.deleteProperty(globalThis, "window");
    if (previousDocument) Object.defineProperty(globalThis, "document", previousDocument);
    else Reflect.deleteProperty(globalThis, "document");
    envKeys.forEach((key, index) => {
      if (previousEnv[index] === undefined) delete process.env[key];
      else process.env[key] = previousEnv[index];
    });
  }
}

test(chartResolutionCachePreservesSeriesWatermarks);
