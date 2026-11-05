import assert from "node:assert/strict";
import { after, before, test } from "node:test";
import { createServer, type ServerResponse } from "node:http";
import { once } from "node:events";
import React, { act, createElement, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiProvider } from "wagmi";
import type { SpotClosedLeg, SpotHoldingsResponse } from "../src/core/spot/api";

// The committed in-memory store is served over real loopback HTTP. No module,
// hook, fetch implementation, wallet connector or clock is replaced.
const A = "0x00000000000000000000000000000000000000aa";
const B = "0x00000000000000000000000000000000000000bb";
const FOREIGN = "0x00000000000000000000000000000000000000cc";
let origin: string;
let domWindow: (Window & typeof globalThis) | undefined;
let holdings: SpotHoldingsResponse;
let createRoot: typeof import("react-dom/client").createRoot;
let flushSync: typeof import("react-dom").flushSync;
let renderToStaticMarkup: typeof import("react-dom/server").renderToStaticMarkup;
const history = new Map<
  string,
  { items: SpotClosedLeg[]; more: boolean; fail?: boolean }
>();
let pending: ServerResponse | null = null;
let holdPage = false;
let pageClosed = false;
let pageRequests = 0;
const requests: string[] = [];
const recordedStats = [
  {
    symbol: "PAX",
    last_price: 3,
    change_24h: -0.1,
    volume_24h: 900,
    high_24h: 4,
    low_24h: 2,
  },
  {
    symbol: "SID",
    last_price: 7,
    change_24h: 0.2,
    volume_24h: 100,
    high_24h: 8,
    low_24h: 6,
  },
];
const recordedBook = {
  bids: [
    ["90", "2"],
    ["100", "3"],
  ],
  asks: [
    ["120", "4"],
    ["110", "5"],
  ],
};
const server = createServer((req, res) => {
  const url = new URL(req.url!, "http://127.0.0.1");
  requests.push(url.pathname + url.search);
  res.setHeader("Content-Type", "application/json");
  if (url.pathname === "/fixtures/stats")
    return void res.end(JSON.stringify(recordedStats));
  if (url.pathname === "/fixtures/book")
    return void res.end(JSON.stringify(recordedBook));
  const address = url.pathname.split("/")[3];
  if (url.pathname.endsWith("/closed-legs")) {
    const entry = history.get(address);
    if (url.searchParams.has("afterId")) {
      pageRequests++;
      if (holdPage) {
        pending = res;
        res.once("close", () => {
          pageClosed = true;
        });
        return;
      }
    }
    if (entry?.fail) {
      res.statusCode = 400;
      res.end(JSON.stringify({ error: "recorded initial read failure" }));
      return;
    }
    const items = entry?.items ?? [];
    return void res.end(
      JSON.stringify({
        address,
        items,
        count: items.length,
        limit: 50,
        nextCursor: entry?.more ? { afterId: items.at(-1)!.id } : null,
      }),
    );
  }
  if (url.pathname.endsWith("/holdings"))
    return void res.end(JSON.stringify({ ...holdings, address }));
  if (url.pathname.endsWith("/positions"))
    return void res.end(JSON.stringify({ address, positions: [] }));
  if (url.pathname === `/v1/users/${address}/orders`) {
    assert.equal(req.method, "GET");
    assert.ok(
      ["open", "all"].includes(url.searchParams.get("status") ?? "all"),
    );
    return void res.end(JSON.stringify({ address, count: 0, items: [] }));
  }
  if (url.pathname.endsWith("/trades"))
    return void res.end(
      JSON.stringify({
        address,
        count: 0,
        limit: 50,
        items: [],
        nextCursor: null,
      }),
    );
  if (url.pathname.endsWith("/pnl/summary"))
    return void res.end(JSON.stringify({ address, realizedUsd: "0" }));
  res.statusCode = 404;
  res.end(JSON.stringify({ error: "fixture route not found" }));
});

