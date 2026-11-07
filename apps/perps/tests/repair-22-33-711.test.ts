import assert from "node:assert/strict";
import { createHook } from "node:async_hooks";
import { EventEmitter, once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer, type Server, type ServerResponse } from "node:http";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import { after, before, test, type TestContext } from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { act, createElement, type ReactNode } from "react";
import type { Root } from "react-dom/client";
import type { QueryClient } from "@tanstack/react-query";
import type { Config } from "wagmi";
import { decodeFunctionData, encodeFunctionResult, type Hex } from "viem";

const A = "0x00000000000000000000000000000000000000ab";
const B = "0x00000000000000000000000000000000000000cd";
const WAD = 10n ** 18n;
const require = createRequire(import.meta.url);

// In-memory peer tables are served through real HTTP and WebSocket servers.
// The repaired hooks, their providers, clients and codecs are never replaced.
const traders = new Map(
  [A, B].map((address, index) => [
    address,
    {
      profile: {
        address,
        vaultAddress: address,
        referralCode: index ? "BETA" : "ALPHA",
        tier: "bronze",
        feeDiscountBps: index + 1,
      },
      stats: {
        address,
        totalPnl: String(index + 100),
        totalVolume: "200",
        totalFeesPaid: "3",
        totalTrades: index + 1,
        winCount: 1,
        lossCount: 0,
        liquidationCount: 0,
        openPositions: 1,
      },
    },
  ]),
);
function position(positionId: string, userAddress = A) {
  return {
    positionId,
    userAddress,
    marketId: 0,
    isLong: true,
    sizeUsd: String(100n * WAD),
    leverage: String(2n * WAD),
    entryPrice: String(50_000n * WAD),
    collateralToken: null,
    collateralAmount: "50000000",
    collateralUsd: String(50n * WAD),
    status: "open",
    openedAt: null,
    openTxHash: null,
    market: { symbol: "BTC" },
  };
}
const positions = new Map<string, ReturnType<typeof position>[]>([
  [A, []],
  [B, []],
]);
const marketRows = [
  {
    marketId: 0,
    symbol: "BTC",
    name: "Bitcoin",
    oracleSymbol: "BTC",
    szDecimals: 5,
    pxDecimals: 1,
    tickSize: "100000000000000000",
    lotSize: "10000000000000",
    tierClass: 0,
    maxLeverage: "1000",
    enabled: true,
    createdAt: null,
    latestPrice: null,
    fundingRate: null,
  },
  {
    marketId: 3,
    symbol: "SOL",
    name: "Solana",
    oracleSymbol: "SOL",
    szDecimals: null,
    pxDecimals: null,
    tickSize: null,
    lotSize: null,
    tierClass: null,
    maxLeverage: null,
    enabled: false,
    createdAt: null,
    latestPrice: null,
    fundingRate: null,
  },
];
interface GraphqlRequest {
  query: string;
  variables?: { address?: string; userAddress?: string };
}
class PendingQuery {
  constructor(
    readonly body: GraphqlRequest,
    readonly data: unknown,
    private readonly response: ServerResponse,
  ) {}
  release(failure = false, data: unknown = this.data) {
    this.response
      .writeHead(200, { "content-type": "application/json" })
      .end(
        JSON.stringify(
          failure
            ? { errors: [{ message: "fixture query refused" }] }
            : { data },
        ),
      );
  }
}
class PendingOi {
  constructor(
    private readonly response: ServerResponse,
    readonly long: number,
    readonly short: number,
  ) {}
  release(success = true) {
    this.response.writeHead(200, { "content-type": "application/json" }).end(
      JSON.stringify(
        success
          ? {
              success: true,
              data: [
                {
                  paxeerSymbol: "BTC",
                  long_oi: this.long,
                  short_oi: this.short,
                },
              ],
            }
          : { success: false },
      ),
    );
  }
}
interface PeerSocket extends EventEmitter {
  readyState: number;
  send(data: string): void;
  close(code?: number): void;
  terminate(): void;
}
interface SocketServer extends EventEmitter {
  clients: Set<PeerSocket>;
  close(callback: () => void): void;
}
const { WebSocketServer } = require("next/dist/compiled/ws") as {
  WebSocketServer: new (options: { server: Server }) => SocketServer;
};
class StreamPeer {
  readonly subscriptions = new Map<string, GraphqlRequest>();
  readonly topics = new Set<string>();
  framesSent = 0;
  constructor(
    readonly socket: PeerSocket,
    readonly path: string,
  ) {
    socket.on("message", (raw: Buffer) => {
      const frame = JSON.parse(raw.toString());
      if (path === "/graphql") {
        if (frame.type === "connection_init")
          this.send({ type: "connection_ack" });
        else if (frame.type === "subscribe")
          this.subscriptions.set(frame.id, frame.payload);
        else if (frame.type === "complete") this.subscriptions.delete(frame.id);
        else if (frame.type === "ping") this.send({ type: "pong" });
      } else if (frame.event === "subscribe") this.topics.add(frame.topic);
    });
  }
  send(frame: unknown) {
    this.framesSent++;
    this.socket.send(JSON.stringify(frame));
  }
  publishPositions(address: string) {
    for (const [id, subscription] of this.subscriptions) {
      if (subscription.variables?.userAddress === address) {
        this.send({
          id,
          type: "next",
          payload: {
            data: {
              positionChanged: structuredClone(positions.get(address) ?? []),
            },
          },
        });
      }
    }
  }
  publishMarkets(rows: unknown[]) {
    this.send({ topic: "markets@all", data: rows, ts: Date.now() });
  }
}
const pending: PendingQuery[] = [];
const queryLog: GraphqlRequest[] = [];
const peers: StreamPeer[] = [];
const peerErrors: unknown[] = [];
let abi: (typeof import("../src/core/network/abi/PerpsPrecompile"))["PerpsPrecompileABI"];
let nodeOutput: (typeof import("./node-leverage-output"))["nodeLeverageOutput"];

