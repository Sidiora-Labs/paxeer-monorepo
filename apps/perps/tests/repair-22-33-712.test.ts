import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { createServer, type ServerResponse } from "node:http";
import { createRequire } from "node:module";
import type { AddressInfo } from "node:net";
import type { Duplex } from "node:stream";
import { after, afterEach, before, test } from "node:test";
import {
  isMainThread,
  parentPort,
  Worker,
  workerData,
} from "node:worker_threads";
import { createElement, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  decodeFunctionData,
  encodeFunctionResult,
  type Address,
  type Hex,
} from "viem";
import type {
  SpotHoldingsResponse,
  SpotOrder,
  SpotPosition,
  SpotPnlSummary,
  SpotTrade,
} from "../src/core/spot/api";
import type { SpotPortfolioState } from "../src/hooks/spot/useSpotPortfolio";
import type { SpotUserStreamName } from "../src/hooks/spot/useSpotUserStream";

// Task record: every finding is exercised through production hooks and transports.
// No unchanged-tree gate ran: step 9 conflicts with the ordered implementation-first gates.
const repairRecord = {
  "spot-portfolio-address-state": "spotPortfolioAddressState",
  "spot-snapshot-live-ordering": "spotSnapshotLiveOrdering",
  "spot-stream-fetch-cancellation": "spotStreamFetchCancellation",
  "registry-spot-state": "registrySpotState",
  "spot-user-frame-selection": "spotUserFrameSelection",
  wiring:
    "Domain 3: reconcile spec/layerx-merge/interfaces.kvx row 3.ts-perps-hooks-usespottokenstate-j27 with contracts/perps-client.md#hooks (outside scope).",
};

// Use the installed React reconciler with an in-memory host, as repair-22-33-11 does.
// The components and their effects, state and query provider are the production code.
const localRequire = createRequire(import.meta.url);
const rendererRequire = createRequire(
  localRequire.resolve("@react-three/fiber"),
);
interface Renderer {
  createContainer(
    container: object,
    tag: number,
    hydration: null,
    strict: boolean,
    concurrent: null,
    prefix: string,
    onError: (error: Error) => void,
    transition: null,
  ): unknown;
  updateContainer(
    node: ReactNode,
    root: unknown,
    parent: null,
    callback: null,
  ): void;
  flushSync<T>(callback: () => T): T;
  flushPassiveEffects(): boolean;
}
const makeRenderer: (config: Record<string, unknown>) => Renderer =
  rendererRequire("react-reconciler");