before(async () => {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  origin = `http://127.0.0.1:${address.port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2/`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:");
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  process.env.NEXT_PUBLIC_MARKET_PROXY_HOST = origin;
  // Install browser APIs before importing providers, widgets or React DOM.
  const { JSDOM } = await import("../src/ui/atoms/test-support/dom");
  domWindow = new JSDOM("<!doctype html><html><body></body></html>", {
    url: origin,
    pretendToBeVisual: true,
  }).window;
  for (const [name, value] of Object.entries({
    window: domWindow,
    document: domWindow.document,
    navigator: domWindow.navigator,
    HTMLElement: domWindow.HTMLElement,
    HTMLFormElement: domWindow.HTMLFormElement,
    HTMLInputElement: domWindow.HTMLInputElement,
    DocumentFragment: domWindow.DocumentFragment,
    MutationObserver: domWindow.MutationObserver,
    Event: domWindow.Event,
    MouseEvent: domWindow.MouseEvent,
    WebSocket: domWindow.WebSocket,
    IS_REACT_ACT_ENVIRONMENT: true,
    SVGElement: domWindow.SVGElement,
    Element: domWindow.Element,
    Node: domWindow.Node,
    CustomEvent: domWindow.CustomEvent,
    getComputedStyle: domWindow.getComputedStyle.bind(domWindow),
  })) {
    Object.defineProperty(globalThis, name, { configurable: true, value });
  }
  ({ createRoot } = await import("react-dom/client"));
  ({ flushSync } = await import("react-dom"));
  ({ renderToStaticMarkup } = await import("react-dom/server"));
  const { SPOT_TOKENS } = await import("../src/core/spot/addresses");
  const row = (
    token: string,
    symbol: string,
    balance: string,
    decimals: number,
  ): SpotHoldingsResponse["erc20"][number] => ({
    token,
    symbol,
    balance,
    decimals,
    name: symbol,
    blockNumber: "1",
    holderCount: 1,
    type: "ERC-20",
  });
  holdings = {
    address: A,
    native: { symbol: "PAX", decimals: 18, balance: "0" },
    erc20: [
      row(FOREIGN, "SID", "999999999999999999999999", 18),
      row(SPOT_TOKENS.USDL.address.toUpperCase(), "USDL", "4000000", 6),
      row("0x00000000000000000000000000000000000000dd", "USDC", "900000000", 6),
    ],
  };
});
after(async () => {
  pending?.destroy();
  // Market subscriptions retain their socket for 15s after the last consumer.
  // Keep the real DOM and fixture origin alive through their normal teardown.
  await new Promise((resolve) => setTimeout(resolve, 15_000));
  domWindow?.close();
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((err) => (err ? reject(err) : resolve())),
  );
});

// Every mounted case uses one real React DOM root, with normal unmount cleanup.
function mount() {
  const tree = document.createElement("div");
  let root: ReturnType<typeof createRoot> | null = null;
  const render = (node: ReactNode) => {
    if (!root) {
      document.body.append(tree);
      root = createRoot(tree);
    }
    act(() => flushSync(() => root!.render(node)));
  };
  const stop = () => {
    if (root) act(() => root!.unmount());
    root = null;
    tree.remove();
  };
  return { tree, render, stop };
}
function textOf(node: Element): string {
  return node.textContent ?? "";
}
function nodes(node: Element): HTMLElement[] {
  return [node as HTMLElement, ...node.querySelectorAll<HTMLElement>("*")];
}
function click(tree: Element, label: string) {
  const button = nodes(tree).find(
    (node): node is HTMLButtonElement =>
      node.tagName === "BUTTON" && textOf(node) === label,
  );
  assert.ok(button, `button ${label} exists in ${textOf(tree)}`);
  assert.ok(!button.disabled, `${label} is enabled`);
  act(() => button.click());
}
function inputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(
    HTMLInputElement.prototype,
    "value",
  )!.set!;
  act(() => {
    setter.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}
async function until(predicate: () => boolean) {
  const deadline = Date.now() + 5000;
  while (!predicate()) {
    assert.ok(
      Date.now() < deadline,
      "real HTTP and React effects settle within five seconds",
    );
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10));
    });
  }
}
async function providers(connected = false) {
  const { wagmiConfig } = await import("../src/core/wagmi");
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const { spotTokenState } = await import("../src/core/spot/tokenState");
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity, gcTime: 0 },
    },
  });
  for (const market of SPOT_MARKETS) {
    const state = spotTokenState(false, [
      true,
      false,
      market.baseDecimals,
      0n,
      0n,
      0n,
    ]);
    client.setQueryData(["spot-token-state", market.baseAddress], state);
    client.setQueryData(
      [
        "spot",
        "tokenState",
        market.baseAddress.toLowerCase(),
        market.baseTokenMeta.isNative,
      ],
      state,
    );
  }
  wagmiConfig.setState((state) => ({
    ...state,
    current: null,
    status: "disconnected",
    connections: new Map(),
  }));
  function FixtureConnection({ children }: { children: ReactNode }) {
    React.useEffect(() => {
      if (!connected) return;
      const connector = wagmiConfig.connectors[0];
      assert.ok(connector);
      // Wagmi's real hydration clears pre-render connections when reconnect
      // is disabled. Publish the fixture store state after that lifecycle.
      wagmiConfig.setState((state) => ({
        ...state,
        chainId: 125,
        current: connector.uid,
        status: "connected",
        connections: new Map([
          [connector.uid, { accounts: [A], chainId: 125, connector }],
        ]),
      }));
    }, []);
    return children;
  }
  const wrap = (node: ReactNode) =>
    createElement(
      WagmiProvider,
      { config: wagmiConfig, reconnectOnMount: false },
      createElement(FixtureConnection, {
        children: createElement(QueryClientProvider, { client }, node),
      }),
    );
  return { wrap, client };
}