function rpcResult(request: { method: string; params?: unknown[] }) {
  if (request.method === "eth_chainId") return "0x7d";
  if (request.method === "eth_blockNumber") return "0x2885868";
  if (request.method === "net_listening") return true;
  if (request.method === "eth_call") {
    const call = request.params![0] as { data: Hex; to: string };
    assert.equal(
      call.to.toLowerCase(),
      "0x0000000000000000000000000000000000001015",
    );
    const decoded = decodeFunctionData({ abi, data: call.data });
    if (
      decoded.functionName === "getMarketConfig" ||
      decoded.functionName === "getMarginTiers"
    ) {
      const transcript = nodeOutput.calls[decoded.functionName];
      const recordedCall = transcript.request.params[0];
      assert.ok(
        recordedCall &&
          typeof recordedCall === "object" &&
          "data" in recordedCall,
      );
      assert.equal(call.data, recordedCall.data);
      return transcript.response.result;
    }
    if (decoded.functionName === "getAccount") {
      return encodeFunctionResult({
        abi,
        functionName: "getAccount",
        result: {
          collateral: 100000000n,
          equityUsd: 100n * WAD,
          initialMarginUsd: 10n * WAD,
          withdrawable: 90000000n,
          maintenanceMarginUsd: WAD,
          feeTier: 0,
          volume14dUsd: 0n,
          positionCount: 0n,
          openOrderCount: 0n,
        },
      });
    }
    if (decoded.functionName === "getMarkPriceDetails") {
      return encodeFunctionResult({
        abi,
        functionName: "getMarkPriceDetails",
        result: [50_000n * WAD, 50_000n * WAD, 50_000n * WAD, true],
      });
    }
    throw new Error(`Unrecorded RPC call: ${decoded.functionName}`);
  }
  throw new Error(`Unrecorded RPC method: ${request.method}`);
}
class Venue {
  readonly oi: PendingOi[] = [];
  oiRequests = 0;
  tickerRequests = 0;
  readonly server: Server;
  readonly sockets: SocketServer;
  origin = "";
  constructor(
    readonly long: number,
    readonly short: number,
  ) {
    this.server = createServer(async (request, response) => {
      try {
        const path = request.url!;
        if (path === "/graphql") {
          let raw = "";
          for await (const chunk of request) raw += chunk;
          const body: GraphqlRequest = JSON.parse(raw);
          queryLog.push(body);
          const address =
            body.variables?.address ?? body.variables?.userAddress ?? "";
          let data: unknown;
          if (body.query.includes("userStats"))
            data = { userStats: traders.get(address)?.stats ?? null };
          else if (/\buser\(/.test(body.query))
            data = { user: traders.get(address)?.profile ?? null };
          else if (/\bpositions\(/.test(body.query))
            data = { positions: structuredClone(positions.get(address) ?? []) };
          else if (body.query.includes("markets"))
            data = { markets: structuredClone(marketRows) };
          else throw new Error(`Unrecorded GraphQL query: ${body.query}`);
          pending.push(new PendingQuery(body, data, response));
          return;
        }
        if (
          path === "/api/markets/open-interest" ||
          path === "/alternate/open-interest"
        ) {
          this.oiRequests++;
          this.oi.push(new PendingOi(response, this.long, this.short));
          return;
        }
        if (
          path === "/api/markets/ticker/BTC" ||
          path === "/alternate/ticker/BTC"
        ) {
          this.tickerRequests++;
          response.writeHead(200, { "content-type": "application/json" }).end(
            JSON.stringify({
              success: true,
              data: {
                index_price: 50_000,
                mark_price: 50_000,
                last_funding_rate_hourly: "0.0001",
              },
            }),
          );
          return;
        }
        if (path === "/" && request.method === "POST") {
          let raw = "";
          for await (const chunk of request) raw += chunk;
          const answer = (rpc: {
            id: number;
            method: string;
            params?: unknown[];
          }) => ({ jsonrpc: "2.0", id: rpc.id, result: rpcResult(rpc) });
          const body = JSON.parse(raw);
          response
            .writeHead(200, { "content-type": "application/json" })
            .end(
              JSON.stringify(
                Array.isArray(body) ? body.map(answer) : answer(body),
              ),
            );
          return;
        }
        throw new Error(`Unrecorded fixture route: ${request.method} ${path}`);
      } catch (error) {
        peerErrors.push(error);
        response.writeHead(500).end(JSON.stringify({ error: String(error) }));
      }
    });
    this.sockets = new WebSocketServer({ server: this.server });
    this.sockets.on(
      "connection",
      (socket: PeerSocket, request: { url: string }) =>
        peers.push(new StreamPeer(socket, request.url)),
    );
  }
  async start() {
    this.server.listen(0, "127.0.0.1");
    await once(this.server, "listening");
    this.origin = `http://127.0.0.1:${(this.server.address() as AddressInfo).port}`;
  }
  async close() {
    for (const request of this.oi.splice(0)) request.release();
    for (const socket of this.sockets.clients) socket.terminate();
    await new Promise<void>((resolve) => this.sockets.close(resolve));
    this.server.closeAllConnections();
    await new Promise<void>((resolve) => this.server.close(() => resolve()));
  }
}
const firstVenue = new Venue(11, 12);
const secondVenue = new Venue(21, 22);
let modules: {
  stats: typeof import("../src/hooks/perps/usePerfdbTraderStats");
  profile: typeof import("../src/hooks/perps/usePerfdbUser");
  markets: typeof import("../src/hooks/perps/usePerpsMarkets");
  portfolio: typeof import("../src/hooks/perps/usePortfolioStats");
  quoter: typeof import("../src/hooks/perps/useQuoter");
  prices: typeof import("../src/hooks/perps/useRealtimePrices");
};
let createRoot: (typeof import("react-dom/client"))["createRoot"];
let QueryClientProvider: (typeof import("@tanstack/react-query"))["QueryClientProvider"];
let WagmiProvider: (typeof import("wagmi"))["WagmiProvider"];
let wagmiConfig: Config;
let queries: QueryClient;
let closeDom: () => void;
const mounted = new Set<Root>();
const savedGlobals = new Map<string, PropertyDescriptor | undefined>();
const savedEnv = new Map<string, string | undefined>();
// Observe real timer handles without replacing the clock or scheduler. The
// RPC transport's recurring ranker has no stop API; after every assertion and
// fixture shutdown, its remaining handles must not keep this process alive.
const timers = new Map<number, NodeJS.Timeout>();
const timerObserver = createHook({
  init(id, type, _trigger, resource) {
    if (type === "Timeout") timers.set(id, resource as NodeJS.Timeout);
  },
  destroy(id) {
    timers.delete(id);
  },
});

before(async () => {
  timerObserver.enable();
  await Promise.all([firstVenue.start(), secondVenue.start()]);
  for (const [key, value] of Object.entries({
    NEXT_PUBLIC_PERPS_GATEWAY_URL: firstVenue.origin,
    NEXT_PUBLIC_PAXSCAN_BASE_URL: firstVenue.origin,
    NEXT_PUBLIC_PAXSCAN_API_URL: `${firstVenue.origin}/api`,
    NEXT_PUBLIC_PAXSCAN_WS_URL: firstVenue.origin.replace("http", "ws"),
    NEXT_PUBLIC_PAXEER_RPC_URLS: firstVenue.origin,
    NEXT_PUBLIC_MARKET_PROXY_HOST: firstVenue.origin,
    NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY: "",
    NEXT_PUBLIC_SUPABASE_URL: firstVenue.origin,
    NEXT_PUBLIC_PAXEER_WALLET_API: firstVenue.origin,
  })) {
    savedEnv.set(key, process.env[key]);
    process.env[key] = value;
  }
  const fixtureRequire = createRequire(
    new URL("../../../services/wallet/demo/package.json", import.meta.url),
  );
  const { JSDOM } = fixtureRequire("jsdom");
  const dom = new JSDOM("<!doctype html><html><body></body></html>", {
    url: firstVenue.origin,
    pretendToBeVisual: true,
  });
  closeDom = () => dom.window.close();
  // Wallet discovery constructs events globally and dispatches on window;
  // both constructors must belong to the actual JSDOM browser realm.
  for (const [key, value] of Object.entries({
    window: dom.window,
    document: dom.window.document,
    sessionStorage: dom.window.sessionStorage,
    Event: dom.window.Event,
    CustomEvent: dom.window.CustomEvent,
    WebSocket: dom.window.WebSocket,
    IS_REACT_ACT_ENVIRONMENT: true,
  })) {
    savedGlobals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { value, configurable: true });
  }
  ({ createRoot } = await import("react-dom/client"));
  const query =
    require("@tanstack/react-query") as typeof import("@tanstack/react-query");
  QueryClientProvider = query.QueryClientProvider;
  queries = new query.QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  ({ WagmiProvider } = require("wagmi") as typeof import("wagmi"));
  ({ wagmiConfig } = await import("../src/core/wagmi"));
  ({ PerpsPrecompileABI: abi } = await import(
    "../src/core/network/abi/PerpsPrecompile"
  ));
  ({ nodeLeverageOutput: nodeOutput } = await import("./node-leverage-output"));
  modules = {
    stats: await import("../src/hooks/perps/usePerfdbTraderStats"),
    profile: await import("../src/hooks/perps/usePerfdbUser"),
    markets: await import("../src/hooks/perps/usePerpsMarkets"),
    portfolio: await import("../src/hooks/perps/usePortfolioStats"),
    quoter: await import("../src/hooks/perps/useQuoter"),
    prices: await import("../src/hooks/perps/useRealtimePrices"),
  };
});
let fixtureCleanup: Promise<void> | undefined;
function closeFixtures() {
  return (fixtureCleanup ??= cleanupFixtures());
}
async function cleanupFixtures() {
  try {
    await act(async () => {
      for (const root of mounted) root.unmount();
      mounted.clear();
    });
    for (const request of pending.splice(0)) request.release();
    queries?.clear();
    await Promise.all([firstVenue.close(), secondVenue.close()]);
    // Let the real socket close callbacks remove their browser listeners before
    // restoring the host globals.
    await settle();
    closeDom?.();
    for (const [key, descriptor] of savedGlobals) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
    for (const [key, value] of savedEnv) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    assert.deepEqual(
      peerErrors,
      [],
      "every producer request uses the committed peer protocol",
    );
  } finally {
    timerObserver.disable();
    for (const timer of timers.values()) timer.unref();
    timers.clear();
  }
}
after(closeFixtures);
async function until(predicate: () => boolean, timeout = 5000) {
  const deadline = Date.now() + timeout;
  while (!predicate()) {
    assert.ok(
      Date.now() < deadline,
      "real loopback operation must settle within its deadline",
    );
    await act(async () => {
      await delay(5);
    });
  }
}
async function settle() {
  await act(async () => {
    await delay(50);
  });
}
async function take(operation: string, account?: string) {
  const matches = (request: PendingQuery) =>
    request.body.query.includes(operation) &&
    (!account ||
      (request.body.variables?.address ??
        request.body.variables?.userAddress) === account);
  await until(() => pending.some(matches));
  return pending.splice(pending.findIndex(matches), 1)[0]!;
}
async function release(
  request: PendingQuery,
  failure = false,
  data: unknown = request.data,
) {
  await act(async () => {
    request.release(failure, data);
    await delay(50);
  });
}
async function mount<P, T>(
  hook: (props: P) => T,
  initial: P,
  wrap: (node: ReactNode) => ReactNode = (node) => node,
) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  mounted.add(root);
  let value: T;
  function Probe({ props }: { props: P }) {
    value = hook(props);
    return createElement("span");
  }
  const render = async (props: P) => {
    await act(async () => root.render(wrap(createElement(Probe, { props }))));
  };
  await render(initial);
  return {
    read() {
      return value!;
    },
    render,
    async close() {
      await act(async () => root.unmount());
      mounted.delete(root);
      container.remove();
    },
  };
}
function withQueries(node: ReactNode) {
  return createElement(QueryClientProvider, { client: queries }, node);
}
function withWallet(node: ReactNode) {
  return withQueries(
    createElement(
      WagmiProvider,
      { config: wagmiConfig, reconnectOnMount: false },
      node,
    ),
  );
}
async function selectWallet(address: typeof A | typeof B) {
  const connector = wagmiConfig.connectors[0]!;
  await act(async () =>
    wagmiConfig.setState({
      chainId: 125,
      current: connector.uid,
      status: "connected",
      connections: new Map([
        [
          connector.uid,
          { accounts: [address] as const, chainId: 125, connector },
        ],
      ]),
    }),
  );
}

