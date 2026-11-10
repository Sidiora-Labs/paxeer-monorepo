/**
 * Proves: widgets/{spot/SpotOrderForm.SpotOrderForm,spot/SpotMarketWatchlist.SpotMarketWatchlist,trading/PerpsOrderForm.PerpsOrderForm,markets/useMarketsRows.useMarketsRows,portfolio/PortfolioFeeTierPanel.PortfolioFeeTierPanel} -> exported TypeScript widget modules and production Next loaders -> pages/spot.SpotPage, page-modules/{spot,perps}, mobile/MobileSpotTradeView, spot/SpotPairHeader and rendered markets/portfolio consumers.
 *
 * Repairs: pages/spot, page-modules/spot, page-modules/perps and SpotPairHeader
 * silently selected a different asset for unknown input; SpotPairHeader emitted
 * USD instead of USDL. SpotOrderForm accepted mismatched registry metadata and
 * malformed token states; SpotMarketWatchlist treated malformed states as live;
 * PerpsOrderForm, buildMarketsRows and PortfolioFeeTierPanel accepted malformed
 * producer results. The real consumers now refuse these at their boundaries.
 * PerpsOrderForm also rendered a zero-width Radix slider while limits loaded,
 * producing NaN CSS; it now reports loading or fixed leverage explicitly.
 * The touched page modules and MobileSpotTradeView need explicit React imports
 * under the declared tsx runner's JSX transform; those imports are repaired.
 *
 * No persistent widget state is claimed: fresh renders consume each result twice,
 * then consume a changed result. Real React, Next loaders, barrels, ABI decoders,
 * catalog mapping, query cache and wallet providers are used without substitution.
 * The catalog hook's existing server limitation is asserted explicitly below.
 */