const context = {};
const renderer = makeRenderer({
  now: Date.now,
  isPrimaryRenderer: true,
  supportsMutation: true,
  supportsPersistence: false,
  supportsHydration: false,
  getRootHostContext: () => context,
  getChildHostContext: () => context,
  getPublicInstance: () => null,
  prepareForCommit: () => null,
  resetAfterCommit() {},
  clearContainer() {},
  scheduleTimeout: setTimeout,
  cancelTimeout: clearTimeout,
  noTimeout: -1,
  getCurrentEventPriority: () => 16,
  detachDeletedInstance() {},
});
const disposals = new Set<() => void>();
function flush() {
  renderer.flushSync(() => {});
  while (renderer.flushPassiveEffects()) renderer.flushSync(() => {});
}
function mount<P, T>(
  read: (props: P) => T,
  initial: P,
  wrap: (node: ReactNode) => ReactNode = (node) => node,
) {
  let props = initial;
  let value: T;
  const renders: T[] = [];
  function Probe() {
    value = read(props);
    renders.push(value);
    return null;
  }
  const root = renderer.createContainer(
    {},
    0,
    null,
    false,
    null,
    "",
    (error) => {
      throw error;
    },
    null,
  );
  function render(next: P) {
    props = next;
    renderer.flushSync(() =>
      renderer.updateContainer(wrap(createElement(Probe)), root, null, null),
    );
    flush();
  }
  function stop() {
    renderer.flushSync(() => renderer.updateContainer(null, root, null, null));
    flush();
    disposals.delete(stop);
  }
  disposals.add(stop);
  render(initial);
  return {
    render,
    stop,
    renders,
    read() {
      flush();
      return value!;
    },
  };
}
async function waitFor<T>(
  probe: () => T,
  label: string,
  timeout = 5000,
): Promise<Exclude<T, false | null | undefined>> {
  const until = Date.now() + timeout;
  for (;;) {
    flush();
    const value = probe();
    if (value) return value as Exclude<T, false | null | undefined>;
    if (Date.now() >= until) throw new Error(`Timed out waiting for ${label}`);
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}
const tick = () => new Promise((resolve) => setTimeout(resolve, 40));
const A: Address = "0x00000000000000000000000000000000000000aa";
const B: Address = "0x00000000000000000000000000000000000000bb";
const TOKEN: Address = "0x0000000000000000000000000000000000000011";
const PAUSED: Address = "0x0000000000000000000000000000000000000022";
const UNREGISTERED: Address = "0x0000000000000000000000000000000000000033";
const POINTER: Address = "0x0000000000000000000000000000000000000044";
const streams: SpotUserStreamName[] = [
  "trade",
  "order",
  "position",
  "pnl",
  "closed-leg",
];

function holding(address: string, balance: string): SpotHoldingsResponse {
  return {
    address,
    native: { symbol: "PAX", decimals: 18, balance },
    erc20: [],
  };
}
function position(qty: string, token = TOKEN): SpotPosition {
  return {
    token,
    symbol: "SID",
    name: "SID",
    decimals: 18,
    isStablecoin: false,
    derived: {
      qty,
      avgEntryPrice: "1",
      avgExitPrice: null,
      totalBought: qty,
      totalSold: "0",
      totalCostBasis: qty,
      totalProceeds: "0",
      realizedPnl: "0",
      realizedPnlUsd: "0",
      tradeCount: 1,
      firstTradeAt: null,
      lastTradeAt: null,
    },
    paxscan: { qty },
    externalActivity: false,
    externalQtyDelta: "0",
  };
}
function order(id: string, status: SpotOrder["status"] = "open"): SpotOrder {
  return {
    id,
    orderId: id,
    type: "limit",
    status,
    token: TOKEN,
    stablecoin: null,
    amount: "10",
    targetPrice: "1",
    stopPrice: null,
    limitPrice: "1",
    triggerPrice: null,
    executionPrice: null,
    isBuy: true,
    placedAt: null,
    executedAt: null,
    cancelledAt: null,
    placedTxHash: null,
    executedTxHash: null,
    symbol: "SID",
    decimals: 18,
  };
}
function trade(address: string, block = "100"): SpotTrade {
  return {
    id: `${address}:${block}`,
    user: address,
    blockNumber: block,
    logIndex: 0,
    txHash: `0x${"1".repeat(64)}`,
    blockTimestamp: "2026-10-22T00:00:00Z",
    tokenIn: TOKEN,
    tokenOut: POINTER,
    amountIn: "10",
    amountOut: "20",
    executionPrice: "2",
    volumeUsd: "20",
    source: "router",
    contract: TOKEN,
    createdAt: "2026-10-22T00:00:00Z",
  };
}

// RFC6455 loopback peer: the browser's real WebSocket connects over TCP.
class Peer {
  received: { op: string; stream?: SpotUserStreamName; address?: string }[] =
    [];
  subscriptions = new Set<string>();
  ended = false;
  private buffer = Buffer.alloc(0);
  constructor(private socket: Duplex) {
    socket.on("data", (chunk) => {
      this.buffer = Buffer.concat([this.buffer, chunk]);
      this.drain();
    });
    socket.on("close", () => {
      this.ended = true;
    });
    socket.on("error", () => {
      this.ended = true;
    });
  }
  private drain() {
    while (this.buffer.length >= 2) {
      const opcode = this.buffer[0]! & 15;
      let length = this.buffer[1]! & 127;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2);
        offset = 4;
      }
      if (length === 127) {
        if (this.buffer.length < 10) return;
        length = Number(this.buffer.readBigUInt64BE(2));
        offset = 10;
      }
      if (this.buffer.length < offset + 4 + length) return;
      const mask = this.buffer.subarray(offset, offset + 4);
      const data = Buffer.from(
        this.buffer.subarray(offset + 4, offset + 4 + length),
      );
      for (let i = 0; i < data.length; i++) data[i] = data[i]! ^ mask[i % 4]!;
      this.buffer = this.buffer.subarray(offset + 4 + length);
      if (opcode === 1) {
        const frame = JSON.parse(
          data.toString(),
        ) as (typeof this.received)[number];
        this.received.push(frame);
        const key = `${frame.stream}|${frame.address}`;
        if (frame.op === "subscribe") this.subscriptions.add(key);
        if (frame.op === "unsubscribe") this.subscriptions.delete(key);
      }
      if (opcode === 8) {
        this.write(0x88, data);
        this.socket.end();
      }
    }
  }
  private write(opcode: number, data: Buffer) {
    if (this.ended) return;
    const header =
      data.length < 126
        ? Buffer.from([opcode, data.length])
        : Buffer.from([opcode, 126, data.length >> 8, data.length & 255]);
    this.socket.write(Buffer.concat([header, data]));
  }
  publish(stream: SpotUserStreamName, address: string, data: unknown) {
    if (this.subscriptions.has(`${stream}|${address}`))
      this.write(0x81, Buffer.from(JSON.stringify({ stream, address, data })));
  }
  close(code = 1012) {
    const data = Buffer.alloc(2);
    data.writeUInt16BE(code);
    this.write(0x88, data);
  }
  destroy() {
    this.socket.destroy();
  }
}
interface Account {
  holdings: SpotHoldingsResponse;
  positions: SpotPosition[];
  orders: SpotOrder[];
  trades: SpotTrade[];
  pnl: SpotPnlSummary;
}
interface Read {
  address: string;
  resource: string;
  response: ServerResponse;
  body: unknown;
  status: number;
  held: boolean;
}
class SpotGateway {
  accounts = new Map<string, Account>();
  peers: Peer[] = [];
  reads: Read[] = [];
  hold: (address: string, resource: string) => boolean = () => false;
  fail: (address: string, resource: string) => boolean = () => false;
  rpcFailure = false;
  rpcCalls: { functionName: string; token?: string }[] = [];
  rpcResult: (data: Hex) => Hex = () => {
    throw new Error("RPC codec not initialized");
  };
  server = createServer((request, response) => {
    response.setHeader("content-type", "application/json");
    if (request.method === "POST") {
      void (async () => {
        const chunks: Buffer[] = [];
        for await (const chunk of request) chunks.push(Buffer.from(chunk));
        const call = JSON.parse(Buffer.concat(chunks).toString()) as {
          id: number;
          method: string;
          params?: [{ data: Hex }];
        };
        const result =
          call.method === "eth_call"
            ? this.rpcResult(call.params![0].data)
            : call.method === "eth_chainId"
              ? "0x7d"
              : "0x1";
        response.end(JSON.stringify({ jsonrpc: "2.0", id: call.id, result }));
      })().catch((error) => {
        response.statusCode = 500;
        response.end(JSON.stringify({ error: String(error) }));
      });
      return;
    }
    const path = new URL(request.url!, "http://fixture.test").pathname;
    const match = /^\/v1\/users\/([^/]+)\/(.+)$/.exec(path);
    assert.ok(match, `Unexpected fixture request ${path}`);
    const address = match[1]!;
    const resource = match[2]!;
    const account = this.accounts.get(address);
    assert.ok(account, `Unknown fixture account ${address}`);
    const body =
      resource === "holdings"
        ? account.holdings
        : resource === "positions"
          ? { address, positions: account.positions }
          : resource === "orders"
            ? {
                address,
                count: account.orders.filter(
                  (o) => o.status === "open" || o.status === "activated",
                ).length,
                items: account.orders.filter(
                  (o) => o.status === "open" || o.status === "activated",
                ),
              }
            : resource === "trades"
              ? {
                  address,
                  count: account.trades.length,
                  limit: 50,
                  items: account.trades,
                  nextCursor: null,
                }
              : resource === "pnl/summary"
                ? account.pnl
                : null;
    assert.ok(body, `Unexpected fixture resource ${resource}`);
    const read: Read = {
      address,
      resource,
      response,
      body: structuredClone(body),
      status: this.fail(address, resource) ? 403 : 200,
      held: this.hold(address, resource),
    };
    this.reads.push(read);
    if (!read.held) this.reply(read);
  });
  constructor() {
    this.server.on("upgrade", (request, socket) => {
      assert.equal(request.url, "/v1/ws");
      const accept = createHash("sha1")
        .update(
          `${request.headers["sec-websocket-key"]}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`,
        )
        .digest("base64");
      socket.write(
        `HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\n\r\n`,
      );
      this.peers.push(new Peer(socket));
    });
  }
  seed(address: string, quantity: string) {
    this.accounts.set(address, {
      holdings: holding(address, quantity),
      positions: [position(quantity)],
      orders: [order(`${address}:order`)],
      trades: [trade(address)],
      pnl: { address, realizedUsd: quantity },
    });
  }
  reply(read: Read) {
    read.held = false;
    if (read.response.destroyed) return;
    read.response.writeHead(read.status);
    read.response.end(
      JSON.stringify(
        read.status === 200 ? read.body : { error: "Fixture read refused" },
      ),
    );
  }
  release() {
    this.reads.filter((read) => read.held).forEach((read) => this.reply(read));
  }
  publish(stream: SpotUserStreamName, address: string, data: unknown) {
    for (const peer of this.peers) peer.publish(stream, address, data);
  }
  updateOrder(address: string, next: SpotOrder) {
    const account = this.accounts.get(address)!;
    account.orders = [
      next,
      ...account.orders.filter((item) => item.orderId !== next.orderId),
    ];
    this.publish("order", address, next);
  }
  updatePosition(address: string, next: SpotPosition) {
    const account = this.accounts.get(address)!;
    account.positions = [
      next,
      ...account.positions.filter((item) => item.token !== next.token),
    ];
    this.publish("position", address, next);
  }
  updatePnl(address: string, realizedUsd: string) {
    const pnl = { address, realizedUsd };
    this.accounts.get(address)!.pnl = pnl;
    this.publish("pnl", address, pnl);
  }
  fill(address: string, block = "101") {
    const next = trade(address, block);
    this.accounts.get(address)!.trades.unshift(next);
    this.publish("trade", address, next);
  }
  async subscribed(address: string, requested = streams) {
    return waitFor(
      () =>
        this.peers.findLast(
          (peer) =>
            !peer.ended &&
            requested.every((stream) =>
              peer.subscriptions.has(`${stream}|${address}`),
            ),
        ),
      "gateway subscriptions",
    );
  }
}
let gateway: SpotGateway;
let origin: string;
let portfolio: typeof import("../src/hooks/spot/useSpotPortfolio");
let user: typeof import("../src/hooks/spot/useSpotUserStream");
function configure(fixtureOrigin: string) {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = fixtureOrigin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = fixtureOrigin;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = fixtureOrigin.replace(
    "http:",
    "ws:",
  );
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = fixtureOrigin;
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = fixtureOrigin;
}
async function loaded(view: { read(): SpotPortfolioState }, address: string) {
  await gateway.subscribed(address);
  await waitFor(
    () => view.read().hasLoadedOnce && !view.read().isLoading,
    "portfolio snapshot",
  );
}
function emptyAccount(state: SpotPortfolioState) {
  assert.equal(state.holdings, null);
  assert.deepEqual(state.positions, []);
  assert.deepEqual(state.openOrders, []);
  assert.deepEqual(state.recentTrades, []);
  assert.equal(state.tradeHistory, null);
  assert.equal(state.pnlSummary, null);
  assert.equal(state.tradeHistoryError, null);
  assert.deepEqual(state.lastFrames, {
    trade: null,
    order: null,
    position: null,
    pnl: null,
    closedLeg: null,
  });
}