async function f023_liveMetricsSortBothDirectionsAndRenderTheSameRows() {
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const { parseSpotStats } =
    await import("../src/hooks/spot/useSpotMarketStream");
  const { sortSpotRows, StatRow } =
    await import("../src/widgets/spot/SpotMarketWatchlist");
  const payload: unknown = await (
    await fetch(`${origin}/fixtures/stats`)
  ).json();
  assert.ok(Array.isArray(payload));
  const rows = SPOT_MARKETS.map((market) => ({
    market,
    stats: parseSpotStats(
      payload.find((item) => item.symbol === market.symbol),
      market.symbol,
    ),
  }));
  for (const key of ["price", "change", "volume"] as const)
    for (const dir of ["asc", "desc"] as const) {
      const sorted = sortSpotRows(rows, key, dir);
      const ascending = key === "volume" ? ["SID", "PAX"] : ["PAX", "SID"];
      const expected = dir === "asc" ? ascending : [...ascending].reverse();
      assert.deepEqual(
        sorted.map((row) => row.market.symbol),
        expected,
      );
      const html = renderToStaticMarkup(
        createElement(
          React.Fragment,
          null,
          ...sorted.map((row) =>
            createElement(StatRow, {
              key: row.market.symbol,
              ...row,
              tokenState: undefined,
              tokenStateError: false,
              isSelected: false,
              isFavorite: false,
              onSelect() {},
              onToggleFavorite() {},
            }),
          ),
        ),
      );
      assert.ok(
        html.indexOf(`${expected[0]}/USDL`) <
          html.indexOf(`${expected[1]}/USDL`),
      );
      assert.ok(html.includes("+20.00%") && html.includes("-10.00%"));
      assert.deepEqual(
        sortSpotRows([...rows].reverse(), key, dir).map(
          (row) => row.market.symbol,
        ),
        expected,
      );
    }
  const updated = rows.map((row) => ({
    ...row,
    stats: row.stats && {
      ...row.stats,
      last_price: row.market.symbol === "PAX" ? 9 : 1,
    },
  }));
  assert.deepEqual(
    sortSpotRows(updated, "price", "desc").map((row) => row.market.symbol),
    ["PAX", "SID"],
  );
  for (const dir of ["asc", "desc"] as const)
    assert.equal(
      sortSpotRows([{ ...rows[0], stats: null }, rows[1]], "price", dir)[1]
        .stats,
      null,
    );
}

