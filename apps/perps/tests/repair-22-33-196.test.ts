import assert from "node:assert/strict";
import { once } from "node:events";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { test } from "node:test";
import { isMainThread, parentPort, Worker, workerData } from "node:worker_threads";
import React from "react";
import "./fixtures/react-runtime";
import nodeTranscript from "./fixtures/leverage-node-output.json";
import { renderToStaticMarkup } from "react-dom/server";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { decodeFunctionData, encodeFunctionData, encodeFunctionResult, parseUnits, type Hex } from "viem";
import type { OrderFormState, OrderPlan } from "../src/core/perps/orderPlan";
import type { OpenPositionQuote } from "../src/hooks/perps/useQuoter";
import type { PendingOrder } from "../src/widgets/trading/PerpsOrderForm";
import type { ChartPriceField, DraggablePriceLine } from "../src/widgets/trading/chartOrderPrices";

const WAD = 10n ** 18n;
const NOW = Date.now();
const DAY = 86_400_000;
const ACCOUNT = "0x0000000000000000000000000000000000000001";
const OTHER_ACCOUNT = "0x0000000000000000000000000000000000000002";

const CONFIG = {
  marketId: 18n, symbol: "NEW", name: "New market", oracleSymbol: "NEWUSD",
  szDecimals: 3, pxDecimals: 8, tickSize: 10n ** 13n, lotSize: WAD,
  maxLeverage: 20n, tierClass: 0, impactNotionalUsd: 20_000n * WAD,
  oiCapUsd: 500_000_000n * WAD, fundingCapPpmPerHour: 100n,
  enabled: true, listedAt: BigInt(Math.floor((NOW - DAY) / 1000)),
};
const TIERS = [{ lowerBoundUsd: 0n, maxLeverage: 20n, maintenanceBps: 50n, deductionUsd: 0n }];
const INITIAL_QUOTE: OpenPositionQuote = {
  entryPrice: 100n * WAD, sizeUsd: 200n * WAD, collateralUsd: 100n * WAD,
  leverage: 2n * WAD, tradingFee: 1n * WAD, tradingFeeUsd: 1n * WAD,
  priceImpact: 0n, liquidationPrice: 50n * WAD, estimatedFunding24h: 0n,
  maintenanceMarginBps: 50n,
};
const FORM: OrderFormState = {
  orderType: "market", marketId: 18n, isLong: true, collateralAmount: "100", leverage: "2",
  triggerPrice: "", limitPrice: "", sizeUsd: "200", scaledStartPrice: "", scaledEndPrice: "",
  scaledTotalOrders: "5", scaledDistribution: "flat", tpslEnabled: false,
  tpPrice: "", slPrice: "", reduceOnly: false, postOnly: false, ioc: false, fok: false,
  ladderId: 1n,
};

function gatewayRow(marketId: number, symbol: string, createdAt: string | null) {
  return { marketId, symbol, name: `${symbol} market`, oracleSymbol: `${symbol}USD`,
    szDecimals: 3, pxDecimals: 8, tickSize: "10000000000000", lotSize: WAD.toString(),
    tierClass: 0, maxLeverage: "20", enabled: true, createdAt, latestPrice: null, fundingRate: null };
}
const MARKETS = [
  gatewayRow(18, "NEW", new Date(NOW - DAY).toISOString()),
  gatewayRow(5, "TSLA", new Date(NOW - 30 * DAY).toISOString()),
  gatewayRow(19, "OLD", new Date(NOW - 15 * DAY).toISOString()),
];
const TICKER = {
  symbol: "PERP_NEW_USDC", paxeerSymbol: "NEW", mark_price: 101, index_price: 99,
  open_interest: 500, last_funding_rate_hourly: null,
  "24h_open": 100, "24h_close": 102, "24h_high": 110, "24h_low": 90,
  "24h_amount": 100, "24h_volume": 10_000,
};