async function spotPortfolioAddressState() {
  for (const failure of ["partial", "all"]) {
    gateway.seed(A, "10");
    gateway.seed(B, "20");
    const view = mount(portfolio.useSpotPortfolio, A as string | null);
    await loaded(view, A);
    gateway.fill(A);
    gateway.updateOrder(A, order("a-new"));
    gateway.updatePosition(A, position("11"));
    gateway.updatePnl(A, "11");
    gateway.publish("closed-leg", A, { id: 1, token: TOKEN });
    await waitFor(
      () => Object.values(view.read().lastFrames).every(Boolean),
      "all A frames",
    );
    gateway.hold = (address) => address === B;
    gateway.fail = (address, resource) =>
      address === B && (failure === "all" || resource !== "holdings");
    view.render(B);
    const switched = view.renders.findLast(
      (state) => state.isConnected && !state.hasLoadedOnce,
    );
    assert.ok(switched);
    emptyAccount(switched);
    assert.equal(switched.error, null);
    await waitFor(
      () =>
        gateway.reads.filter((read) => read.address === B && read.held)
          .length >= 5,
      "B reads",
    );
    gateway.release();
    await loaded(view, B);
    assert.deepEqual(view.read().positions, []);
    assert.deepEqual(view.read().openOrders, []);
    assert.deepEqual(view.read().recentTrades, []);
    assert.equal(view.read().pnlSummary, null);
    assert.ok(view.read().tradeHistoryError);
    assert.deepEqual(view.read().lastFrames, {
      trade: null,
      order: null,
      position: null,
      pnl: null,
      closedLeg: null,
    });
    if (failure === "all") {
      assert.equal(view.read().holdings, null);
      assert.ok(view.read().error);
    } else {
      assert.deepEqual(view.read().holdings, holding(B, "20"));
      assert.equal(view.read().error, null);
    }
    gateway.updatePosition(B, position("25"));
    await waitFor(
      () => view.read().positions[0]?.derived.qty === "25",
      "B position",
    );
    assert.equal(view.read().positions.length, 1);
    const before = gateway.reads.length;
    view.render(B.toUpperCase());
    assert.equal(view.read().positions[0]?.derived.qty, "25");
    assert.equal(
      gateway.reads.length,
      before,
      "case-only account changes retain the same generation",
    );
    gateway.hold = () => false;
    gateway.fail = () => true;
    view.read().refresh();
    await waitFor(
      () => gateway.reads.length >= before + 5 && !view.read().isLoading,
      "same-account failed refresh",
    );
    assert.equal(view.read().positions[0]?.derived.qty, "25");
    assert.equal(
      view.read().holdings?.native.balance ?? null,
      failure === "all" ? null : "20",
    );
    assert.ok(view.read().error);
    view.render(null);
    emptyAccount(view.read());
    assert.equal(view.read().hasLoadedOnce, false);
    assert.equal(view.read().error, null);
    view.stop();
    await waitFor(
      () => gateway.peers.every((peer) => peer.ended),
      "account socket cleanup",
    );
    gateway.hold = () => false;
    gateway.fail = () => false;
  }
}

