import assert from "node:assert/strict";
import { once } from "node:events";
import test, { type TestContext } from "node:test";
import { setImmediate } from "node:timers/promises";
import { Window } from "happy-dom";
import WebSocket, { WebSocketServer } from "ws";
import type {
  Bar,
  LibrarySymbolInfo,
  ResolutionString,
} from "../packages/paxeer-backend/src/types/tradingview";

async function symbolSocketMarketsPollCandleHistoryForLiveBars(t: TestContext) {
  const server = new WebSocketServer({ port: 0, host: "localhost" });
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  const origin = `http://localhost:${address.port}`;
  const browser = new Window({ url: origin });
  const globals = ["window", "document", "WebSocket"] as const;
  const descriptors = globals.map((name) =>
    Object.getOwnPropertyDescriptor(globalThis, name),
  );
  const gateway = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  const disabled = process.env.NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  delete process.env.NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER;
  const clients: WebSocket[] = [];
  class MarketSocket extends WebSocket {
    constructor(url: string) {
      super(url);
      clients.push(this);
    }
  }
  Object.defineProperties(globalThis, {
    window: { configurable: true, value: browser },
    document: { configurable: true, value: browser.document },
    WebSocket: { configurable: true, value: MarketSocket },
  });
  let stopTrades: (() => void) | undefined;
  let stopBars: (() => void) | undefined;
  let clockEnabled = false;
  t.after(async () => {
    stopBars?.();
    stopTrades?.();
    if (clockEnabled) t.mock.timers.tick(15_000);
    const closed = clients.map((client) =>
      client.readyState === WebSocket.CLOSED
        ? Promise.resolve()
        : once(client, "close"),
    );
    for (const client of server.clients) client.terminate();
    await Promise.all(closed);
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
    await setImmediate();
    t.mock.timers.reset();
    await browser.happyDOM.close();
    globals.forEach((name, index) => {
      const descriptor = descriptors[index];
      if (descriptor) Object.defineProperty(globalThis, name, descriptor);
      else Reflect.deleteProperty(globalThis, name);
    });
    if (gateway === undefined) delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
    else process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = gateway;
    if (disabled === undefined)
      delete process.env.NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER;
    else process.env.NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER = disabled;
  });

  const frames: { event: string; topic?: string }[] = [];
  let socket: WebSocket;
  const subscribed = new Promise<void>((resolve) => {
    server.on("connection", (peer, request) => {
      assert.equal(request.url, "/ws/eth");
      socket = peer;
      peer.on("message", (data) => {
        frames.push(JSON.parse(data.toString()));
        if (frames.length === 3) resolve();
      });
    });
  });
  const { PerpsDatafeed } =
    await import("../packages/paxeer-backend/src/core/datafeed/perps");
  const { hasDedicatedMarketStream, subscribeNativeTrades } =
    await import("../packages/paxeer-backend/src/hooks/perps/useSidPaxMarketStream");
  assert.equal(hasDedicatedMarketStream("ETH"), true);
  const start = 1_800_000_000_000;
  t.mock.timers.enable({
    apis: ["Date", "setTimeout", "setInterval"],
    now: start,
  });
  clockEnabled = true;
  stopTrades = subscribeNativeTrades("ETH", () => {});
  await subscribed;
  const expectedTopics = [
    { event: "subscribe", topic: "ETH@perp_orderbook" },
    { event: "subscribe", topic: "ETH@perp_trade" },
    { event: "subscribe", topic: "ETH@perp_stats" },
  ];
  assert.deepEqual(frames, expectedTopics);

  const resolution = "5" as ResolutionString;
  const historyBar: Bar = {
    time: start - 60_000,
    open: 100,
    high: 110,
    low: 90,
    close: 105,
    volume: 12,
  };
  const liveBars: Bar[][] = [
    [{ ...historyBar, time: start - 120_000 }],
    [historyBar, { ...historyBar, time: start, close: 106 }],
    [{ ...historyBar, time: start, close: 108, volume: 14 }],
    [{ ...historyBar, time: start - 60_000, close: 99 }],
  ];
  const requests: { url: URL; time: number }[] = [];
  let seedHistory = true;
  t.mock.method(globalThis, "fetch", async (input: string | URL | Request) => {
    const url = new URL(
      input instanceof Request ? input.url : String(input),
      origin,
    );
    assert.equal(url.pathname.startsWith("/api/price-feed/eth/"), true);
    assert.equal(url.searchParams.get("symbol"), "ETH");
    if (url.pathname.endsWith("/symbols"))
      return Response.json({ name: "ETH", ticker: "ETH" });
    assert.equal(url.pathname, "/api/price-feed/eth/history");
    let bars: Bar[];
    if (seedHistory) {
      seedHistory = false;
      bars = [historyBar];
    } else {
      requests.push({ url, time: Date.now() });
      const next = liveBars.shift();
      assert.ok(next, "Each poll consumes one candle response");
      bars = next;
    }
    return Response.json({
      s: "ok",
      t: bars.map((bar) => bar.time / 1000),
      o: bars.map((bar) => bar.open),
      h: bars.map((bar) => bar.high),
      l: bars.map((bar) => bar.low),
      c: bars.map((bar) => bar.close),
      v: bars.map((bar) => bar.volume),
    });
  });
  const feed = new PerpsDatafeed();
  const symbol = await new Promise<LibrarySymbolInfo>((resolve, reject) =>
    feed.resolveSymbol("ETH", resolve, reject),
  );
  const historical = await new Promise<Bar[]>((resolve, reject) =>
    feed.getBars(
      { ...symbol, ticker: "PERP_ETH_USDC" },
      resolution,
      {
        from: start / 1000 - 300,
        to: start / 1000,
        countBack: 1,
        firstDataRequest: true,
      },
      resolve,
      reject,
    ),
  );
  assert.deepEqual(historical, [historyBar]);
  const ticks: Bar[] = [];
  let resets = 0;
  feed.subscribeBars(
    symbol,
    resolution,
    (bar) => ticks.push(bar),
    "live",
    () => resets++,
  );
  stopBars = () => feed.unsubscribeBars("live");

  async function advance(ms: number) {
    t.mock.timers.tick(ms);
    await setImmediate();
  }
  await advance(250);
  assert.equal(requests.length, 1);
  assert.deepEqual(ticks, []);
  await advance(1999);
  assert.equal(requests.length, 1);
  await advance(1);
  await advance(200);
  assert.equal(requests.length, 2);
  assert.deepEqual(ticks, [{ ...historyBar, time: start, close: 106 }]);
  await advance(2000);
  await advance(200);
  assert.equal(requests.length, 3);
  assert.deepEqual(ticks.at(-1), {
    ...historyBar,
    time: start,
    close: 108,
    volume: 14,
  });
  await advance(2000);
  await advance(200);
  assert.equal(requests.length, 4);
  assert.equal(ticks.length, 2);
  assert.equal(liveBars.length, 0);
  assert.equal(resets, 0);
  for (const { url, time } of requests) {
    const now = Math.floor(time / 1000);
    assert.deepEqual(Object.fromEntries(url.searchParams), {
      symbol: "ETH",
      resolution: "5",
      from: String(now - 300),
      to: String(now + 1),
      countback: "1",
    });
  }

  feed.unsubscribeBars("live");
  await advance(10_000);
  assert.equal(requests.length, 4);
  assert.equal(ticks.length, 2);
  const pong = once(socket!, "pong");
  socket!.ping();
  await pong;
  assert.deepEqual(frames, expectedTopics);
}

test(symbolSocketMarketsPollCandleHistoryForLiveBars);