async function fixture() {
  // No src module is imported until every import-time URL names this fixture.
  let abi: typeof import("../src/core/network/abi/PerpsPrecompile").PerpsPrecompileABI;
  const state = {
    tickSize: CONFIG.tickSize, quote: { ...INITIAL_QUOTE }, failQuote: false,
    reads: [] as string[], errors: [] as unknown[], submitted: [] as Hex[],
  };
  const server = createServer(async (request, response) => {
    try {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      const raw = Buffer.concat(chunks).toString();
      response.setHeader("Content-Type", "application/json");
      if (request.url === "/graphql") {
        assert.equal(request.method, "POST");
        assert.match(JSON.parse(raw).query, /markets/);
        response.end(JSON.stringify({ data: { markets: MARKETS } }));
      } else if (request.url === "/api/markets/tickers") {
        response.end(JSON.stringify({ success: true, data: [TICKER] }));
      } else if (request.url === "/book") {
        response.end(JSON.stringify({ bids: [["98", "2"], ["100", "3"]], asks: [["106", "4"], ["104", "1"]] }));
      } else if (request.url === "/stats") {
        response.end(JSON.stringify({ symbol: "NEW", mark_price: 103, index_price: 97, volume_24h: 10_000, open_interest: 500 }));
      } else if (request.url === "/orders") {
        const submitted = JSON.parse(raw) as { data: Hex };
        const decoded = decodeFunctionData({ abi, data: submitted.data });
        assert.equal(decoded.functionName, "placeOrder");
        state.submitted.push(submitted.data);
        response.end(JSON.stringify({ accepted: true }));
      } else {
        assert.equal(request.url, "/");
        const answer = (rpc: { id: number; method: string; params?: unknown[] }) => {
          if (rpc.method === nodeTranscript.metadata.listening.request.method) {
            assert.deepEqual(rpc.params ?? [], nodeTranscript.metadata.listening.request.params);
            return { ...nodeTranscript.metadata.listening.response, id: rpc.id };
          }
          if (rpc.method === "eth_blockNumber") return { jsonrpc: "2.0", id: rpc.id, result: "0x100" };
          if (rpc.method === "eth_chainId") return { jsonrpc: "2.0", id: rpc.id, result: "0x7d" };
          assert.equal(rpc.method, "eth_call");
          const call = rpc.params?.[0] as { data: Hex; to: string };
          assert.match(call.to, /1015$/i);
          const decoded = decodeFunctionData({ abi, data: call.data });
          state.reads.push(decoded.functionName);
          let result: Hex;
          switch (decoded.functionName) {
            case "getMarketConfig":
              assert.equal(decoded.args[0], 18n);
              result = encodeFunctionResult({ abi, functionName: "getMarketConfig", result: { ...CONFIG, tickSize: state.tickSize } });
              break;
            case "getMarginTiers":
              assert.equal(decoded.args[0], 18n);
              result = encodeFunctionResult({ abi, functionName: "getMarginTiers", result: TIERS });
              break;
            case "getFeeTier": {
              const account = decoded.args[0].toLowerCase();
              assert.ok([ACCOUNT, OTHER_ACCOUNT].includes(account));
              result = encodeFunctionResult({ abi, functionName: "getFeeTier", result:
                account === ACCOUNT ? [3, 450n, -25n, 5_000_000n * WAD] : [0, 600n, 300n, 0n] });
              break;
            }
            case "quoteOpenPosition":
              assert.equal(decoded.args[0], 18n);
              assert.equal(decoded.args[2], 100_000_000n);
              assert.equal(decoded.args[3], 2n * WAD);
              assert.equal(decoded.args[4], true);
              if (state.failQuote) return { jsonrpc: "2.0", id: rpc.id, error: { code: -32000, message: "Quote unavailable" } };
              result = encodeFunctionResult({ abi, functionName: "quoteOpenPosition", result: state.quote });
              break;
            default: throw new Error(`Unexpected RPC ${decoded.functionName}`);
          }
          return { jsonrpc: "2.0", id: rpc.id, result };
        };
        const rpc = JSON.parse(raw);
        response.end(JSON.stringify(Array.isArray(rpc) ? rpc.map(answer) : answer(rpc)));
      }
    } catch (error) {
      state.errors.push(error);
      response.writeHead(500).end(JSON.stringify({ error: String(error) }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  const origin = `http://127.0.0.1:${address.port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = origin.replace("http:", "ws:") + "/socket";
  process.env.NEXT_PUBLIC_PAXEER_RPC_URLS = origin;
  abi = (await import("../src/core/network/abi/PerpsPrecompile")).PerpsPrecompileABI;
  const form = await import("../src/widgets/trading/PerpsOrderForm");
  const query = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const marketConfig = await import("../src/hooks/perps/useMarketConfig");
  const config = () => query.fetchQuery({ ...marketConfig.marketConfigQueryOptions(18n), staleTime: 0 });
  const { useQuoter } = await import("../src/hooks/perps/useQuoter");
  const { COLLATERAL_OPTIONS } = await import("../src/core/perps/config");
  const quoteInputs = { marketId: 18n, collateralToken: COLLATERAL_OPTIONS[0].value,
    collateralAmount: 100_000_000n, leverage: 2n * WAD, isLong: true };
  const inputHash = form.quoteInputHash({ marketId: 18n, collateralToken: quoteInputs.collateralToken,
    collateralBigInt: quoteInputs.collateralAmount, leverageBigInt: quoteInputs.leverage, isLong: true });
  const quote = async (takenAt: number) => form.bindOrderQuote(await useQuoter().quoteOpenPosition(quoteInputs), inputHash, takenAt);
  return {
    state, origin, abi, form, query, config, quote, inputHash,
    async submit(plan: OrderPlan) {
      for (const call of plan.calls) {
        const data = encodeFunctionData({ abi, functionName: "placeOrder", args: call.args });
        const response = await fetch(`${origin}/orders`, { method: "POST", body: JSON.stringify({ data }) });
        assert.equal(response.status, 200);
        assert.deepEqual(await response.json(), { accepted: true });
      }
    },
    async close() {
      query.clear();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
      assert.deepEqual(state.errors, [], "fixture must reject every unrecorded request");
    },
  };
}

async function inWorker(proof: string) {
  const require = createRequire(import.meta.url);
  const worker = new Worker(`require(${JSON.stringify(require.resolve("tsx/cjs"))}); require(${JSON.stringify(require.resolve("tsx/esm/api"))}).register(); require(${JSON.stringify(new URL(import.meta.url).pathname)});`, {
    eval: true, workerData: { proof },
  });
  try {
    const [result] = await Promise.race([
      once(worker, "message", { signal: AbortSignal.timeout(45_000) }),
      once(worker, "exit").then(([code]) => { throw new Error(`Proof worker exited without a result (${code})`); }),
    ]);
    assert.deepEqual(result, { ok: true });
  } finally {
    await worker.terminate();
  }
}

class MemoryOrderLine implements DraggablePriceLine {
  text = "";
  constructor(public price: number) {}
  getPrice() { return this.price; }
  setPrice(value: number) { this.price = value; }
  setText(value: string) { this.text = value; }
}

async function f045_chart_drag_uses_market_tick_grid() {
  if (isMainThread) return inWorker(f045_chart_drag_uses_market_tick_grid.name);
  const f = await fixture();
  try {
    const { chartOrderLineDrag } = await import("../src/widgets/trading/Chart");
    const { useChartPriceChange, snapChartPrice } = await import("../src/widgets/trading/chartOrderPrices");
    const { useOrderLineStore } = await import("../src/hooks/perps/useOrderLineStore");
    const scenarios = [
      { tick: 10n ** 13n, coordinate: 0.012345, expected: "0.01235" },
      { tick: 25n * 10n ** 16n, coordinate: 12.37, expected: "12.25" },
      { tick: 5n * WAD, coordinate: 103, expected: "105" },
      { tick: 1n, coordinate: 1.4e-16, expected: "0.00000000000000014" },
      { tick: 10n ** 13n, coordinate: 0.012344999999, expected: "0.01234" },
    ];
    for (const scenario of scenarios) {
      f.state.tickSize = scenario.tick;
      const data = await f.config();
      for (const field of ["entryPrice", "tpPrice", "slPrice"] as const) {
        const line = new MemoryOrderLine(scenario.coordinate);
        const handler = chartOrderLineDrag("NEW", field, () => ({ symbol: "NEW", tickSize: data.config.tickSize }));
        const sequence = useOrderLineStore.getState()._chartDragSeq;
        handler.call(line);
        assert.equal(line.price, Number(scenario.expected));
        assert.match(line.text, new RegExp(scenario.expected.replaceAll(".", "\\.")));
        assert.deepEqual(useChartPriceChange.getState().change, { symbol: "NEW", field, price: scenario.expected });
        assert.equal(useOrderLineStore.getState()[field], Number(scenario.expected));
        assert.equal(useOrderLineStore.getState()._chartDragSeq, sequence + 1);
      }
    }
    const before = useChartPriceChange.getState().change;
    chartOrderLineDrag("NEW", "entryPrice", () => ({ symbol: "OTHER", tickSize: 1n })).call(new MemoryOrderLine(1));
    chartOrderLineDrag("NEW", "entryPrice", () => ({ symbol: "NEW", tickSize: null })).call(new MemoryOrderLine(1));
    assert.equal(useChartPriceChange.getState().change, before);
    assert.throws(() => snapChartPrice(NaN, 1n), RangeError);
    assert.throws(() => snapChartPrice(1, 0n), RangeError);
    assert.throws(() => snapChartPrice(0.001, WAD), RangeError);
    assert.ok(f.state.reads.includes("getMarketConfig"));
  } finally { await f.close(); }
}

async function f033_drag_and_typed_prices_reach_the_same_exact_plan() {
  if (isMainThread) return inWorker(f033_drag_and_typed_prices_reach_the_same_exact_plan.name);
  const f = await fixture();
  try {
    const { chartOrderLineDrag } = await import("../src/widgets/trading/Chart");
    const { useChartPriceChange } = await import("../src/widgets/trading/chartOrderPrices");
    for (const scenario of [
      { tick: 10n ** 13n, entry: 0.012345, tp: 0.02, sl: 0.01, expected: "0.01235" },
      { tick: 25n * 10n ** 16n, entry: 12.37, tp: 15.12, sl: 10.13, expected: "12.25" },
    ]) {
      f.state.tickSize = scenario.tick;
      const data = await f.config();
      const values: Record<ChartPriceField, string> = { entryPrice: "", tpPrice: "", slPrice: "" };
      const setters = Object.fromEntries(Object.keys(values).map((field) => [field, (value: string) => { values[field as ChartPriceField] = value; }])) as Record<ChartPriceField, (value: string) => void>;
      const unsubscribe = useChartPriceChange.subscribe(({ change }) => {
        if (change) f.form.applyChartPriceChange(change, "NEW", setters);
      });
      try {
        for (const [field, coordinate] of [["entryPrice", scenario.entry], ["tpPrice", scenario.tp], ["slPrice", scenario.sl]] as const)
          chartOrderLineDrag("NEW", field, () => ({ symbol: "NEW", tickSize: scenario.tick })).call(new MemoryOrderLine(coordinate));
      } finally { unsubscribe(); }
      assert.equal(values.entryPrice, scenario.expected);
      const state: OrderFormState = { ...FORM, orderType: "limit", triggerPrice: values.entryPrice,
        tpslEnabled: true, tpPrice: values.tpPrice, slPrice: values.slPrice };
      const dragged = f.form.preparePendingOrder(state, null, data);
      const typed = f.form.preparePendingOrder({ ...state, triggerPrice: scenario.expected }, null, data);
      assert.deepEqual(dragged.plan.calls, typed.plan.calls);
      const dataBytes = encodeFunctionData({ abi: f.abi, functionName: "placeOrder", args: dragged.plan.calls[0].args });
      const decoded = decodeFunctionData({ abi: f.abi, data: dataBytes });
      assert.equal(decoded.functionName, "placeOrder");
      if (decoded.functionName !== "placeOrder") throw new Error("Unexpected call");
      assert.equal(decoded.args[0].limitPrice, parseUnits(scenario.expected, 18));
      assert.equal(decoded.args[0].takeProfitPrice, parseUnits(values.tpPrice, 18));
      assert.equal(decoded.args[0].stopLossPrice, parseUnits(values.slPrice, 18));
      assert.throws(() => f.form.preparePendingOrder({ ...state, triggerPrice: "0.012351" }, null, data));
      assert.throws(() => f.form.preparePendingOrder({ ...state, tpPrice: "0.020001" }, null, data), /tick size/);
      assert.throws(() => f.form.preparePendingOrder({ ...state, slPrice: "0.010001" }, null, data), /tick size/);
      f.form.applyChartPriceChange({ symbol: "OTHER", field: "entryPrice", price: "999" }, "NEW", setters);
      assert.equal(values.entryPrice, scenario.expected);
    }
  } finally { await f.close(); }
}

async function f028_watchlist_uses_catalog_membership_and_listing_window() {
  if (isMainThread) return inWorker(f028_watchlist_uses_catalog_membership_and_listing_window.name);
  const f = await fixture();
  try {
    const { perpsMarketsQueryOptions } = await import("../src/hooks/perps/usePerpsMarkets");
    const { MarketWatchlist, filterWatchlist, watchlistMetadata } = await import("../src/widgets/trading/MarketSelectorPanel");
    const catalog = await f.query.fetchQuery(perpsMarketsQueryOptions());
    const fresh = catalog.tradeable.find((market) => market.marketId === 18);
    assert.ok(fresh);
    assert.equal(filterWatchlist(catalog.tradeable, "new", "", NOW)[0].symbol, "NEW");
    assert.deepEqual(filterWatchlist(catalog.tradeable, "rwa", "", NOW).map((market) => market.symbol), ["TSLA"]);
    // Canonical catalog metadata can classify any new id, independently of its new-listing flag.
    const newStock = { ...fresh, category: "stocks", isNew: true };
    assert.deepEqual(filterWatchlist([newStock], "rwa", "", NOW), [newStock]);
    assert.deepEqual(filterWatchlist([newStock], "new", "", NOW), [newStock]);
    assert.equal(watchlistMetadata({ ...fresh, category: "new", listedAt: new Date(NOW - 15 * DAY).toISOString() }, NOW).isNew, false);
    assert.equal(watchlistMetadata({ ...fresh, listedAt: new Date(NOW - 14 * DAY).toISOString() }, NOW).isNew, true);
    assert.equal(watchlistMetadata({ ...fresh, listedAt: new Date(NOW - 14 * DAY - 1).toISOString() }, NOW).isNew, false);
    assert.equal(watchlistMetadata({ ...fresh, listedAt: new Date(NOW + 1).toISOString() }, NOW).isNew, false);
    assert.equal(watchlistMetadata({ ...fresh, listedAt: null }, NOW).isNew, false);
    assert.equal(watchlistMetadata({ ...fresh, isNew: false }, NOW).isNew, false);
    assert.deepEqual(filterWatchlist([newStock], "rwa", "missing", NOW), []);
    // Render the real catalog consumer with the query provider, including its New badge.
    const html = renderToStaticMarkup(React.createElement(QueryClientProvider, { client: f.query },
      React.createElement(MarketWatchlist, { selectedSymbol: "NEW", onSelect() {}, chrome: "popover" })));
    assert.match(html, /NEW/);
    assert.match(html, /aria-label="New listing"[^>]*>NEW</);
  } finally { await f.close(); }
}

async function f030_uncalled_legacy_funding_layout_is_retired() {
  const path = new URL("../src/widgets/trading/MobileTradingLayout.tsx", import.meta.url);
  assert.equal(existsSync(path), false, "the unused repeating-interval countdown must be retired");
  function checkDirectory(directory: URL) {
    for (const file of readdirSync(directory, { withFileTypes: true })) {
      const path = new URL(file.name + (file.isDirectory() ? "/" : ""), directory);
      if (file.isDirectory()) checkDirectory(path);
      else if (/\.[jt]sx?$/.test(file.name)) assert.doesNotMatch(readFileSync(path, "utf8"), /\b(?:from\s*|import\s*\()\s*["'][^"']*MobileTradingLayout["']/);
    }
  }
  checkDirectory(new URL("../src/", import.meta.url));
}

async function f025_orderbook_midpoint_and_one_sided_states_match_depth() {
  if (isMainThread) return inWorker(f025_orderbook_midpoint_and_one_sided_states_match_depth.name);
  const f = await fixture();
  try {
    const { parseVenueBook } = await import("../src/core/net/venueStream");
    const { buildBook, OrderbookMidValue, OrderbookMidCaption } = await import("../src/widgets/trading/Orderbook");
    const raw = await (await fetch(`${f.origin}/book`)).json();
    const venue = parseVenueBook(raw, NOW);
    assert.ok(venue);
    const map = (side: typeof venue.bids) => side.map((level) => ({ price: level.price, quantity: level.size }));
    const book = buildBook(map(venue.asks), map(venue.bids));
    assert.equal(book.bids[0].price, 100);
    assert.equal(book.asks[0].price, 104);
    assert.equal(book.mid, 102);
    assert.equal(book.spread, 4);
    assert.equal(book.spreadPct, 4 / 102 * 100);
    assert.match(renderToStaticMarkup(React.createElement(OrderbookMidValue, { book })), />102\.00</);
    assert.match(renderToStaticMarkup(React.createElement(OrderbookMidCaption, { book })), /mid · spread 4\.00/);
    for (const [asks, bids, label] of [[[], map(venue.bids), "Bid-only"], [map(venue.asks), [], "Ask-only"], [[], [], "Empty"]] as const) {
      const oneSided = buildBook([...asks], [...bids]);
      assert.equal(oneSided.mid, null);
      assert.equal(oneSided.spread, null);
      assert.equal(oneSided.spreadPct, null);
      const caption = renderToStaticMarkup(React.createElement(OrderbookMidCaption, { book: oneSided }));
      assert.match(caption, new RegExp(label));
      assert.doesNotMatch(caption, />mid · spread/);
      assert.match(renderToStaticMarkup(React.createElement(OrderbookMidValue, { book: oneSided })), /—/);
    }
  } finally { await f.close(); }
}

async function f029_authoritative_mark_index_and_reference_prices_keep_their_identity() {
  if (isMainThread) return inWorker(f029_authoritative_mark_index_and_reference_prices_keep_their_identity.name);
  const f = await fixture();
  try {
    const { PairHeader, PairPriceChips, pairPriceMeasures, pairTickersQueryOptions } = await import("../src/widgets/trading/PairHeader");
    const { perpsMarketsQueryOptions } = await import("../src/hooks/perps/usePerpsMarkets");
    const { parseSidPaxStats } = await import("../src/hooks/perps/useSidPaxMarketStream");
    const { parseVenueBook } = await import("../src/core/net/venueStream");
    const tickers = await f.query.fetchQuery(pairTickersQueryOptions());
    const stats = parseSidPaxStats(await (await fetch(`${f.origin}/stats`)).json(), "NEW");
    const book = parseVenueBook(await (await fetch(`${f.origin}/book`)).json(), NOW);
    assert.ok(stats && book);
    const measures = pairPriceMeasures(tickers.NEW, stats, book, [{ price: 107 }]);
    assert.deepEqual(measures, { mark: 101, index: 99, referenceMid: 102, lastTrade: 107 });
    assert.deepEqual(pairPriceMeasures(undefined, stats, book, []), { mark: 103, index: 97, referenceMid: 102, lastTrade: null });
    const unavailable = pairPriceMeasures(undefined, null, book, [{ price: 107 }]);
    assert.equal(unavailable.mark, null);
    assert.equal(unavailable.index, null);
    const chips = renderToStaticMarkup(React.createElement(PairPriceChips, { measures: unavailable }));
    assert.match(chips, /Index.*Unavailable/);
    assert.match(chips, /Reference mid.*102/);
    assert.match(chips, /Last trade.*107/);
    await f.query.fetchQuery(perpsMarketsQueryOptions());
    const html = renderToStaticMarkup(React.createElement(QueryClientProvider, { client: f.query },
      React.createElement(PairHeader, { selectedPair: "NEW/USD", onPairChange() {} })));
    assert.match(html, /Mark/);
    assert.match(html, /\$101\.00/);
    assert.match(html, /Index.*\$99\.00/);
    const missing = pairPriceMeasures({ mark_price: NaN, index_price: 0 }, null, { bids: [{ price: 100 }], asks: [] }, []);
    assert.deepEqual(missing, { mark: null, index: null, referenceMid: null, lastTrade: null });
  } finally { await f.close(); }
}

async function f032_confirmation_refreshes_expired_quotes_before_submission() {
  if (isMainThread) return inWorker(f032_confirmation_refreshes_expired_quotes_before_submission.name);
  const f = await fixture();
  try {
    const marketData = await f.config();
    const original = f.form.preparePendingOrder(FORM, await f.quote(NOW), marketData);
    assert.ok(original.boundQuote?.identity);
    let now = NOW + 6_001;
    let shown: PendingOrder | null = null;
    let current = true;
    f.state.quote = { ...INITIAL_QUOTE, entryPrice: 105n * WAD, tradingFeeUsd: 2n * WAD };
    const context = {
      inputHash: f.inputHash, now: () => now,
      requote: () => f.quote(now), isCurrent: () => current,
      present: (value: PendingOrder) => { shown = value; }, submit: f.submit,
    };
    assert.equal(await f.form.confirmPendingOrder(original, context), "refreshed");
    assert.equal(f.state.submitted.length, 0, "expired confirmation must not sign");
    assert.ok(shown);
    const refreshed = shown as PendingOrder;
    assert.notEqual(refreshed.boundQuote?.identity, original.boundQuote?.identity);
    assert.equal(refreshed.boundQuote?.takenAt, now);
    assert.equal(refreshed.confirmation.tradingFeeUsd, 2n * WAD);
    assert.notEqual(refreshed.confirmation.acceptablePrice, original.confirmation.acceptablePrice);
    assert.equal(await f.form.confirmPendingOrder(refreshed, context), "submitted");
    assert.equal(f.state.submitted.length, 1);
    const submitted = decodeFunctionData({ abi: f.abi, data: f.state.submitted[0] });
    assert.equal(submitted.functionName, "placeOrder");
    if (submitted.functionName !== "placeOrder") throw new Error("Unexpected call");
    assert.equal(submitted.args[0].acceptablePrice, refreshed.confirmation.acceptablePrice);
    assert.equal(f.state.reads.filter((read) => read === "quoteOpenPosition").length, 2);
    await assert.rejects(f.form.confirmPendingOrder(refreshed, { ...context, inputHash: "changed" }), /inputs changed/);
    current = false;
    assert.equal(await f.form.confirmPendingOrder(refreshed, context), "cancelled");
    assert.equal(f.state.submitted.length, 1);
    current = true;
    now += 6_001;
    f.state.failQuote = true;
    await assert.rejects(f.form.confirmPendingOrder(refreshed, context));
    assert.equal(f.state.submitted.length, 1);
    f.state.failQuote = false;
    // An edit/cancel while the real quote is in flight cannot present or submit it.
    assert.equal(await f.form.confirmPendingOrder(refreshed, { ...context, requote: async () => {
      const result = await f.quote(now);
      current = false;
      return result;
    } }), "cancelled");
    assert.equal(f.state.submitted.length, 1);
    current = true;
    await assert.rejects(f.form.confirmPendingOrder(refreshed, { ...context, requote: () => f.quote(NOW) }), /current quote/);
    assert.equal(f.state.submitted.length, 1);
  } finally { await f.close(); }
}

async function f031_fee_preview_renders_the_connected_accounts_onchain_tier() {
  if (isMainThread) return inWorker(f031_fee_preview_renders_the_connected_accounts_onchain_tier.name);
  const f = await fixture();
  try {
    const { useFeeTier, feeTierQueryOptions } = await import("../src/hooks/perps/useFeeTier");
    function AccountFees({ address }: { address: string | undefined }) {
      const tier = useFeeTier(address);
      return React.createElement(f.form.FeeTierRates, { view: tier.view });
    }
    const render = (address: string | undefined) => renderToStaticMarkup(React.createElement(QueryClientProvider,
      { client: f.query }, React.createElement(AccountFees, { address })));
    await f.query.fetchQuery(feeTierQueryOptions(ACCOUNT));
    assert.match(render(ACCOUNT), /0\.045% \/ -0\.0025%/);
    await f.query.fetchQuery(feeTierQueryOptions(OTHER_ACCOUNT));
    assert.match(render(OTHER_ACCOUNT), /0\.06% \/ 0\.03%/);
    assert.match(render(undefined), /Unavailable/);
    assert.equal(f.state.reads.filter((read) => read === "getFeeTier").length, 2);
    const bound = await f.quote(NOW);
    const pending = f.form.preparePendingOrder(FORM, bound, await f.config());
    assert.equal(pending.confirmation.tradingFeeUsd, INITIAL_QUOTE.tradingFeeUsd, "quoted trading fee stays independent of the tier rates");
  } finally { await f.close(); }
}

const proofs = {
  f045_chart_drag_uses_market_tick_grid,
  f033_drag_and_typed_prices_reach_the_same_exact_plan,
  f028_watchlist_uses_catalog_membership_and_listing_window,
  f030_uncalled_legacy_funding_layout_is_retired,
  f025_orderbook_midpoint_and_one_sided_states_match_depth,
  f029_authoritative_mark_index_and_reference_prices_keep_their_identity,
  f032_confirmation_refreshes_expired_quotes_before_submission,
  f031_fee_preview_renders_the_connected_accounts_onchain_tier,
};
if (isMainThread) {
  test(f045_chart_drag_uses_market_tick_grid);
  test(f033_drag_and_typed_prices_reach_the_same_exact_plan);
  test(f028_watchlist_uses_catalog_membership_and_listing_window);
  test(f030_uncalled_legacy_funding_layout_is_retired);
  test(f025_orderbook_midpoint_and_one_sided_states_match_depth);
  test(f029_authoritative_mark_index_and_reference_prices_keep_their_identity);
  test(f032_confirmation_refreshes_expired_quotes_before_submission);
  test(f031_fee_preview_renders_the_connected_accounts_onchain_tier);
} else {
  const proof = proofs[(workerData as { proof: keyof typeof proofs }).proof];
  proof().then(() => parentPort!.postMessage({ ok: true })).catch((error: unknown) => {
    parentPort!.postMessage({ ok: false, error: error instanceof Error ? error.stack : String(error) });
  });
}
