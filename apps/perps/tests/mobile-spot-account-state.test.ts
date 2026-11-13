/**
 * Task 22.33.201 / audit 22.30.12:
 * f046: mobileOrdersRecoverAfterAccountChangesAndRestart and
 * mobileTradesRecoverAfterAccountChangesAndRestart exercise the production
 * wallet provider, mobile tabs, HTTP transport and GraphQL fixture snapshots.
 * f047: completeMobileSpotTradeViewIncludesTheRealBalancePanel renders the
 * entire production view with its balance child and Next dynamic boundaries.
 * Wiring request: domain 3 owns tests/proof_widgets.test.ts; replace its
 * filtered MobileConsumer render with the full-view assertion below.
 * Baseline verification was not run: the bounded gate order forbids testing
 * before implementation, and the new repair test did not exist on the base.
 */
import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { createServer, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";
import { createRequire } from "node:module";
import { setTimeout as delay } from "node:timers/promises";
import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createConfig, http, WagmiProvider } from "wagmi";
import { injected } from "wagmi/connectors";

type Kind = "orders" | "trades";
type Wallet = `0x${string}`;
const accountA: Wallet = "0x1111111111111111111111111111111111111111";
const accountB: Wallet = "0x2222222222222222222222222222222222222222";

// Committed GraphQL response fixtures, using the fields selected by the real
// GetOrders/GetTrades documents. Each snapshot has a distinct visible price.
const snapshots = {
  orders: [
    { orderId: "101", userAddress: accountA, marketId: 0, orderType: 0,
      orderTypeName: "Limit", isLong: true, triggerPrice: "11000000000000000000",
      sizeUsd: "100000000000000000000", status: "active", positionId: null,
      executionPrice: null, failureReason: null, placedAt: "1730000000",
      resolvedAt: null, placedTxHash: "0x101", resolvedTxHash: null },
    { orderId: "202", userAddress: accountB, marketId: 0, orderType: 0,
      orderTypeName: "Limit", isLong: false, triggerPrice: "22000000000000000000",
      sizeUsd: "200000000000000000000", status: "active", positionId: null,
      executionPrice: null, failureReason: null, placedAt: "1730000001",
      resolvedAt: null, placedTxHash: "0x202", resolvedTxHash: null },
  ],
  trades: [
    { id: "301", positionId: "31", userAddress: accountA, marketId: 0,
      tradeType: 0, isLong: true, sizeUsd: "100000000000000000000",
      price: "11000000000000000000", realizedPnl: "1000000000000000000",
      feeUsd: "10000000000000000", blockNumber: "100", txHash: "0x301",
      blockTimestamp: "1730000000" },
    { id: "402", positionId: "42", userAddress: accountB, marketId: 0,
      tradeType: 0, isLong: false, sizeUsd: "200000000000000000000",
      price: "22000000000000000000", realizedPnl: "2000000000000000000",
      feeUsd: "20000000000000000", blockNumber: "101", txHash: "0x402",
      blockTimestamp: "1730000001" },
  ],
};

interface Exchange {
  kind: Kind;
  address: string;
  variables: Record<string, unknown>;
  response: ServerResponse;
  cancelled: boolean;
}
const exchanges: Exchange[] = [];
const unexpected: string[] = [];
function respond(response: ServerResponse, body: unknown, status = 200) {
  if (response.destroyed) return;
  response.writeHead(status, { "content-type": "application/json" });
  response.end(JSON.stringify(body));
}
const server = createServer(async (request, response) => {
  const chunks: Buffer[] = [];
  for await (const chunk of request) chunks.push(Buffer.from(chunk));
  const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
  if (request.url === "/graphql" && /query GetMarkets\b/.test(body.query)) {
    respond(response, { data: { markets: [{
      marketId: 0, symbol: "BTC", name: "Bitcoin", oracleSymbol: "BTC",
      szDecimals: 3, pxDecimals: 2, tickSize: "10000000000000000",
      lotSize: "1000000000000000", tierClass: 0,
      maxLeverage: "100000000000000000000", enabled: true,
      createdAt: "2026-10-27T12:00:00Z", latestPrice: null, fundingRate: null,
    }] } });
    return;
  }
  const kind = /query GetOrders\b/.test(body.query) ? "orders"
    : /query GetTrades\b/.test(body.query) ? "trades" : null;
  if (request.url !== "/graphql" || !kind) {
    unexpected.push(`${request.method} ${request.url}: ${body.query}`);
    respond(response, { errors: [{ message: "Unrecorded request" }] }, 400);
    return;
  }
  const exchange: Exchange = {
    kind, address: body.variables.userAddress, variables: body.variables,
    response, cancelled: false,
  };
  response.on("close", () => { exchange.cancelled = !response.writableEnded; });
  exchanges.push(exchange);
});

let origin: string;
let secondary: typeof import("../src/widgets/mobile/MobileSecondaryTabContent");
let spotView: typeof import("../src/widgets/mobile/MobileSpotTradeView");
let spotConfig: typeof import("../src/core/spot/config");
let chain: typeof import("../src/core/chain");