interface AccountView {
  value: { address: string } | null;
  isLoading: boolean;
  error: Error | null;
}
async function accountRace(
  operation: string,
  hook: (address: string | undefined) => AccountView,
) {
  for (const failure of [false, true]) {
    for (const finishBFirst of [false, true]) {
      const mountedHook = await mount(hook, A as string | undefined);
      const old = await take(operation, A);
      await mountedHook.render(B);
      assert.deepEqual(mountedHook.read(), {
        value: null,
        isLoading: true,
        error: null,
      });
      const current = await take(operation, B);
      if (finishBFirst) {
        await release(current);
        await until(() => !mountedHook.read().isLoading);
      }
      await release(old, failure);
      assert.equal(mountedHook.read().error, null);
      if (!finishBFirst) {
        assert.equal(mountedHook.read().value, null);
        assert.equal(mountedHook.read().isLoading, true);
        await release(current);
        await until(() => !mountedHook.read().isLoading);
      }
      assert.equal(mountedHook.read().value?.address, B);
      assert.equal(mountedHook.read().isLoading, false);
      await mountedHook.render(A);
      const disconnected = await take(operation, A);
      assert.equal(
        mountedHook.read().value,
        null,
        "completed prior-account values clear at replacement",
      );
      await mountedHook.render(undefined);
      await release(disconnected, failure);
      assert.deepEqual(mountedHook.read(), {
        value: null,
        isLoading: false,
        error: null,
      });
      await mountedHook.close();
    }
    const reopened = await mount(hook, A as string | undefined);
    const priorGeneration = await take(operation, A);
    await reopened.render(undefined);
    await reopened.render(A);
    const nextGeneration = await take(operation, A);
    await release(priorGeneration, failure);
    assert.deepEqual(reopened.read(), {
      value: null,
      isLoading: true,
      error: null,
    });
    await release(nextGeneration);
    await until(() => !reopened.read().isLoading);
    assert.equal(reopened.read().value?.address, A);
    const count = queryLog.length;
    await reopened.render(`  ${A.toUpperCase()}  `);
    await settle();
    assert.equal(
      queryLog.length,
      count,
      "normalized spelling retains the same request generation",
    );
    assert.equal(reopened.read().value?.address, A);
    await reopened.render(B);
    await release(await take(operation, B), true);
    await until(() => !reopened.read().isLoading);
    assert.match(reopened.read().error!.message, /fixture query refused/);
    await reopened.render(A);
    assert.deepEqual(reopened.read(), {
      value: null,
      isLoading: true,
      error: null,
    });
    await release(await take(operation, A));
    await reopened.close();
  }
}
async function statsAccountRace() {
  await accountRace("UserStats", (address) => {
    const { stats, ...state } = modules.stats.usePerfdbTraderStats(address);
    return { ...state, value: stats };
  });
}
test(statsAccountRace);
async function profileAccountRace() {
  await accountRace("query User(", (address) => {
    const { user, ...state } = modules.profile.usePerfdbUser(address);
    return { ...state, value: user };
  });
}
test(profileAccountRace);