async function spotSnapshotLiveOrdering() {
  gateway.seed(A, "10");
  gateway.hold = () => true;
  let view = mount(portfolio.useSpotPortfolio, A);
  await gateway.subscribed(A);
  await waitFor(
    () =>
      gateway.reads.filter((read) => read.held && !read.response.destroyed)
        .length === 5,
    "initial held snapshot",
  );
  async function pushAndRelease(qty: string, closed: string, opened: string) {
    gateway.updateOrder(A, order(closed, "executed"));
    gateway.updateOrder(A, order(opened));
    gateway.updateOrder(A, order(opened));
    gateway.updatePosition(A, position(qty));
    gateway.updatePosition(A, position(qty));
    gateway.updatePnl(A, qty);
    await waitFor(
      () =>
        view.read().pnlSummary?.realizedUsd === qty &&
        view.read().positions[0]?.derived.qty === qty,
      "live update before snapshot",
    );
    gateway.release();
    await loaded(view, A);
    assert.deepEqual(
      view.read().openOrders.map((item) => item.orderId),
      [opened],
    );
    assert.equal(view.read().positions.length, 1);
    assert.equal(view.read().positions[0]?.derived.qty, qty);
    assert.equal(view.read().pnlSummary?.realizedUsd, qty);
  }
  await pushAndRelease("20", `${A}:order`, "second");
  const previousReads = gateway.reads.length;
  const first = await gateway.subscribed(A);
  first.close();
  await waitFor(() => first.ended, "socket stopped");
  await gateway.subscribed(A);
  await waitFor(
    () => gateway.reads.length >= previousReads + 5,
    "recovery snapshot",
  );
  await pushAndRelease("30", "second", "third");
  view.stop();
  await waitFor(
    () => gateway.peers.every((peer) => peer.ended),
    "component stopped",
  );
  const restartReads = gateway.reads.length;
  view = mount(portfolio.useSpotPortfolio, A);
  await gateway.subscribed(A);
  await waitFor(
    () =>
      gateway.reads.length >= restartReads + 5 &&
      gateway.reads.filter((read) => read.held && !read.response.destroyed)
        .length === 5,
    "reopened snapshot",
  );
  await pushAndRelease("40", "third", "fourth");
}

