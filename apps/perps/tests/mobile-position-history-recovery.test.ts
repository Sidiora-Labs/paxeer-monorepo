import assert from "node:assert/strict";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { after, before, test } from "node:test";
import { runInNewContext } from "node:vm";
import { isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { PositionTpSlTarget } from "../src/widgets/trading/PerpsPositionsPanel";
import type { PositionsPanelData, PositionsPanelSnapshot } from "../src/widgets/trading/positionsPanelData";

const walletA = "0x1111111111111111111111111111111111111111";
const walletB = "0x2222222222222222222222222222222222222222";
const operations = ["GetAllOrdersV2", "GetTPSLOrders", "GetClosedPositions", "GetTrades", "GetLiquidations"] as const;
type Operation = typeof operations[number];

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => { resolve = done; });
  return { promise, resolve };
}

// Committed indexer transcript fields consumed by the production queries.
function transcript(address: string, id: string, operation: Operation) {
  const position = {
    positionId: id, userAddress: address, marketId: 1, isLong: true,
    sizeUsd: "1000000000000000000000", leverage: "5000000000000000000",
    entryPrice: "2000000000000000000000", collateralToken: "0x0000000000000000000000000000000000000001",
    collateralAmount: "200000000", collateralUsd: "200000000000000000000",
    status: "closed", realizedPnl: "10000000000000000000", exitPrice: "2020000000000000000000",
    openedAt: "2024-01-01T00:00:00Z", closedAt: "2024-01-02T00:00:00Z", closeTxHash: "0x1234",
    market: { symbol: "ETH" },
  };
  const order = {
    orderId: id, positionId: id, userAddress: address, marketId: 1, isLong: true,
    orderType: 2, triggerPrice: "2200000000000000000000", sizeUsd: position.sizeUsd,
    status: "active", failureReason: null, resolvedTxHash: null,
  };
  switch (operation) {
    case "GetAllOrdersV2": return { orders: [order] };
    case "GetTPSLOrders": return { takeProfits: [order], stopLosses: [{ ...order, orderType: 3, triggerPrice: "1900000000000000000000" }] };
    case "GetClosedPositions": return { positions: [position] };
    case "GetTrades": return { trades: [{ id, positionId: id, userAddress: address, marketId: 1, tradeType: "close", isLong: true, sizeUsd: position.sizeUsd, price: position.exitPrice, realizedPnl: position.realizedPnl, feeUsd: "1000000000000000000", blockNumber: "10", txHash: "0x1234", blockTimestamp: position.closedAt }] };
    case "GetLiquidations": return { liquidations: [{ id, positionId: id, marketId: 1, price: position.exitPrice, penalty: "1000000000000000000", keeper: walletB, txHash: "0x1234", blockTimestamp: position.closedAt }] };
  }
}

interface ResponsePlan {
  id: string;
  fail: boolean;
  release: Promise<void>;
  received: () => void;
}
const plans = new Map<string, ResponsePlan[]>();
const requests: Array<{ operation: string; address: string }> = [];
const unexpected: string[] = [];
let scriptMode: "error" | "missing" | "success" = "success";
let origin: string;
let panel: typeof import("../src/widgets/trading/PerpsPositionsPanel");
let dataModule: typeof import("../src/widgets/trading/positionsPanelData");
let loader: typeof import("../src/widgets/trading/tradingViewLoader");
let pnl: typeof import("../src/core/perps/pnlHelpers");
let SetTpSlModal: typeof import("../src/widgets/trading/modals/SetTpSlModal").SetTpSlModal;

const server = createServer((request, response) => {
  void (async () => {
    if (request.url === "/static/charting_library/charting_library.standalone.js") {
      if (scriptMode === "error") response.writeHead(503).end("Download unavailable");
      else response.writeHead(200, { "Content-Type": "application/javascript" }).end(
        scriptMode === "success" ? "window.TradingView = { widget: function widget() {} };" : "void 0;",
      );
      return;
    }
    let body = "";
    for await (const chunk of request) body += chunk;
    const { query, variables } = JSON.parse(body);
    const operation = /query\s+(\w+)/.exec(query)?.[1] ?? "unknown";
    const address = variables.userAddress;
    requests.push({ operation, address });
    const plan = plans.get(`${address}:${operation}`)?.shift();
    if (!plan || request.url !== "/graphql" || request.method !== "POST") {
      unexpected.push(`${request.method} ${request.url} ${address}:${operation}`);
      response.writeHead(400).end();
      return;
    }
    plan.received();
    await plan.release;
    response.writeHead(200, { "Content-Type": "application/json" }).end(JSON.stringify(
      plan.fail ? { errors: [{ message: `${operation} unavailable` }] } : { data: transcript(address, plan.id, operation as Operation) },
    ));
  })().catch((error: unknown) => response.destroy(error instanceof Error ? error : new Error(String(error))));
});