async function registryMarketCatalog() {
  const doc = readFileSync(
    new URL("../../../contracts/perps-client.md", import.meta.url),
    "utf8",
  );
  assert.ok(doc.includes("usePerpsMarkets()"));
  for (const field of ["catalog", "tradeable", "isLoading", "error", "refetch"])
    assert.ok(doc.includes(field));
  assert.match(
    doc,
    /no arguments and returns `\{ catalog, tradeable, isLoading, error, refetch \}`/,
  );
  assert.match(
    doc,
    /`tradeable` contains only enabled\s+entries, each with non-null numeric `maxLeverage`/,
  );
  assert.match(
    doc,
    /Network, GraphQL and decoder errors expose `error` and empty arrays/,
  );
  const options = modules.markets.perpsMarketsQueryOptions();
  assert.deepEqual(options.queryKey, ["perps", "markets"]);
  assert.equal(options.retry, false);
  const hook = await mount(
    () => modules.markets.usePerpsMarkets(),
    undefined,
    withQueries,
  );
  assert.deepEqual(hook.read().catalog, []);
  assert.deepEqual(hook.read().tradeable, []);
  assert.equal(hook.read().isLoading, true);
  assert.equal(hook.read().error, null);
  assert.equal(typeof hook.read().refetch, "function");
  await release(await take("GetMarkets"));
  await until(() => !hook.read().isLoading);
  assert.deepEqual(
    hook.read().catalog.map((row) => row.marketId),
    [0, 3],
  );
  assert.deepEqual(
    hook.read().tradeable.map((row) => row.marketId),
    [0],
  );
  assert.equal(hook.read().catalog[1]!.maxLeverage, null);
  assert.equal(hook.read().tradeable[0]!.maxLeverage, 1000);
  assert.equal(hook.read().catalog[0]!.baseToken, "BTC");
  assert.equal(hook.read().catalog[0]!.tickSize, "100000000000000000");
  let refetch!: ReturnType<
    ReturnType<typeof modules.markets.usePerpsMarkets>["refetch"]
  >;
  await act(async () => {
    refetch = hook.read().refetch();
  });
  const failed = await take("GetMarkets");
  assert.equal(hook.read().isLoading, false);
  assert.equal(
    hook.read().catalog.length,
    2,
    "same-query refresh retains available rows while pending",
  );
  await release(failed, true);
  await act(async () => {
    await refetch;
  });
  await until(() => hook.read().error !== null);
  assert.deepEqual(hook.read().catalog, []);
  assert.deepEqual(hook.read().tradeable, []);
  assert.equal(hook.read().isLoading, false);
  await act(async () => {
    refetch = hook.read().refetch();
  });
  await release(await take("GetMarkets"), false, { markets: [] });
  await act(async () => {
    await refetch;
  });
  await until(() => hook.read().error === null);
  assert.deepEqual(hook.read().catalog, []);
  assert.deepEqual(hook.read().tradeable, []);
  await act(async () => {
    refetch = hook.read().refetch();
  });
  await release(await take("GetMarkets"), false, {
    markets: [{ marketId: 0 }],
  });
  await act(async () => {
    await refetch;
  });
  await until(() => hook.read().error !== null);
  assert.deepEqual(hook.read().catalog, []);
  await hook.close();
  queries.clear();
}
test(registryMarketCatalog);