async function spotStreamFetchCancellation() {
  gateway.seed(A, "10");
  gateway.seed(B, "20");
  const view = mount(portfolio.useSpotPortfolio, A as string | null);
  await loaded(view, A);
  gateway.hold = (address) => address === A;
  gateway.accounts.get(A)!.holdings = holding(A, "999");
  gateway.accounts.get(A)!.positions = [position("999")];
  gateway.accounts.get(A)!.pnl = { address: A, realizedUsd: "999" };
  gateway.fill(A);
  await waitFor(
    () =>
      gateway.reads.filter((read) => read.address === A && read.held).length ===
      3,
    "first A trade refresh",
  );
  gateway.fill(A, "102");
  const late = await waitFor(() => {
    const reads = gateway.reads.filter(
      (read) => read.address === A && read.held,
    );
    return reads.length === 6 && reads;
  }, "both trade-triggered A refreshes");
  view.render(B);
  await loaded(view, B);
  await waitFor(
    () => late.every((read) => read.response.destroyed),
    "all A refresh controllers canceled",
  );
  gateway.release();
  await tick();
  assert.deepEqual(view.read().holdings, holding(B, "20"));
  assert.deepEqual(view.read().positions, [position("20")]);
  assert.deepEqual(view.read().pnlSummary, { address: B, realizedUsd: "20" });
  assert.equal(view.read().lastFrames.trade, null);
  assert.ok(view.read().recentTrades.every((item) => item.user === B));
  gateway.hold = (address) => address === B;
  gateway.fill(B);
  const unmounted = await waitFor(() => {
    const reads = gateway.reads.filter(
      (read) => read.address === B && read.held,
    );
    return reads.length === 3 && reads;
  }, "B refreshes before unmount");
  view.stop();
  await waitFor(
    () => unmounted.every((read) => read.response.destroyed),
    "unmounted refresh controllers canceled",
  );
}