import "./fixtures/react-runtime";
import assert from "node:assert/strict";
import { test } from "node:test";
import { createRequire } from "node:module";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { act, createElement, Fragment, isValidElement, type ReactElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { WagmiProvider } from "wagmi";
import Router, { type NextRouter } from "next/router";
import { RouterContext } from "next/dist/shared/lib/router-context.shared-runtime";
import { decodeFunctionResult, encodeFunctionResult } from "viem";
import { spotWidgetElements } from "./fixtures/spot-widget-elements";
import { spotTokenState, type VaultTokenInfo } from "../src/core/spot/tokenState";
import { feeTierView } from "../src/core/perps/feeTier";
import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";

async function renderer() {
  const { wagmiConfig } = await import("../src/core/wagmi");
  const client = new QueryClient();
  function render(node: ReactNode): string {
    return renderToStaticMarkup(
      createElement(WagmiProvider, { config: wagmiConfig, reconnectOnMount: false },
        createElement(QueryClientProvider, { client },
          createElement(RouterContext.Provider, { value: Router as NextRouter }, node))),
    );
  }
  return { client, render };
}

// Keep the actual elements emitted by a consumer, including their props and
// component identity. Unrelated sibling widgets are not substituted or rendered.
function selectedElements(node: ReactNode, types: readonly unknown[]): ReactElement[] {
  if (Array.isArray(node)) return node.flatMap((child) => selectedElements(child, types));
  if (!isValidElement<{ children?: ReactNode }>(node)) return [];
  if (types.includes(node.type)) return [node];
  return selectedElements(node.props.children, types);
}

function noSelection(symbol: string): never {
  throw new Error(`Unexpected selection during rendering: ${symbol}`);
}

async function spotWidgetsReachDesktopMobileAndRouteConsumers() {
  const page = await import("../src/pages/spot");
  const widgets = await import("../src/widgets/spot");
  const direct = await import("../src/widgets/spot/SpotOrderForm");
  const desktop = await import("../src/page-modules/spot");
  const mobile = await import("../src/widgets/mobile/MobileSpotTradeView");
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const { client, render } = await renderer();
  const previousGate = process.env.NEXT_PUBLIC_RELEASE_SPOT_OPEN;
  process.env.NEXT_PUBLIC_RELEASE_SPOT_OPEN = "1";
  try {
    assert.equal(widgets.SpotOrderForm, direct.SpotOrderForm);
    for (const consumer of [desktop, mobile]) {
      // These are the very functions passed to next/dynamic by each consumer.
      const loaded = await consumer.loadSpotOrderForm();
      assert.equal(loaded.default, widgets.SpotOrderForm);
      assert.equal((await consumer.loadSpotOrderForm()).default, direct.SpotOrderForm);
    }
    const pageHtml = render(createElement(page.SpotPage, { symbol: "SID" }));
    assert.ok(pageHtml.includes("SID/USDL"));
    assert.ok(pageHtml.includes("Connect Wallet"));
    const info: VaultTokenInfo = [true, false, 6, 10_000_000n, 12_000_000n, 2_000_000n];
    for (const market of SPOT_MARKETS) {
      const route = await page.getServerSideProps({ query: { market: market.symbol.toLowerCase() } });
      assert.ok("props" in route);
      assert.equal(route.props.symbol, market.symbol);
      const tree = page.SpotPage(route.props);
      const composed = spotWidgetElements(tree);
      const form = composed.find((node) => node.type === widgets.SpotOrderForm);
      const watchlist = composed.find((node) => node.type === widgets.SpotMarketWatchlist);
      assert.ok(form);
      assert.ok(watchlist);
      assert.equal(form.props.market, market, "route forwards the registry's actual market");
      assert.equal(watchlist.props.selectedSymbol, market.symbol);
      for (const paused of [false, true, false]) {
        const produced = spotTokenState(paused, info);
        assert.equal(direct.spotSubmitBlocked(produced), paused ? "TRADE_MARKET_HALTED" : null);
        for (const entry of SPOT_MARKETS) {
          client.setQueryData(["spot-token-state", entry.baseAddress],
            entry === market ? produced : spotTokenState(false, info));
        }
        for (let pass = 0; pass < 2; pass++) {
          const html = render(createElement(page.SpotPage, route.props));
          assert.equal((html.match(/>Paused<\/span>/g) ?? []).length, paused ? 1 : 0);
          assert.ok(html.includes(market.pair));
          assert.ok(html.includes("Checking token trading status"));
          assert.ok(html.includes("Connect Wallet"));
        }
      }
      for (const consumer of [desktop, mobile]) {
        const LoadedForm = (await consumer.loadSpotOrderForm()).default;
        for (let pass = 0; pass < 2; pass++) {
          const html = render(createElement(LoadedForm, { market, compact: consumer === mobile }));
          assert.ok(html.includes(market.symbol));
          assert.ok(html.includes("Connect Wallet"));
        }
        assert.throws(() => render(createElement(LoadedForm, {
          market: { ...market, baseDecimals: market.baseDecimals + 1 },
        })), /Invalid spot order market/);
      }
      // Execute the mobile consumer itself with React's real hook dispatcher,
      // then render its actual toolbar and Next form boundary. The unrelated
      // MobileSpotBalancePanel.tsx::MobileSpotBalancePanel also lacks a React
      // import under tsx; no replacement balance panel is introduced here.
      function MobileConsumer(): ReactElement {
        const mobileTree = mobile.MobileSpotTradeView({ market });
        const formNode = spotWidgetElements(mobileTree)
          .find((node) => node.type === mobile.SpotOrderFormWidget);
        assert.ok(formNode);
        assert.equal(formNode.props.market, market);
        assert.equal(formNode.props.compact, true);
        return createElement(Fragment, null, ...selectedElements(mobileTree, [
          "button", mobile.SpotOrderFormWidget,
        ]));
      }
      for (let pass = 0; pass < 2; pass++) {
        assert.ok(render(createElement(MobileConsumer)).includes("Hide Book"));
      }
      const live = spotTokenState(false, info);
      const malformed = { ...live, reason: "TokenPaused" } as typeof live;
      assert.equal(direct.spotSubmitBlocked(malformed), "TokenStateUnavailable");
      client.setQueryData(["spot-token-state", market.baseAddress], malformed);
      assert.throws(() => render(createElement(page.SpotPage, route.props)), /Invalid spot token state/);
      client.setQueryData(["spot-token-state", market.baseAddress], live);
      assert.ok(render(createElement(page.SpotPage, route.props)).includes(market.pair), "consumer recovers from a refused result");
    }
    for (const malformed of ["NONE", "", ["PAX", "SID"], { symbol: "PAX" }]) {
      assert.deepEqual(await page.getServerSideProps({ query: { market: malformed } } as Parameters<typeof page.getServerSideProps>[0]),
        { notFound: true });
    }
    assert.throws(() => page.SpotPage({ symbol: "NONE" }), /Unknown spot market/);
  } finally {
    client.clear();
    if (previousGate === undefined) delete process.env.NEXT_PUBLIC_RELEASE_SPOT_OPEN;
    else process.env.NEXT_PUBLIC_RELEASE_SPOT_OPEN = previousGate;
  }
}

async function pairHeaderConsumesWatchlistSelectionInSettlementUnits() {
  const { SpotPairHeader, SpotMarketWatchlist } = await import("../src/widgets/spot");
  const { SPOT_MARKETS } = await import("../src/core/spot/config");
  const { client, render } = await renderer();
  try {
    for (const market of SPOT_MARKETS) {
      const observed: string[] = [];
      function HeaderConsumer(): ReactElement {
        const tree = SpotPairHeader({ selectedPair: market.pair, onPairChange: (pair) => observed.push(pair) });
        const watchlist = spotWidgetElements(tree).find((node) => node.type === SpotMarketWatchlist);
        assert.ok(watchlist, "header consumes the exported watchlist");
        assert.equal(watchlist.props.selectedSymbol, market.symbol);
        if (observed.length === 0) (watchlist.props.onSelect as (symbol: string) => void)(market.symbol);
        return tree;
      }
      for (let pass = 0; pass < 2; pass++) {
        observed.length = 0;
        const html = render(createElement(HeaderConsumer));
        assert.deepEqual(observed, [market.pair]);
        assert.ok(html.includes(market.pair));
      }
    }
    for (const selectedPair of ["NONE", "PAX/OTHER", "PAX/USDL/OTHER"]) {
      assert.throws(() => render(createElement(SpotPairHeader, { selectedPair, onPairChange: noSelection })), /Unknown spot market/);
    }
  } finally { client.clear(); }
}

async function perpsLoaderDeliversTheRealOrderFormAndRefusesMalformedMarket() {
  const consumer = await import("../src/page-modules/perps");
  const producer = await import("../src/widgets/trading/PerpsOrderForm");
  const { PERPS_MARKETS } = await import("../src/core/perps/config");
  const { client, render } = await renderer();
  try {
    for (const market of PERPS_MARKETS.slice(0, 2)) {
      for (let pass = 0; pass < 2; pass++) {
        const loaded = await consumer.loadPerpsOrderForm();
        assert.equal(loaded.default, producer.PerpsOrderForm);
        const html = render(createElement(loaded.default, { market }));
        assert.ok(html.includes("Connect Wallet"));
        assert.ok(html.includes("Leverage"));
        assert.ok(html.includes(`aria-label="${market.symbol} perpetual order"`));
        assert.ok(!html.includes("NaN"));
        assert.ok(html.includes("Loading leverage limits"));
        assert.ok(!html.includes('role="slider"'));
      }
      for (const malformed of [{ ...market, marketId: NaN }, { ...market, marketId: -1 }, { ...market, symbol: "" }]) {
        const loaded = await consumer.loadPerpsOrderForm();
        assert.throws(() => render(createElement(loaded.default, { market: malformed })), /Invalid perpetual order market/);
      }
    }
  } finally { client.clear(); }
}

async function marketsRowsCrossExportsIntoTheRealTableWithoutStaleResults() {
  const producer = await import("../src/widgets/markets/useMarketsRows");
  const consumer = await import("../src/widgets/markets");
  const { catalogFromGateway } = await import("../src/core/perps/marketCatalog");
  const { normalizeMarket, normalizeMarketsFrame } = await import("../src/hooks/perps/useMarketsStream");
  const { client, render } = await renderer();
  const now = Date.parse("2026-10-27T12:00:00Z");
  const row: GatewayMarketRow = {
    marketId: 18, symbol: "DOGE", name: "Dogecoin", oracleSymbol: "DOGE",
    szDecimals: 3, pxDecimals: 4, tickSize: "100000000000000", lotSize: "1000000000000000",
    tierClass: 0, maxLeverage: "20", enabled: true, createdAt: new Date(now).toISOString(),
    latestPrice: null, fundingRate: null,
  };
  function table(rows: Parameters<typeof consumer.MarketsTable>[0]["rows"]): ReactElement {
    return createElement(consumer.MarketsTable, {
      rows, loading: false, favorites: new Set<string>(), onToggleFavorite: noSelection,
      sortKey: "symbol", sortDir: "asc", onToggleSort: noSelection,
    });
  }
  try {
    assert.equal(consumer.useMarketsRows, producer.useMarketsRows);
    assert.equal(consumer.filterAndSort, producer.filterAndSort);
    // apps/perps/src/hooks/perps/usePerpsMarkets.ts::usePerpsMarkets has
    // effect-only loading, so SSR must expose no catalog and no live rows.
    // Expected populated server catalogs require an upstream hook change.
    function InitialRowsConsumer(): ReactElement {
      const result = consumer.useMarketsRows();
      assert.deepEqual(result.rows, []);
      assert.equal(result.bySymbol.size, 0);
      assert.equal(result.hasAnyLive, false);
      return table(result.rows);
    }
    for (let pass = 0; pass < 2; pass++) {
      assert.ok(render(createElement(InitialRowsConsumer)).includes("No markets"));
    }
    const catalog = catalogFromGateway({ markets: [row] }, now);
    const markets = catalog.tradeable.map((entry) => {
      assert.notEqual(entry.maxLeverage, null);
      return { ...entry, baseToken: entry.symbol, maxLeverage: entry.maxLeverage! };
    });
    for (const price of [2, 3, 2]) {
      const snapshot = normalizeMarket({ symbol: "DOGE", price, change_24h: 5, volume_24h: 1234, open_interest: 100 });
      assert.ok(snapshot);
      for (let pass = 0; pass < 2; pass++) {
        const produced = producer.buildMarketsRows(markets, [snapshot]);
        const consumed = consumer.filterAndSort(produced, { category: "new", search: "doge" });
        assert.equal(consumed[0].snapshot, snapshot);
        assert.equal(consumed[0].price, price);
        const html = render(table(consumed));
        assert.ok(html.includes("DOGE"));
        assert.ok(html.includes(`$${price.toFixed(2)}`));
        assert.deepEqual(consumer.filterAndSort(produced, { search: "UNKNOWN" }), []);
      }
      assert.throws(() => producer.buildMarketsRows(markets, [{ ...snapshot, price: NaN }]), /Invalid market snapshot/);
      assert.throws(() => producer.buildMarketsRows(markets, [{ ...snapshot, volume_24h: -1 }]), /Invalid market snapshot/);
    }
    for (const field of ["price", "change_24h", "volume_24h", "open_interest", "volume_7d", "volume_30d",
      "trades_count", "last_updated", "price_updated_at", "price_24h", "high_24h", "low_24h", "amount_24h",
      "reported_volume_24h", "reported_open_interest"]) {
      for (const malformed of ["bad", "", " ", "0x10", "1e309", false, [], {}, NaN, Infinity,
        ...(["price", "change_24h", "last_updated"].includes(field) ? [] : [null])]) {
        const raw = { symbol: "DOGE", price: 2, [field]: malformed };
        assert.equal(normalizeMarket(raw), null, `${field} must refuse ${String(malformed)}`);
        const frame = normalizeMarketsFrame([raw]);
        assert.deepEqual(frame, []);
        const sibling = normalizeMarketsFrame([raw, { symbol: "SIBLING", price: 4 }]);
        assert.equal(sibling.length, 1);
        assert.equal(sibling[0].symbol, "SIBLING");
        const rows = producer.buildMarketsRows(markets, frame);
        assert.equal(rows[0].hasLiveData, false);
        assert.equal(rows[0].snapshot, undefined);
        const html = render(table(rows));
        assert.ok(html.includes("DOGE"));
        assert.ok(!html.includes("$0.00"), "refused input is unavailable, never a live zero price");
      }
    }
    for (const field of ["price", "change_24h", "last_updated"]) {
      const allowed = normalizeMarket({ symbol: "DOGE", price: 2, [field]: null, volume_24h: 1234 });
      assert.ok(allowed);
      assert.equal(allowed[field as keyof typeof allowed], null);
      const live = producer.buildMarketsRows(markets, [allowed]);
      assert.equal(live[0].hasLiveData, true);
      assert.equal(live[0].volume24h, 1234);
      assert.ok(render(table(live)).includes("DOGE"));
    }
    for (const field of ["price", "volume_24h", "open_interest", "volume_7d", "volume_30d", "trades_count", "last_updated", "price_updated_at", "price_24h", "high_24h", "low_24h", "amount_24h", "reported_volume_24h", "reported_open_interest"]) {
      assert.equal(normalizeMarket({ symbol: "DOGE", price: 2, [field]: -1 }), null);
      assert.ok(normalizeMarket({ symbol: "DOGE", price: 2, [field]: 0 }));
    }
    const zero = normalizeMarket({ symbol: "DOGE", price: 0 });
    assert.ok(zero, "a valid numeric zero is distinct from a refused row");
    const zeroRows = producer.buildMarketsRows(markets, [zero]);
    assert.equal(zeroRows[0].hasLiveData, true);
    assert.equal(zeroRows[0].snapshot, zero);
    assert.equal(normalizeMarket({ symbol: "DOGE", price: "2.5", change_24h: "-5" })?.price, 2.5);
    assert.equal(normalizeMarket({ symbol: "DOGE", price: -1 }), null);
    const recovered = normalizeMarketsFrame([{ symbol: "BAD", price: "bad" }, { symbol: "DOGE", price: 3 }]);
    assert.equal(recovered.length, 1);
    assert.ok(render(table(producer.buildMarketsRows(markets, recovered))).includes("$3.00"));
  } finally { client.clear(); }
}

async function marketsNullableSnapshotsReachMountedView() {
  assert.equal(process.env.PROOF_NULLABLE_MARKETS, "1", "run through the real Go gateway proof");
  const origin = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL!;
  const fixtureRequire = createRequire(new URL("../../../services/wallet/demo/package.json", import.meta.url));
  const { JSDOM } = fixtureRequire("jsdom");
  const dom = new JSDOM("<!doctype html><html><body></body></html>", { url: origin, pretendToBeVisual: true });
  const globals = new Map<string, PropertyDescriptor | undefined>();
  for (const [key, value] of Object.entries({ window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })) {
    globals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { value, configurable: true });
  }
  const { createRoot } = await import("react-dom/client");
  const stream = await import("../src/hooks/perps/useMarketsStream");
  assert.equal(stream.MARKETS_WS_URL, `${origin.replace(/^http/, "ws")}/ws/markets`);
  const { useMarketsRows, filterAndSort } = await import("../src/widgets/markets/useMarketsRows");
  const { MarketsTable, MarketsHighlights } = await import("../src/widgets/markets");
  const { useRealtimePrices, tickerFromSnapshot } = await import("../src/hooks/perps/useRealtimePrices");
  const { fmtPrice, fmtChangePct, fmtUsdCompact } = await import("../src/ui/utils/format");
  const { fundingWadFigure, parseFundingRateHourly } = await import("../src/core/perps/fundingRate");
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity, gcTime: Infinity } } });
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  let latest: ReturnType<typeof useMarketsRows> | undefined;
  let prices: ReturnType<typeof useRealtimePrices> | undefined;
  let snapshots: ReturnType<typeof stream.useMarketsSnapshot> = [];
  let options: Parameters<typeof filterAndSort>[1] = { sortKey: "symbol", sortDir: "asc" };
  function MountedMarkets(): ReactElement {
    latest = useMarketsRows();
    prices = useRealtimePrices();
    snapshots = stream.useMarketsSnapshot();
    const visible = filterAndSort(latest.rows, options);
    return createElement(Fragment, null,
      createElement("output", { "data-proof": "ready" }, String(latest.hasAnyLive)),
      createElement("div", { "data-proof": "table" }, createElement(MarketsTable, {
        rows: visible, loading: !latest.hasAnyLive, favorites: new Set<string>(), onToggleFavorite: noSelection,
        sortKey: options.sortKey ?? "symbol", sortDir: options.sortDir ?? "asc", onToggleSort: noSelection,
      })),
      createElement("div", { "data-proof": "highlights" }, createElement(MarketsHighlights, { rows: latest.rows, hasAnyLive: latest.hasAnyLive })),
    );
  }
  function renderMounted() {
    root.render(createElement(QueryClientProvider, { client },
      createElement(RouterContext.Provider, { value: Router as NextRouter }, createElement(MountedMarkets))));
  }
  async function until(predicate: () => boolean) {
    const deadline = Date.now() + 12_000;
    while (!predicate()) {
      assert.ok(Date.now() < deadline, "real markets socket and mounted view must settle within twelve seconds");
      await act(async () => { await new Promise(resolve => setTimeout(resolve, 10)); });
    }
  }
  type RawSnapshot = {
    market_id: string; symbol: string; price: number | null; change_24h: number | null; last_updated: number | null;
    volume_24h: number; open_interest: number; funding_8h: string | null;
    perp_funding_rate: string | null; last_funding_rate_hourly: string | null;
  };
  let raw: RawSnapshot[] = [];
  const socket = new WebSocket(stream.MARKETS_WS_URL);
  socket.addEventListener("open", () => socket.send(JSON.stringify({ event: "subscribe", topic: "markets@all" })));
  socket.addEventListener("message", event => {
    const frame = JSON.parse(String(event.data));
    if (frame.topic === "markets@all" && Array.isArray(frame.data)) raw = frame.data;
  });
  try {
    await act(async () => { renderMounted(); });
    for (let phase = 0; phase <= 3; phase++) {
      if (phase > 0) {
        console.log(`PROOF_ADVANCE=${phase}`);
        const acknowledgement = join(process.env.PROOF_PHASE_DIR!, String(phase));
        await until(() => existsSync(acknowledgement));
        assert.equal(readFileSync(acknowledgement, "utf8"), String(phase));
      }
      function matches(rows: readonly { symbol: string; price: number | null; change_24h: number | null; last_updated: number | null }[]) {
        return rows.length === 18 && Array.from({ length: 8 }, (_, id) => {
          const symbol = raw.find(row => Number(row.market_id) === id)?.symbol;
          const row = rows.find(row => row.symbol === symbol);
          const mask = phase === 1 ? id : phase === 2 ? 0 : 7;
          return row && (row.price === null) === Boolean(mask & 1) &&
            (row.change_24h === null) === Boolean(mask & 2) && (row.last_updated === null) === Boolean(mask & 4) &&
            (row.price === null || row.price === 100 + id + phase);
        }).every(Boolean);
      }
      await until(() => matches(raw) && matches(snapshots) && latest?.rows.length === 18 && latest.rows.every(row => row.hasLiveData));
      assert.equal(latest!.hasAnyLive, true);
      assert.equal(container.querySelector('[data-proof="ready"]')?.textContent, "true");
      assert.equal(container.querySelectorAll('[data-proof="table"] [role="row"]').length, 19);
      assert.deepEqual(prices, { prices: {}, tickers: {} }, "producer frames without fresh price timestamps never invent a ticker");
      for (let id = 0; id < 8; id++) {
        const wire = raw.find(row => Number(row.market_id) === id)!;
        const row = latest!.bySymbol.get(wire.symbol)!;
        const snap = row.snapshot!;
        const mask = phase === 1 ? id : phase === 2 ? 0 : 7;
        assert.equal(wire.price === null, Boolean(mask & 1));
        assert.equal(wire.change_24h === null, Boolean(mask & 2));
        assert.equal(wire.last_updated === null, Boolean(mask & 4));
        assert.equal(row.price, wire.price);
        assert.equal(row.change24h, wire.change_24h);
        assert.equal(row.volume24h, 1200 + id);
        assert.equal(row.openInterest, 150 + id);
        const rate = id === 7 ? null : (id % 2 ? -1n : 1n) * 123456789012345678901n;
        assert.equal(row.fundingHourly, rate);
        assert.equal(row.funding8h, rate === null ? null : rate * 8n);
        assert.equal(snap.perp_funding_rate, rate);
        for (const [field, expected] of [["last_funding_rate_hourly", rate], ["perp_funding_rate", rate], ["funding_8h", rate === null ? null : rate * 8n]] as const) {
          assert.equal(wire[field] === null ? null : parseFundingRateHourly(wire[field]), expected);
        }
        const element = container.querySelector(`[data-proof="table"] button[aria-label="Add ${row.symbol} to favorites"]`)?.closest('[role="row"]');
        assert.ok(element);
        const text = element.textContent!;
        assert.ok(text.includes(fmtUsdCompact(row.volume24h)));
        assert.ok(text.includes(fmtUsdCompact(row.openInterest)));
        assert.ok(text.includes(fundingWadFigure(row.funding8h).text));
        if (wire.price === null) {
          assert.ok(!text.includes("$0.00"));
          assert.throws(() => tickerFromSnapshot(snap), /Market price unavailable/);
        } else {
          assert.ok(text.includes(`$${fmtPrice(wire.price)}`));
          const ticker = tickerFromSnapshot(snap);
          assert.equal(ticker.mark_price, wire.price);
          assert.equal(ticker.index_price, wire.price);
          assert.equal(ticker.last_funding_rate_hourly, rate);
        }
        const changes = [...element.querySelectorAll('span')].filter(span => span.textContent === (row.change24h === null ? "—" : fmtChangePct(row.change24h)));
        assert.ok(changes.length >= 2);
        if (row.change24h === null) {
          assert.ok(changes.every(span => !span.className.includes("text-emerald") && !span.className.includes("text-red")));
          assert.ok(!text.includes("+0.00%"));
        }
        const newListing = [...container.querySelectorAll('[data-proof="highlights"] button')].find(button => button.textContent?.includes(row.name));
        if (id < 5) {
          assert.ok(newListing, "new listings retain nullable snapshots");
          if (row.price !== null) assert.ok(newListing.textContent!.includes(`$${fmtPrice(row.price)}`));
          if (row.change24h !== null) assert.ok(newListing.textContent!.includes(fmtChangePct(row.change24h)));
        }
        const highlightPanels = container.querySelector('[data-proof="highlights"]')!.firstElementChild!.children;
        for (const panel of [highlightPanels[1], highlightPanels[2]]) {
          if (row.change24h === null) assert.ok(![...panel.querySelectorAll('button')].some(button => button.textContent?.includes(row.name)));
        }
      }
      for (const sortKey of ["price", "change24h"] as const) {
        for (const sortDir of ["asc", "desc"] as const) {
          const sorted = filterAndSort(latest!.rows, { sortKey, sortDir });
          let unavailable = false;
          let previous: number | undefined;
          for (const row of sorted) {
            const value = row[sortKey];
            if (value === null) { unavailable = true; continue; }
            assert.equal(unavailable, false, "unavailable metrics sort last in both directions");
            if (previous !== undefined) assert.ok(sortDir === "asc" ? value >= previous : value <= previous);
            previous = value;
          }
          options = { sortKey, sortDir };
          await act(async () => { renderMounted(); });
          const displayed = [...container.querySelectorAll('[data-proof="table"] [role="row"] button[aria-label^="Add "]')].map(button => button.getAttribute("aria-label"));
          assert.deepEqual(displayed, sorted.map(row => `Add ${row.symbol} to favorites`));
        }
      }
      const target = latest!.rows.find(row => row.marketId === 7)!;
      options = { search: target.symbol.toLowerCase(), category: "favorites", favorites: new Set([target.symbol]) };
      await act(async () => { renderMounted(); });
      assert.equal(container.querySelectorAll('[data-proof="table"] button[aria-label^="Add "]').length, 1);
      assert.ok(container.querySelector('[data-proof="table"]')!.textContent!.includes(target.symbol));
      assert.equal(filterAndSort(latest!.rows, { category: "new" }).length, 8);
      options = { search: "UNKNOWN" };
      await act(async () => { renderMounted(); });
      assert.ok(container.querySelector('[data-proof="table"]')!.textContent!.includes("No markets"));
      options = { sortKey: "symbol", sortDir: "asc" };
      await act(async () => { renderMounted(); });
      const highlights = container.querySelector('[data-proof="highlights"]')!;
      assert.equal(highlights.querySelectorAll(".animate-pulse").length, 0);
      if (phase === 0 || phase === 3) {
        assert.equal([...highlights.querySelectorAll('div')].filter(node => node.textContent === "No data yet").length, 2);
        for (const button of highlights.querySelectorAll('button')) {
          assert.ok(button.textContent!.includes("—"));
          assert.ok(!button.textContent!.includes("+0.00%"));
        }
      }
    }
  } finally {
    await act(async () => { root.unmount(); });
    socket.close();
    client.clear();
    await new Promise(resolve => setTimeout(resolve, 30));
    dom.window.close();
    for (const [key, descriptor] of globals) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
}