before(async () => {
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:") + "/socket";
  dataModule = await import("../src/widgets/trading/positionsPanelData");
  panel = await import("../src/widgets/trading/PerpsPositionsPanel");
  loader = await import("../src/widgets/trading/tradingViewLoader");
  pnl = await import("../src/core/perps/pnlHelpers");
  ({ SetTpSlModal } = await import("../src/widgets/trading/modals/SetTpSlModal"));
});
after(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  assert.deepEqual(unexpected, []);
  assert.ok([...plans.values()].every((queue) => queue.length === 0));
});

function batch(address: string, id: string, options: { hold?: boolean; fail?: boolean } = {}) {
  const release = deferred();
  const received = operations.map((operation) => {
    const receipt = deferred();
    const key = `${address}:${operation}`;
    const queue = plans.get(key) ?? [];
    queue.push({ id, fail: options.fail ?? false, release: release.promise, received: receipt.resolve });
    plans.set(key, queue);
    return receipt.promise;
  });
  if (!options.hold) release.resolve();
  return { received: Promise.all(received), release: release.resolve };
}

function refresh(data: PositionsPanelData) {
  return Promise.all([data.refreshLive(), data.refreshHistory()]);
}

function assertEmpty(snapshot: PositionsPanelSnapshot) {
  assert.deepEqual(snapshot, { allOrders: [], closedPositions: [], trades: [], liquidations: [], tpslMap: {}, errors: {}, loading: false });
}

function assertRows(snapshot: PositionsPanelSnapshot, id: string) {
  assert.deepEqual(snapshot.allOrders.map((row) => row.orderId), [id]);
  assert.deepEqual(snapshot.closedPositions.map((row) => row.positionId), [id]);
  assert.deepEqual(snapshot.trades.map((row) => row.id), [id]);
  assert.deepEqual(snapshot.liquidations.map((row) => row.id), [id]);
  assert.deepEqual(snapshot.tpslMap, { [id]: { tp: "2200000000000000000000", sl: "1900000000000000000000" } });
  assert.equal(snapshot.loading, false);
}

function elements(node: ReactNode): ReactElement<Record<string, unknown>>[] {
  if (Array.isArray(node)) return node.flatMap(elements);
  if (!isValidElement<Record<string, unknown>>(node)) return [];
  return [node, ...elements(node.props.children as ReactNode)];
}

async function f034MobilePositionOpensExistingTpSlModal() {
  const recorded = transcript(walletA, "42", "GetClosedPositions");
  assert.ok(recorded.positions);
  const pos = recorded.positions[0];
  assert.ok(pos);
  const parsed = pnl.parsePositionRow(pos, "ETH");
  const markPrice = 2100;
  const pnlData = pnl.computePositionPnl(parsed, pnl.markPriceWad(markPrice), [{ lowerBoundUsd: 0n, maxLeverage: 10n * pnl.WAD, maintenanceBps: 50n, deductionUsd: 0n }]);
  const selection: { target: PositionTpSlTarget | null } = { target: null };
  const card = panel.MobilePositionCardContent({
    pos, parsed, pnlData, markPrice, getMarketSymbol: () => "ETH", isClosing: false,
    onClose: () => assert.fail("TP/SL must not close the position"),
    onSetTpsl: (target) => { selection.target = target; },
  });
  const button = elements(card).find((node) => node.type === "button" && node.props.children === "TP / SL");
  assert.ok(button);
  assert.equal(button.props.disabled, undefined);
  assert.equal(typeof button.props.onClick, "function");
  if (typeof button.props.onClick === "function") button.props.onClick();
  assert.deepEqual(selection.target, { positionId: "42", entryPrice: 2000, isLong: true, markPrice: 2100, size: 1000 });
  const modal = panel.PositionTpSlModal({ target: selection.target, onClose: () => { selection.target = null; }, onSuccess: () => {} });
  assert.ok(modal);
  assert.equal(modal.type, SetTpSlModal);
  assert.equal(modal.props.positionId, "42");
  assert.equal(modal.props.currentEntryPrice, 2000);
  assert.equal(modal.props.currentSize, 1000);
  assert.equal(modal.props.isLong, true);
  assert.equal(modal.props.markPrice, 2100);
  modal.props.onClose();
  assert.equal(panel.PositionTpSlModal({ target: selection.target, onClose: () => {}, onSuccess: () => {} }), null);
}