async function spotUserFrameSelection() {
  gateway.seed(A, "10");
  gateway.seed(B, "20");
  type Selection = {
    address: string | null;
    stream: SpotUserStreamName;
    streams: SpotUserStreamName[];
  };
  const view = mount(
    (selection: Selection) => ({
      single: user.useSpotUserStream(selection.stream, selection.address),
      multiple: user.useSpotUserStreams(selection.streams, selection.address),
    }),
    { address: A, stream: "trade", streams: ["trade", "order"] } as Selection,
  );
  await gateway.subscribed(A, ["trade", "order"]);
  gateway.fill(A);
  gateway.updateOrder(A, order("a-frame"));
  await waitFor(
    () => view.read().single && view.read().multiple.order,
    "A retained frames",
  );
  view.render({ address: B, stream: "trade", streams: ["trade", "order"] });
  assert.equal(view.read().single, null);
  assert.deepEqual(view.read().multiple, {});
  await gateway.subscribed(B, ["trade", "order"]);
  gateway.fill(A);
  await tick();
  assert.equal(view.read().single, null);
  assert.deepEqual(view.read().multiple, {});
  gateway.fill(B);
  await waitFor(
    () => view.read().single && view.read().multiple.trade,
    "B frame",
  );
  assert.equal(view.read().single?.address, B);
  const beforeCase = view.read().single;
  view.render({
    address: B.toUpperCase(),
    stream: "trade",
    streams: ["order", "trade", "trade"],
  });
  assert.equal(
    view.read().single,
    beforeCase,
    "normalized subscription identity is stable",
  );
  view.render({ address: B, stream: "order", streams: ["order"] });
  assert.equal(view.read().single, null);
  assert.deepEqual(view.read().multiple, {});
  gateway.fill(B);
  await tick();
  assert.equal(view.read().single, null);
  assert.deepEqual(view.read().multiple, {});
  gateway.updateOrder(B, order("b-frame"));
  await waitFor(
    () => view.read().single && view.read().multiple.order,
    "order selection frame",
  );
  assert.deepEqual(Object.keys(view.read().multiple), ["order"]);
  view.render({ address: B, stream: "trade", streams: ["trade"] });
  assert.equal(view.read().single, null);
  assert.deepEqual(view.read().multiple, {});
  view.render({ address: null, stream: "trade", streams: [] });
  assert.equal(view.read().single, null);
  assert.deepEqual(view.read().multiple, {});
}