async function feeTierProducerReachesPanelAndRefusesMalformedView() {
  const { PortfolioFeeTierPanel } = await import("../src/widgets/portfolio/PortfolioFeeTierPanel");
  const { client, render } = await renderer();
  try {
    for (const tier of [2, 3, 2]) {
      const encoded = encodeFunctionResult({ abi: PerpsPrecompileABI, functionName: "getFeeTier",
        result: [tier, 250n, -50n, 123n * 10n ** 18n] });
      const produced = feeTierView(decodeFunctionResult({ abi: PerpsPrecompileABI, functionName: "getFeeTier", data: encoded }));
      for (let pass = 0; pass < 2; pass++) {
        const html = render(createElement(PortfolioFeeTierPanel, { address: undefined, view: produced }));
        assert.ok(html.includes(`Tier ${tier} · Taker 2.5 bps / Maker -0.5 bps`));
      }
      for (const malformed of [{ ...produced, tier: -1 }, { ...produced, takerBps: NaN }, { ...produced, volume14dUsd: -1n }]) {
        assert.throws(() => render(createElement(PortfolioFeeTierPanel, { address: undefined, view: malformed })), /Invalid fee tier view/);
      }
    }
    assert.ok(render(createElement(PortfolioFeeTierPanel, { address: undefined })).includes("Connect your wallet"));
  } finally { client.clear(); }
}

test(spotWidgetsReachDesktopMobileAndRouteConsumers);
test(pairHeaderConsumesWatchlistSelectionInSettlementUnits);
test(perpsLoaderDeliversTheRealOrderFormAndRefusesMalformedMarket);
test(marketsRowsCrossExportsIntoTheRealTableWithoutStaleResults);
test(feeTierProducerReachesPanelAndRefusesMalformedView);

test(marketsNullableSnapshotsReachMountedView);