async function f014_foreignSymbolCannotSupplyTheSellBalanceOrPrecision() {
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const { spotApi } = await import("../src/core/spot/api");
  const { spotBaseHolding, spotSellExceedsBalance, SpotOrderForm } =
    await import("../src/widgets/spot/SpotOrderForm");
  const market = SPOT_MARKETS.find((m) => m.symbol === "SID")!;
  const received = await spotApi.getHoldings(A);
  const orders = await spotApi.getOrders(A, { status: "open", limit: 100 });
  assert.deepEqual(orders, { address: A, count: 0, items: [] });
  assert.ok(requests.includes(`/v1/users/${A}/orders?status=open&limit=100`));
  assert.equal(spotBaseHolding(market, received), null);
  assert.equal(
    spotSellExceedsBalance(
      1n,
      spotBaseHolding(market, received),
      market.baseDecimals,
    ),
    true,
  );
  const correct = {
    ...received.erc20[0],
    token: market.baseAddress.toUpperCase(),
    balance: "2000000",
    decimals: 18,
  };
  const exact = spotBaseHolding(market, {
    ...received,
    erc20: [...received.erc20, correct],
  });
  assert.deepEqual(exact, { raw: "2000000", decimals: market.baseDecimals });
  assert.equal(
    spotSellExceedsBalance(2000000n, exact, market.baseDecimals),
    false,
  );
  assert.equal(
    spotSellExceedsBalance(2000001n, exact, market.baseDecimals),
    true,
  );
  const { wrap, client } = await providers(true);
  const view = mount();
  try {
    view.render(wrap(createElement(SpotOrderForm, { market })));
    await until(
      () =>
        requests.some((path) => path.endsWith(`${A}/holdings`)) &&
        textOf(view.tree).includes("$4.00"),
    );
    click(view.tree, "Sell");
    click(view.tree, "Limit");
    const fields = [...view.tree.querySelectorAll("input")];
    assert.ok(fields.length >= 2);
    for (const field of fields.filter(
      (node) => node.type === "text" || node.type === "number",
    ))
      inputValue(field, "1");
    assert.ok(textOf(view.tree).includes("Insufficient SID Balance"));
  } finally {
    view.stop();
    client.clear();
  }
}

async function f015_stopLimitOffersOnlyTheLifetimeTheProducerEnforces() {
  const { SpotOrderForm, spotTimeInForceOptions } =
    await import("../src/widgets/spot/SpotOrderForm");
  const { spotOrderRail } = await import("../src/core/spot/slippage");
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const { wrap, client } = await providers();
  const view = mount();
  try {
    view.render(
      wrap(createElement(SpotOrderForm, { market: SPOT_MARKETS[1] })),
    );
    click(view.tree, "Limit");
    click(view.tree, "IOC");
    click(view.tree, "Stop-Limit");
    const labels = nodes(view.tree)
      .filter((node) => node.tagName === "BUTTON")
      .map(textOf);
    assert.ok(labels.includes("GTC"));
    assert.ok(!labels.includes("IOC"));
    assert.ok(!labels.includes("FOK"));
    assert.deepEqual(spotTimeInForceOptions("stop-limit"), ["GTC"]);
    for (const tif of spotTimeInForceOptions("stop-limit"))
      assert.equal(spotOrderRail("stop-limit", tif, false), "stop");
    click(view.tree, "Limit");
    const gtc = nodes(view.tree).find(
      (node) => node.tagName === "BUTTON" && textOf(node) === "GTC",
    );
    assert.ok(String(gtc?.className).includes("bg-primary/20"));
    assert.deepEqual(spotTimeInForceOptions("limit"), ["GTC", "IOC", "FOK"]);
  } finally {
    view.stop();
    client.clear();
  }
}

async function f016_singleStableFundingMatchesDisplayValidationAndPercentages() {
  const { SpotOrderForm, spotBuyFunding, spotPercentageAmount } =
    await import("../src/widgets/spot/SpotOrderForm");
  const { spotApi } = await import("../src/core/spot/api");
  const { selectStableForBuy } =
    await import("../src/core/spot/stableSelector");
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const funding = spotBuyFunding(await spotApi.getHoldings(A), 5000000n);
  assert.equal(funding.selection.stable.symbol, "USDL");
  assert.equal(funding.selection.available, 4000000n);
  assert.equal(funding.selection.sufficient, false);
  assert.deepEqual(
    funding.selection,
    selectStableForBuy({ amountUsd: 5000000n, balances: funding.balances }),
  );
  assert.equal(
    spotPercentageAmount({
      side: "BUY",
      sizeMode: "USD",
      balance: { raw: funding.selection.available.toString(), decimals: 6 },
      pct: 100,
      price: "1",
      baseDecimals: 6,
      quoteDecimals: 6,
    }),
    "4",
  );
  const { wrap, client } = await providers(true);
  const view = mount();
  try {
    view.render(
      wrap(createElement(SpotOrderForm, { market: SPOT_MARKETS[1] })),
    );
    await until(() => textOf(view.tree).includes("$4.00"));
    assert.ok(textOf(view.tree).includes("Spendable (USDL)"));
    assert.ok(
      textOf(view.tree).includes(
        "Each buy spends only USDL; balances of other tokens cannot be combined.",
      ),
    );
    click(view.tree, "Limit");
    click(view.tree, "USD");
    click(view.tree, "Max");
    const fields = [...view.tree.querySelectorAll("input")];
    assert.ok(fields.some((node) => node.value === "4"));
    const amountField = fields.find((node) => node.value === "4")!;
    inputValue(amountField, "5");
    const price = fields.find(
      (node) =>
        node !== amountField &&
        (node.type === "text" || node.type === "number"),
    )!;
    assert.ok(price);
    inputValue(price, "1");
    assert.ok(textOf(view.tree).includes("Insufficient USD Balance"));
  } finally {
    view.stop();
    client.clear();
  }
}