// Isolate the real RPC reader's long-lived ranking timers in a worker; no subprocess
// or external toolchain is involved, and the parent owns the loopback RPC endpoint.
async function tokenStateWorker() {
  configure(workerData.origin as string);
  const { useSpotTokenState } =
    await import("../src/hooks/spot/useSpotTokenState");
  const queries = new QueryClient({
    defaultOptions: { queries: { gcTime: Infinity } },
  });
  const view = mount(
    (input: { token: Address; native: boolean }) =>
      useSpotTokenState(input.token, input.native),
    { token: TOKEN, native: false },
    (node) => createElement(QueryClientProvider, { client: queries }, node),
  );
  assert.deepEqual(view.read(), { state: null, error: null });
  await waitFor(() => view.read().state, "registered token state");
  assert.deepEqual(view.read(), {
    state: { tradeable: true, paused: false, reason: null },
    error: null,
  });
  view.render({ token: PAUSED, native: false });
  assert.deepEqual(view.read(), { state: null, error: null });
  await waitFor(() => view.read().state, "paused token state");
  assert.deepEqual(view.read().state, {
    tradeable: false,
    paused: true,
    reason: "TokenPaused",
  });
  view.render({ token: UNREGISTERED, native: false });
  await waitFor(() => view.read().state, "unregistered token state");
  assert.deepEqual(view.read().state, {
    tradeable: false,
    paused: false,
    reason: "TokenNotRegistered",
  });
  view.render({ token: TOKEN, native: true });
  await waitFor(() => view.read().state, "native token pointer state");
  assert.equal(view.read().state?.tradeable, true);
  parentPort!.postMessage({ phase: "fail-rpc" });
  await new Promise<void>((resolve) =>
    parentPort!.once("message", () => resolve()),
  );
  await queries.refetchQueries({
    queryKey: ["spot", "tokenState", TOKEN.toLowerCase(), true],
    exact: true,
  });
  await waitFor(() => view.read().error, "failed token read");
  assert.equal(
    view.read().state,
    null,
    "errors must hide stale cached token state",
  );
  assert.equal(typeof view.read().error, "string");
  assert.ok(view.read().error!.length > 0);
  view.stop();
  queries.clear();
}

async function registrySpotState() {
  const docs = readFileSync(
    new URL("../../../contracts/perps-client.md", import.meta.url),
    "utf8",
  );
  const hooks = docs.split("## Hooks\n")[1]!.split("## Routes")[0]!;
  assert.match(
    hooks,
    /useSpotTokenState\(token: Address, native\?: boolean\): SpotTokenStateResult/,
  );
  assert.match(hooks, /state: SpotTokenState \| null, error: string \| null/);
  assert.match(
    hooks,
    /While the initial read is loading, both fields are null/,
  );
  assert.match(hooks, /failed read makes it unavailable/);
  assert.match(hooks, /3\.ts-perps-hooks-usespottokenstate-j27/);
  assert.equal(Object.keys(repairRecord).length, 6);
  const worker = new Worker(new URL(import.meta.url), {
    workerData: { origin },
    execArgv: ["--import", "tsx"],
  });
  let deadline: ReturnType<typeof setTimeout>;
  try {
    await new Promise<void>((resolve, reject) => {
      deadline = setTimeout(
        () => reject(new Error("Token-state worker timed out")),
        15000,
      );
      worker.on("message", (message: { phase: string; error?: string }) => {
        if (message.phase === "fail-rpc") {
          gateway.rpcFailure = true;
          worker.postMessage("ready");
        } else if (message.phase === "done") resolve();
        else reject(new Error(message.error));
      });
      worker.on("error", reject);
      worker.on("exit", (code) => {
        if (code !== 0) reject(new Error(`token state worker exit ${code}`));
      });
    });
    assert.ok(gateway.rpcCalls.some((call) => call.functionName === "WETH"));
    assert.ok(
      gateway.rpcCalls.some(
        (call) =>
          call.functionName === "isTokenPaused" && call.token === POINTER,
      ),
    );
    assert.ok(
      gateway.rpcCalls.some(
        (call) =>
          call.functionName === "getTokenInfo" && call.token === POINTER,
      ),
    );
  } finally {
    clearTimeout(deadline!);
    await worker.terminate();
  }
}