async function positionPeer() {
  await until(() =>
    peers.some(
      (peer) =>
        peer.socket.readyState === 1 &&
        [...peer.subscriptions.values()].some((sub) =>
          sub.query.includes("PositionChanged"),
        ),
    ),
  );
  return peers.find(
    (peer) =>
      peer.socket.readyState === 1 &&
      [...peer.subscriptions.values()].some((sub) =>
        sub.query.includes("PositionChanged"),
      ),
  )!;
}
async function portfolioSnapshotLiveOrdering() {
  positions.set(A, [position("1")]);
  let hook = await mount(
    () => modules.portfolio.usePortfolioStats(),
    undefined,
    withWallet,
  );
  // The real provider clears hydrated connections on mount. Select only after
  // that lifecycle has settled, then await the subscription before publishing.
  await selectWallet(A);
  let peer = await positionPeer();
  const initial = await take("GetPositions", A);
  await act(async () => peer.publishPositions(A));
  await until(() => hook.read().positions.length === 1);
  positions.set(A, []);
  await act(async () => peer.publishPositions(A));
  await until(() => hook.read().positions.length === 0);
  await release(initial);
  await until(() => !hook.read().isLoading);
  assert.deepEqual(
    hook.read().positions,
    [],
    "a delayed initial snapshot cannot resurrect a closed position",
  );
  assert.equal(
    hook.read().totalVaultUsd,
    100,
    "the real ABI account read also completed",
  );

  positions.set(A, [position("2")]);
  await act(async () => peer.publishPositions(A));
  await until(() => hook.read().positions.length === 1);
  peer.socket.close(1012);
  await until(() =>
    peers.some(
      (next) =>
        next !== peer &&
        next.path === "/graphql" &&
        next.subscriptions.size > 0,
    ),
  );
  peer = await positionPeer();
  const recovery = await take("GetPositions", A);
  positions.set(A, []);
  await act(async () => peer.publishPositions(A));
  await until(() => hook.read().positions.length === 0);
  await release(recovery);
  assert.deepEqual(
    hook.read().positions,
    [],
    "a delayed reconnect snapshot cannot overwrite the live close",
  );

  // Stop and reopen the actual singleton consumer with its durable peer store.
  positions.set(A, [position("3")]);
  await act(async () => modules.portfolio.refreshPositions());
  const stoppedSession = await take("GetPositions", A);
  await hook.close();
  positions.set(A, []);
  hook = await mount(
    () => modules.portfolio.usePortfolioStats(),
    undefined,
    withWallet,
  );
  await selectWallet(A);
  await positionPeer();
  const reopened = await take("GetPositions", A);
  await release(reopened);
  await until(() => !hook.read().isLoading);
  await release(stoppedSession);
  assert.deepEqual(
    hook.read().positions,
    [],
    "same-account reopen invalidates the old session's pending read",
  );
  await settle();
  assert.deepEqual(hook.read().positions, []);
  await hook.close();
}
test(portfolioSnapshotLiveOrdering);