before(async () => {
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  secondary = await import("../src/widgets/mobile/MobileSecondaryTabContent");
  spotView = await import("../src/widgets/mobile/MobileSpotTradeView");
  spotConfig = await import("../src/core/spot/config");
  chain = await import("../src/core/chain");
});
after(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

// Use the real React reconciler already shipped with the declared fiber
// dependency. The host is an in-memory tree; no hooks or modules are replaced.
interface MemoryNode {
  type: string;
  props: Record<string, unknown>;
  text?: string;
  children: MemoryNode[];
}
interface MemoryReconciler {
  createContainer(container: MemoryNode, tag: number, hydration: null, strict: boolean,
    concurrent: null, prefix: string, onError: (error: Error) => void, transitions: null): object;
  updateContainer(node: ReactNode, root: object, parent: null, callback: () => void): void;
  flushPassiveEffects(): boolean;
  flushSync(callback: () => void): void;
}
const fiberRequire = createRequire(createRequire(import.meta.url).resolve("@react-three/fiber"));
const createReconciler = fiberRequire("react-reconciler") as
  (host: Record<string, unknown>) => MemoryReconciler;

function memoryRenderer() {
  const container: MemoryNode = { type: "root", props: {}, children: [] };
  const context = {};
  function remove(parent: MemoryNode, child: MemoryNode) {
    const index = parent.children.indexOf(child);
    assert.ok(index >= 0);
    parent.children.splice(index, 1);
  }
  function append(parent: MemoryNode, child: MemoryNode) {
    if (parent.children.includes(child)) remove(parent, child);
    parent.children.push(child);
  }
  function insert(parent: MemoryNode, child: MemoryNode, before: MemoryNode) {
    if (parent.children.includes(child)) remove(parent, child);
    const index = parent.children.indexOf(before);
    assert.ok(index >= 0);
    parent.children.splice(index, 0, child);
  }
  const renderer = createReconciler({
    now: Date.now, isPrimaryRenderer: false,
    supportsMutation: true, supportsPersistence: false, supportsHydration: false,
    getRootHostContext: () => context, getChildHostContext: () => context,
    getPublicInstance: (node: MemoryNode) => node,
    prepareForCommit: () => null, resetAfterCommit: () => {},
    createInstance: (type: string, props: Record<string, unknown>): MemoryNode => ({ type, props, children: [] }),
    createTextInstance: (text: string): MemoryNode => ({ type: "#text", text, props: {}, children: [] }),
    appendInitialChild: append, appendChild: append, appendChildToContainer: append,
    removeChild: remove, removeChildFromContainer: remove,
    insertBefore: insert, insertInContainerBefore: insert,
    finalizeInitialChildren: () => false, shouldSetTextContent: () => false,
    prepareUpdate: (_node: MemoryNode, _type: string, _old: unknown, props: Record<string, unknown>) => props,
    commitUpdate: (node: MemoryNode, props: Record<string, unknown>) => { node.props = props; },
    commitTextUpdate: (node: MemoryNode, _old: string, text: string) => { node.text = text; },
    resetTextContent: (node: MemoryNode) => { node.children = []; },
    clearContainer: (node: MemoryNode) => { node.children = []; },
    scheduleTimeout: setTimeout, cancelTimeout: clearTimeout, noTimeout: -1,
    getCurrentEventPriority: () => 16, detachDeletedInstance: () => {},
  });
  const root = renderer.createContainer(container, 0, null, false, null, "", (error) => { throw error; }, null);
  function render(node: ReactNode) {
    renderer.flushSync(() => renderer.updateContainer(node, root, null, () => {}));
    renderer.flushPassiveEffects();
  }
  function nodes(node: MemoryNode = container): MemoryNode[] {
    return [node, ...node.children.flatMap((child) => nodes(child))];
  }
  function text() {
    return nodes().filter((node) => node.type === "#text").map((node) => node.text).join("");
  }
  async function until(check: () => boolean, message: string) {
    const deadline = Date.now() + 3_000;
    while (Date.now() < deadline) {
      renderer.flushPassiveEffects();
      if (check()) return;
      await delay(5);
    }
    assert.fail(`${message}; rendered: ${text()}`);
  }
  return { renderer, render, nodes, text, until };
}

function providers() {
  const config = createConfig({
    chains: [chain.paxeerChain], connectors: [injected()],
    transports: { [chain.paxeerChain.id]: http(origin) },
    storage: null, multiInjectedProviderDiscovery: false,
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  function account(address: Wallet | null) {
    const connector = config.connectors[0];
    config.setState((state) => ({
      ...state, status: address ? "connected" : "disconnected",
      current: address ? connector.uid : null,
      connections: address ? new Map([[connector.uid, { accounts: [address], chainId: chain.paxeerChain.id, connector }]]) : new Map(),
    }));
  }
  function wrap(node: ReactNode) {
    return createElement(WagmiProvider, { config, reconnectOnMount: false },
      createElement(QueryClientProvider, { client }, node));
  }
  return { account, wrap, client };
}

async function recoverWalletRows(kind: Kind) {
  const start = exchanges.length;
  const host = memoryRenderer();
  const provider = providers();
  const tab = kind === "orders" ? "Open Orders" : "Trade History";
  const empty = kind === "orders" ? "No open orders" : "No trade history";
  const failure = kind === "orders" ? "Failed to load open orders" : "Failed to load trade history";
  const view = () => provider.wrap(createElement(secondary.MobileSecondaryTabContent, { activeTab: tab }));
  function account(address: Wallet | null) {
    host.renderer.flushSync(() => provider.account(address));
    host.renderer.flushPassiveEffects();
  }
  async function request(index: number, address: Wallet) {
    await host.until(() => exchanges.length > start + index, `request ${index} reached the fixture`);
    const exchange = exchanges[start + index];
    assert.equal(exchange.kind, kind);
    assert.equal(exchange.address, address);
    assert.deepEqual(exchange.variables, kind === "orders"
      ? { userAddress: address, status: "active" }
      : { userAddress: address, limit: 50 });
    return exchange;
  }
  function success(exchange: Exchange, accountIndex: 0 | 1) {
    respond(exchange.response, { data: { [kind]: [snapshots[kind][accountIndex]] } });
  }
  try {
    provider.account(accountA);
    host.render(view());
    const first = await request(0, accountA);
    account(accountB);
    const second = await request(1, accountB);
    await host.until(() => first.cancelled, "superseded account request was aborted at the transport");
    success(second, 1);
    await host.until(() => host.text().includes("$22.00"), "current wallet rows rendered");
    success(first, 0);
    await delay(10);
    assert.ok(!host.text().includes("$11.00"));
    assert.ok(host.text().includes("$22.00"));
    await host.until(() => host.text().includes("BTC/USD"), "real catalog reached the row consumer");

    account(accountA);
    assert.ok(!host.text().includes("$22.00"), "previous wallet rows reset immediately");
    const failed = await request(2, accountA);
    respond(failed.response, { errors: [{ message: "Indexer unavailable" }] }, 400);
    await host.until(() => host.text().includes(failure), "HTTP failure is visible");
    assert.ok(host.nodes().some((node) => node.props.role === "alert"));
    assert.ok(!host.text().includes(empty), "failed reads never masquerade as successful emptiness");

    account(accountB);
    assert.ok(!host.text().includes(failure), "wallet change resets its predecessor's error");
    const graphqlFailure = await request(3, accountB);
    respond(graphqlFailure.response, { errors: [{ message: "History read failed" }] });
    await host.until(() => host.text().includes(failure), "GraphQL failure is visible");
    const retry = host.nodes().find((node) => node.type === "button" && node.props.onClick);
    assert.ok(retry);
    host.renderer.flushSync(() => (retry.props.onClick as () => void)());
    host.renderer.flushPassiveEffects();
    assert.ok(!host.text().includes(failure));
    const retried = await request(4, accountB);
    respond(retried.response, { data: { [kind]: [] } });
    await host.until(() => host.text().includes(empty), "retry can recover to a successfully empty snapshot");
    assert.ok(!host.nodes().some((node) => node.props.role === "alert"));

    account(accountA);
    const stopped = await request(5, accountA);
    host.render(null);
    await host.until(() => stopped.cancelled, "unmount aborts the pending request");
    provider.account(accountA);
    host.render(view());
    const reopened = await request(6, accountA);
    success(reopened, 0);
    await host.until(() => host.text().includes("$11.00"), "reopened component recovers the current wallet snapshot");
    respond(stopped.response, { errors: [{ message: "Late failure from stopped component" }] });
    await delay(10);
    assert.ok(host.text().includes("$11.00"));
    assert.ok(!host.text().includes(failure));

    account(null);
    assert.ok(host.text().includes("Connect wallet"));
    assert.ok(!host.text().includes("$11.00"), "disconnect clears wallet-scoped rows");
    account(accountB);
    const reconnected = await request(7, accountB);
    success(reconnected, 1);
    await host.until(() => host.text().includes("$22.00"), "reconnect reads the new wallet");
    assert.deepEqual(unexpected, []);
  } finally {
    host.render(null);
    provider.client.clear();
  }
}

async function mobileOrdersRecoverAfterAccountChangesAndRestart() {
  await recoverWalletRows("orders");
}
async function mobileTradesRecoverAfterAccountChangesAndRestart() {
  await recoverWalletRows("trades");
}
function completeMobileSpotTradeViewIncludesTheRealBalancePanel() {
  const provider = providers();
  try {
    for (const market of spotConfig.SPOT_MARKETS) {
      for (let restart = 0; restart < 2; restart++) {
        const html = renderToStaticMarkup(provider.wrap(createElement(spotView.MobileSpotTradeView, { market })));
        assert.match(html, /Spot Balance/);
        assert.match(html, />Connect<\/button>/);
        assert.match(html, /Hide Book/);
        assert.match(html, /min-height:480px/);
        assert.match(html, /min-height:360px/);
      }
    }
  } finally {
    provider.client.clear();
  }
}

test(mobileOrdersRecoverAfterAccountChangesAndRestart);
test(mobileTradesRecoverAfterAccountChangesAndRestart);
test(completeMobileSpotTradeViewIncludesTheRealBalancePanel);