if (isMainThread) {
  before(async () => {
    gateway = new SpotGateway();
    await new Promise<void>((resolve) =>
      gateway.server.listen(0, "127.0.0.1", resolve),
    );
    origin = `http://127.0.0.1:${(gateway.server.address() as AddressInfo).port}`;
    configure(origin);
    // All application modules, including RPC ABIs, are imported after fixture URLs.
    const { pecorVaultAbi } =
      await import("../src/core/network/spot_abi/PECORVault");
    const { pecorAbi } = await import("../src/core/network/spot_abi/PECOR");
    gateway.rpcResult = (data) => {
      try {
        const decoded = decodeFunctionData({ abi: pecorVaultAbi, data });
        const token = decoded.args?.[0] as Address;
        gateway.rpcCalls.push({
          functionName: decoded.functionName,
          token: token?.toLowerCase(),
        });
        if (gateway.rpcFailure) return "0x";
        if (decoded.functionName === "isTokenPaused")
          return encodeFunctionResult({
            abi: pecorVaultAbi,
            functionName: "isTokenPaused",
            result: token.toLowerCase() === PAUSED,
          });
        assert.equal(decoded.functionName, "getTokenInfo");
        return encodeFunctionResult({
          abi: pecorVaultAbi,
          functionName: "getTokenInfo",
          result: [token.toLowerCase() !== UNREGISTERED, false, 18, 0n, 0n, 0n],
        });
      } catch (error) {
        const decoded = decodeFunctionData({ abi: pecorAbi, data });
        assert.equal(decoded.functionName, "WETH", String(error));
        gateway.rpcCalls.push({ functionName: "WETH" });
        return encodeFunctionResult({
          abi: pecorAbi,
          functionName: "WETH",
          result: POINTER,
        });
      }
    };
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: new EventTarget(),
    });
    Object.defineProperty(globalThis, "document", {
      configurable: true,
      value: Object.assign(new EventTarget(), { hidden: false }),
    });
    portfolio = await import("../src/hooks/spot/useSpotPortfolio");
    user = await import("../src/hooks/spot/useSpotUserStream");
  });
  afterEach(async () => {
    disposals.forEach((dispose) => dispose());
    gateway.release();
    await waitFor(
      () => gateway.peers.every((peer) => peer.ended),
      "socket teardown",
    );
    gateway.hold = () => false;
    gateway.fail = () => false;
    gateway.reads = [];
    gateway.peers = [];
    gateway.accounts.clear();
    gateway.rpcFailure = false;
  });
  after(async () => {
    for (const peer of gateway.peers) peer.destroy();
    gateway.server.closeAllConnections();
    await new Promise<void>((resolve) => gateway.server.close(() => resolve()));
    Reflect.deleteProperty(globalThis, "window");
    Reflect.deleteProperty(globalThis, "document");
  });
  test(spotPortfolioAddressState);
  test(spotSnapshotLiveOrdering);
  test(spotStreamFetchCancellation);
  test(spotUserFrameSelection);
  test(registrySpotState);
} else {
  void tokenStateWorker().then(
    () => parentPort!.postMessage({ phase: "done" }),
    (error: unknown) =>
      parentPort!.postMessage({
        phase: "error",
        error: error instanceof Error ? error.stack : String(error),
      }),
  );
}