async function f035WalletSwitchRejectsLateResults() {
  const a = new dataModule.PositionsPanelData(walletA);
  a.open();
  batch(walletA, "11");
  await refresh(a);
  assertRows(a.getSnapshot(), "11");
  const late = batch(walletA, "12", { hold: true });
  const pending = refresh(a);
  await late.received;
  a.close();
  assertEmpty(a.getSnapshot());
  const b = new dataModule.PositionsPanelData(walletB);
  b.open();
  assertEmpty(b.getSnapshot());
  batch(walletB, "21");
  await refresh(b);
  assertRows(b.getSnapshot(), "21");
  late.release();
  await pending;
  assertEmpty(a.getSnapshot());
  assertRows(b.getSnapshot(), "21");
  b.close();
  assertEmpty(b.getSnapshot());
  const disconnected = new dataModule.PositionsPanelData(undefined);
  disconnected.open();
  const count = requests.length;
  await refresh(disconnected);
  assert.equal(requests.length, count);
  assertEmpty(disconnected.getSnapshot());
  disconnected.close();
}

async function f035StopReopenRejectsSupersededResults() {
  for (const fail of [false, true]) {
    const data = new dataModule.PositionsPanelData(walletA);
    data.open();
    const old = batch(walletA, "31", { hold: true, fail });
    const pending = refresh(data);
    await old.received;
    data.close();
    assertEmpty(data.getSnapshot());
    data.open();
    batch(walletA, "32");
    await refresh(data);
    assertRows(data.getSnapshot(), "32");
    old.release();
    await pending;
    assertRows(data.getSnapshot(), "32");
    assert.deepEqual(data.getSnapshot().errors, {});
    const superseded = batch(walletA, "33", { hold: true, fail });
    const earlier = refresh(data);
    await superseded.received;
    batch(walletA, "34");
    await refresh(data);
    superseded.release();
    await earlier;
    assertRows(data.getSnapshot(), "34");
    assert.deepEqual(data.getSnapshot().errors, {});
    data.close();
  }
}

async function f035FailuresAreVisibleAndRetryRecovers() {
  const data = new dataModule.PositionsPanelData(walletB);
  data.open();
  batch(walletB, "41");
  await refresh(data);
  batch(walletB, "42", { fail: true });
  await refresh(data);
  assertRows(data.getSnapshot(), "41");
  const errors = data.getSnapshot().errors;
  assert.deepEqual(Object.keys(errors).sort(), ["liquidations", "orders", "positions", "tpsl", "trades"]);
  let retrying: ReturnType<typeof refresh> | undefined;
  for (const [key, error] of Object.entries(errors)) {
    const alert = panel.PanelDataFailure({ label: key, error, onRetry: () => { retrying = refresh(data); } });
    assert.ok(alert);
    const markup = renderToStaticMarkup(alert);
    assert.match(markup, /role="alert"/);
    assert.match(markup, /unavailable/);
    assert.match(markup, /Previously loaded rows may be stale/);
    assert.match(markup, /Retry/);
  }
  batch(walletB, "43");
  const alert = panel.PanelDataFailure({ label: "Orders", error: errors.orders, onRetry: () => { retrying = refresh(data); } });
  const retry = elements(alert).find((node) => node.type === "button");
  assert.ok(retry);
  assert.equal(typeof retry.props.onClick, "function");
  if (typeof retry.props.onClick === "function") retry.props.onClick();
  assert.ok(retrying);
  await retrying;
  assertRows(data.getSnapshot(), "43");
  assert.deepEqual(data.getSnapshot().errors, {});
  assert.equal(panel.PanelDataFailure({ label: "Orders", error: data.getSnapshot().errors.orders, onRetry: () => {} }), null);
  data.close();
}