async function quoterCacheOrigin() {
  const first = await mount(
    (url) => modules.quoter.useQuoter(url),
    `${firstVenue.origin}/api/markets/`,
  );
  const equivalent = await mount(
    (url) => modules.quoter.useQuoter(url),
    `${firstVenue.origin}/api/markets`,
  );
  const second = await mount(
    (url) => modules.quoter.useQuoter(url),
    `${secondVenue.origin}/api/markets`,
  );
  const quotes = [
    first.read().quoteMarket(0n),
    equivalent.read().quoteMarket(0n),
    second.read().quoteMarket(0n),
  ];
  await until(() => firstVenue.oi.length === 1 && secondVenue.oi.length === 1);
  assert.equal(
    firstVenue.oiRequests,
    1,
    "equivalent normalized URLs share one in-flight request",
  );
  firstVenue.oi.shift()!.release();
  secondVenue.oi.shift()!.release();
  const [a, alias, b] = await Promise.all(quotes);
  assert.equal(a!.longOI, 11n * WAD);
  assert.equal(a!.shortOI, 12n * WAD);
  assert.equal(alias!.longOI, a!.longOI);
  assert.equal(b!.longOI, 21n * WAD);
  assert.equal(b!.shortOI, 22n * WAD);
  assert.equal(a!.openInterest.origin, `${firstVenue.origin}/api/markets`);
  assert.equal(b!.openInterest.origin, `${secondVenue.origin}/api/markets`);
  assert.equal(a!.openInterest.source, "fresh");
  assert.ok(
    a!.openInterest.fetchedAt !== null &&
      a!.openInterest.ageMs !== null &&
      a!.openInterest.ageMs >= 0,
  );
  const cached = await second.read().quoteMarket(0n);
  assert.equal(cached.longOI, b!.longOI);
  assert.equal(cached.openInterest.source, "cache");
  assert.equal(secondVenue.oiRequests, 1);
  // Expire only this cache naturally, then exercise the explicit same-origin
  // stale fallback. No fake clock or replacement transport is involved.
  await delay(60_010);
  const staleRead = first.read().quoteMarket(0n);
  await until(() => firstVenue.oi.length === 1);
  firstVenue.oi.shift()!.release(false);
  const stale = await staleRead;
  assert.equal(stale.longOI, 11n * WAD);
  assert.equal(stale.openInterest.source, "stale");
  assert.equal(stale.openInterest.origin, `${firstVenue.origin}/api/markets`);
  assert.equal(stale.openInterest.fetchedAt, a!.openInterest.fetchedAt);
  assert.ok(
    stale.openInterest.ageMs! >= 60_000 && stale.openInterest.ageMs! <= 300_000,
  );
  assert.equal(
    secondVenue.oiRequests,
    1,
    "a failed first-origin refresh cannot touch the second origin",
  );
  const alternate = await mount(
    (url) => modules.quoter.useQuoter(url),
    `${firstVenue.origin}/alternate/`,
  );
  const unavailableRead = alternate.read().quoteMarket(0n);
  await until(() => firstVenue.oi.length === 1);
  firstVenue.oi.shift()!.release(false);
  const unavailable = await unavailableRead;
  assert.equal(unavailable.longOI, 0n);
  assert.equal(unavailable.shortOI, 0n);
  assert.deepEqual(unavailable.openInterest, {
    origin: `${firstVenue.origin}/alternate`,
    source: "unavailable",
    fetchedAt: null,
    ageMs: null,
  });
  await alternate.close();
  await first.close();
  await equivalent.close();
  await second.close();
}
test(quoterCacheOrigin);