async function f024_midpointUsesBothBestSidesAndReportsMissingLiquidity() {
  const { parseVenueBook } = await import("../src/core/net/venueStream");
  const { buildSpotBook, SpotOrderbookView } =
    await import("../src/widgets/spot/SpotOrderbook");
  const received: unknown = await (
    await fetch(`${origin}/fixtures/book`)
  ).json();
  const decoded = parseVenueBook(received, Date.now());
  assert.ok(decoded);
  const book = buildSpotBook(decoded.asks, decoded.bids);
  assert.equal(book.bids[0].price, 100);
  assert.equal(book.asks[0].price, 110);
  const html = renderToStaticMarkup(
    createElement(SpotOrderbookView, { symbol: "SID", book }),
  );
  assert.ok(html.includes("$105.0000"));
  assert.ok(!html.includes("$100.0000"));
  assert.ok(html.includes("Mid Price"));
  for (const [asks, bids] of [
    [decoded.asks, []],
    [[], decoded.bids],
    [[], []],
  ]) {
    const oneSided = renderToStaticMarkup(
      createElement(SpotOrderbookView, {
        symbol: "SID",
        book: buildSpotBook(asks, bids),
      }),
    );
    assert.ok(!oneSided.includes("$"));
    assert.ok(oneSided.includes("—"));
    assert.ok(oneSided.includes("Mid Price"));
  }
}

async function f039_completeDesktopAndMobileConsumersRenderSuccessfully() {
  const { SpotPage } = await import("../src/pages/spot");
  const { MobileSpotTradeView } =
    await import("../src/widgets/mobile/MobileSpotTradeView");
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const { wrap, client } = await providers();
  try {
    for (const market of SPOT_MARKETS)
      for (let reopen = 0; reopen < 2; reopen++) {
        const desktop = renderToStaticMarkup(
          wrap(createElement(SpotPage, { symbol: market.symbol })),
        );
        assert.ok(desktop.includes(market.pair));
        assert.ok(desktop.includes("Connecting"));
        assert.ok(desktop.includes("No recent trades"));
        assert.ok(desktop.includes("Connect Wallet"));
        const mobile = renderToStaticMarkup(
          wrap(createElement(MobileSpotTradeView, { market })),
        );
        assert.ok(mobile.includes("Spot Balance"));
        assert.ok(mobile.includes("Hide Book"));
        assert.ok(!mobile.includes("NaN"));
      }
  } finally {
    client.clear();
  }
}