// In-memory browser resource store. Script bytes come from the loopback server
// and execute in an isolated JS realm; the production loader owns all listeners.
class ScriptResource extends EventTarget {
  src = "";
  async = false;
  defer = false;
  dataset: Record<string, string> = {};
  readonly listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  constructor(private readonly store: ScriptDocument) { super(); }
  override addEventListener(type: string, listener: EventListenerOrEventListenerObject | null, options?: boolean | AddEventListenerOptions) {
    if (listener) {
      const listeners = this.listeners.get(type) ?? new Set();
      listeners.add(listener);
      this.listeners.set(type, listeners);
    }
    super.addEventListener(type, listener, options);
  }
  override removeEventListener(type: string, listener: EventListenerOrEventListenerObject | null, options?: boolean | EventListenerOptions) {
    if (listener) this.listeners.get(type)?.delete(listener);
    super.removeEventListener(type, listener, options);
  }
  remove() { this.store.scripts.delete(this); }
}

class ScriptDocument {
  readonly scripts = new Set<ScriptResource>();
  readonly downloads: Promise<void>[] = [];
  readonly window: { TradingView?: { widget: unknown } } = {};
  readonly head = {
    appendChild: (script: ScriptResource) => {
      this.scripts.add(script);
      const download = (async () => {
        const response = await fetch(new URL(script.src, origin));
        if (!response.ok) { script.dispatchEvent(new Event("error")); return; }
        runInNewContext(await response.text(), { window: this.window });
        script.dispatchEvent(new Event("load"));
      })();
      this.downloads.push(download);
      return script;
    },
  };
  createElement(tag: string) { assert.equal(tag, "script"); return new ScriptResource(this); }
  querySelector(selector: string) {
    assert.equal(selector, 'script[data-tradingview-loader="1"]');
    return [...this.scripts].find((script) => script.dataset.tradingviewLoader === "1") ?? null;
  }
}

async function f018DownloadAndMissingGlobalRecoverOnReopen() {
  const document = new ScriptDocument();
  const previous = new Map(["window", "document"].map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  Object.defineProperty(globalThis, "window", { configurable: true, value: document.window });
  Object.defineProperty(globalThis, "document", { configurable: true, value: document });
  try {
    for (const mode of ["error", "missing"] as const) {
      scriptMode = mode;
      const first = loader.loadTradingView();
      assert.equal(loader.loadTradingView(), first, "concurrent chart mounts share one request");
      const failed = [...document.scripts][0];
      assert.ok(failed);
      await assert.rejects(first, mode === "error" ? /Failed to load/ : /global missing/);
      await Promise.all(document.downloads);
      assert.equal(document.scripts.size, 0, "failed script must be removed before reopening the chart");
      assert.equal(failed.listeners.get("load")?.size, 0);
      assert.equal(failed.listeners.get("error")?.size, 0);
      failed.dispatchEvent(new Event("error"));
      failed.dispatchEvent(new Event("load"));
    }
    scriptMode = "success";
    // Stop the failed chart and reopen: a fresh script download must settle.
    const retry = loader.loadTradingView();
    assert.equal(loader.loadTradingView(), retry);
    assert.equal(document.scripts.size, 1);
    const replacement = [...document.scripts][0];
    assert.ok(replacement);
    await retry;
    await Promise.all(document.downloads);
    assert.equal(typeof document.window.TradingView?.widget, "function");
    assert.equal(replacement.listeners.get("load")?.size, 0);
    assert.equal(replacement.listeners.get("error")?.size, 0);
    assert.equal(document.downloads.length, 3);
    await loader.loadTradingView();
    assert.equal(document.downloads.length, 3);
  } finally {
    for (const [key, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
}

test(f034MobilePositionOpensExistingTpSlModal);
test(f035WalletSwitchRejectsLateResults);
test(f035StopReopenRejectsSupersededResults);
test(f035FailuresAreVisibleAndRetryRecovers);
test(f018DownloadAndMissingGlobalRecoverOnReopen);