async function priceFreshnessClock(t: TestContext) {
  // Close shared peers before the root suite waits for event-loop idleness.
  t.after(closeFixtures);
  const hook = await mount(() => modules.prices.useRealtimePrices(), undefined);
  await until(() =>
    peers.some(
      (peer) => peer.path === "/ws/markets" && peer.topics.has("markets@all"),
    ),
  );
  const peer = peers.find(
    (peer) =>
      peer.path === "/ws/markets" &&
      peer.socket.readyState === 1 &&
      peer.topics.has("markets@all"),
  )!;
  const updatedAt = Date.now();
  await act(async () =>
    peer.publishMarkets([
      {
        symbol: "BTC",
        price: 50_000,
        price_updated_at: updatedAt,
        last_updated: updatedAt,
      },
      {
        symbol: "ETH",
        price: 2000,
        price_updated_at: updatedAt - 15_000,
        last_updated: updatedAt,
      },
    ]),
  );
  await until(
    () => hook.read().prices.BTC === 50_000 && hook.read().prices.ETH === 2000,
  );
  const sent = peer.framesSent;
  await until(() => hook.read().prices.ETH === undefined, 16_000);
  assert.equal(hook.read().prices.BTC, 50_000);
  assert.equal(hook.read().tickers.ETH, undefined);
  await until(() => hook.read().prices.BTC === undefined, 16_000);
  assert.ok(Date.now() - updatedAt >= 30_000);
  assert.deepEqual(hook.read(), { prices: {}, tickers: {} });
  assert.equal(peer.socket.readyState, 1, "the socket remains open at expiry");
  assert.equal(
    peer.framesSent,
    sent,
    "no market or control frame drove expiry",
  );
  await hook.close();
}
test(priceFreshnessClock);