function leg(id: number, symbol: string): SpotClosedLeg {
  return {
    id,
    symbol,
    token: FOREIGN,
    qty: "1000000",
    decimals: 6,
    entryPrice: "1",
    exitPrice: "2",
    pnl: "1000000",
    pnlUsd: "1000000",
    pnlPct: "1",
    holdingSeconds: 60,
    openedAt: "2026-10-01T00:00:00Z",
    closedAt: "2026-10-01T00:01:00Z",
    closeTxHash: "",
    fullyClosed: true,
    name: symbol,
  };
}
async function f027_walletChangesAbortPagesAndReopenRecoversRetryableHistory() {
  const { ClosedLegsTab } =
    await import("../src/widgets/spot/SpotPositionsPanel");
  const view = mount();
  const render = (address: string) =>
    view.render(createElement(ClosedLegsTab, { address }));
  try {
    history.set(A, { items: [leg(1, "A-EXHAUSTED")], more: false });
    history.set(B, { items: [leg(2, "B-FIRST")], more: true });
    render(A);
    await until(() => textOf(view.tree).includes("A-EXHAUSTED"));
    assert.ok(textOf(view.tree).includes("End of history"));
    render(B);
    await until(() => textOf(view.tree).includes("B-FIRST"));
    assert.ok(!textOf(view.tree).includes("A-EXHAUSTED"));
    assert.ok(!textOf(view.tree).includes("End of history"));
    holdPage = true;
    pageClosed = false;
    const beforePage = pageRequests;
    click(view.tree, "Load more");
    await until(() => pageRequests === beforePage + 1);
    render(A);
    await until(() => textOf(view.tree).includes("A-EXHAUSTED") && pageClosed);
    assert.ok(pending);
    pending.end(
      JSON.stringify({
        address: B,
        items: [leg(3, "B-STALE-PAGE")],
        nextCursor: null,
      }),
    );
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 30));
    });
    assert.ok(!textOf(view.tree).includes("B-STALE-PAGE"));
    holdPage = false;
    view.stop();
    history.set(A, { items: [leg(4, "A-RECOVERED")], more: true, fail: true });
    render(A);
    await until(() => textOf(view.tree).includes("Retry closed legs"));
    assert.ok(!textOf(view.tree).includes("No closed legs yet"));
    history.get(A)!.fail = false;
    click(view.tree, "Retry closed legs");
    await until(() => textOf(view.tree).includes("A-RECOVERED"));
    assert.ok(!textOf(view.tree).includes("Retry closed legs"));
    view.stop();
    history.set(A, { items: [leg(5, "A-AFTER-REOPEN")], more: false });
    render(A);
    await until(() => textOf(view.tree).includes("A-AFTER-REOPEN"));
    assert.ok(!textOf(view.tree).includes("A-RECOVERED"));
    assert.ok(textOf(view.tree).includes("End of history"));
    render(B);
    await until(() => textOf(view.tree).includes("B-FIRST"));
    holdPage = true;
    pageClosed = false;
    const pages = pageRequests;
    click(view.tree, "Load more");
    await until(() => pageRequests === pages + 1);
    view.stop();
    await until(() => pageClosed);
  } finally {
    view.stop();
    holdPage = false;
    pending?.destroy();
  }
}

async function f026_tradeHighlightMemoryIsBoundedAndExpiresDuringContinuousFrames() {
  const { RecentTradeHighlights, RECENT_TRADE_KEY_LIMIT, useFlashIds } =
    await import("../src/widgets/spot/SpotTradesTape");
  const { parseSpotTrade } =
    await import("../src/hooks/spot/useSpotMarketStream");
  const decode = (id: number) => {
    const trade = parseSpotTrade({
      id: String(id),
      side: "BUY",
      price: "3",
      size: "2",
      trade_time: Date.now(),
    });
    assert.ok(trade);
    return trade;
  };
  const recent = new RecentTradeHighlights();
  for (let i = 0; i < 10000; i++) {
    recent.update([decode(i)], i);
    assert.ok(recent.seen.size <= RECENT_TRADE_KEY_LIMIT);
    assert.ok(recent.expires.size <= RECENT_TRADE_KEY_LIMIT);
  }
  assert.equal(recent.active(20000).size, 0);
  const first = decode(10001);
  let active = new Set<string>();
  function HighlightConsumer({
    trades,
  }: {
    trades: ReturnType<typeof decode>[];
  }) {
    active = useFlashIds(trades);
    return createElement("div", null, [...active].join(","));
  }
  const view = mount();
  try {
    let trades = [first];
    view.render(createElement(HighlightConsumer, { trades }));
    assert.ok(active.has(first.trade_id!));
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 200));
      });
      trades = [decode(10002 + i), ...trades];
      view.render(createElement(HighlightConsumer, { trades }));
    }
    assert.ok(
      !active.has(first.trade_id!),
      "the first highlight expires even while new frames keep arriving",
    );
    assert.ok(active.has(trades[0].trade_id!));
    await until(() => active.size === 0);
    view.render(createElement(HighlightConsumer, { trades: [...trades] }));
    assert.equal(active.size, 0, "repeated snapshots do not reflash seen keys");
  } finally {
    view.stop();
  }
}

for (const criterion of [
  f023_liveMetricsSortBothDirectionsAndRenderTheSameRows,
  f014_foreignSymbolCannotSupplyTheSellBalanceOrPrecision,
  f015_stopLimitOffersOnlyTheLifetimeTheProducerEnforces,
  f016_singleStableFundingMatchesDisplayValidationAndPercentages,
  f024_midpointUsesBothBestSidesAndReportsMissingLiquidity,
  f039_completeDesktopAndMobileConsumersRenderSuccessfully,
  f027_walletChangesAbortPagesAndReopenRecoversRetryableHistory,
  f026_tradeHighlightMemoryIsBoundedAndExpiresDuringContinuousFrames,
])
  test(criterion);
