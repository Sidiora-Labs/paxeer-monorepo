import assert from "node:assert/strict";
import { after, before, test, type TestContext } from "node:test";
import { createRequire } from "node:module";
import { createServer, type ServerResponse } from "node:http";
import { setImmediate } from "node:timers/promises";
import { parseUnits } from "viem";
import type { GatewayMarketRow } from "../packages/paxeer-backend/src/core/perps/marketCatalog";

const require = createRequire(import.meta.url);
const originalGateway = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
let metadata: typeof import("../packages/paxeer-backend/src/nativeMetadata");
let runtime: typeof import("../packages/paxeer-backend/src/runtime");
let config: typeof import("../packages/paxeer-backend/src/core/perps/config");
let catalog: typeof import("../packages/paxeer-backend/src/hooks/perps/usePerpsMarkets");
let queries: typeof import("@tanstack/react-query");
let mode: "empty" | "failure" | "late" | "catalog" = "empty";
let reads: number[] = [];
const waiting = new Set<ServerResponse>();
const server = createServer(async (request, response) => {
  let body = "";
  for await (const chunk of request) body += chunk;
  assert.equal(request.url, "/graphql");
  assert.match(JSON.parse(body).query, /query GetMarkets/);
  reads.push(Date.now());
  response.setHeader("Content-Type", "application/json");
  if (mode === "late") {
    waiting.add(response);
    response.on("close", () => waiting.delete(response));
    return;
  }
  response.end(
    JSON.stringify(
      mode === "failure"
        ? { errors: [{ message: "Market catalog unavailable" }] }
        : { data: { markets: mode === "catalog" ? catalogRows() : [] } },
    ),
  );
});

before(async () => {
  await new Promise<void>((resolve) => server.listen(0, resolve));
  const address = server.address();
  assert.ok(address && typeof address === "object");
  const host =
    address.family === "IPv6" ? `[${address.address}]` : address.address;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = `http://${host}:${address.port}`;
  queries = require("@tanstack/react-query");
  config = require("../packages/paxeer-backend/src/core/perps/config");
  runtime = require("../packages/paxeer-backend/src/runtime");
  metadata = require("../packages/paxeer-backend/src/nativeMetadata");
  catalog = require("../packages/paxeer-backend/src/hooks/perps/usePerpsMarkets");
});

after(async () => {
  server.closeAllConnections();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
  if (originalGateway === undefined)
    delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  else process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = originalGateway;
});

function catalogRows(): GatewayMarketRow[] {
  const rows = config.PERPS_MARKETS.slice(0, 3).map(
    (market): GatewayMarketRow => ({
      marketId: market.marketId,
      symbol: market.symbol,
      name: market.name,
      oracleSymbol: market.symbol,
      szDecimals: 8,
      pxDecimals: 2,
      tickSize: parseUnits("0.05", 18).toString(),
      lotSize: "1",
      tierClass: 0,
      maxLeverage: "50",
      enabled: true,
      createdAt: null,
      latestPrice: null,
      fundingRate: null,
    }),
  );
  rows[1].enabled = false;
  rows[2].tickSize = null;
  rows.push({
    ...rows[0],
    marketId: 100,
    symbol: "CATALOG",
    name: "Catalog market",
    oracleSymbol: "CATALOG",
    tickSize: parseUnits("0.000005", 18).toString(),
    pxDecimals: 6,
  });
  return rows;
}

async function until(predicate: () => boolean) {
  const deadline = performance.now() + 5000;
  while (!predicate() && performance.now() < deadline) await setImmediate();
  assert.ok(predicate(), "Catalog reaches the expected state");
}

function assertBuiltInRows(price: number) {
  const tick = price >= 1 ? 0.01 : price >= 0.01 ? 0.0001 : 0.000001;
  const digits = price >= 1 ? 2 : price >= 0.01 ? 4 : 6;
  runtime.updateNativeSnapshot({
    markets: config.PERPS_MARKETS.map((market) => ({
      symbol: market.symbol,
      price,
      index_price: price,
    })),
  });
  const symbols = metadata.nativeSymbols();
  const rows = metadata.nativeMarketRows();
  assert.equal(symbols.length, config.PERPS_MARKETS.length);
  assert.equal(rows.length, config.PERPS_MARKETS.length);
  for (const market of config.PERPS_MARKETS) {
    const symbol = metadata.toTradingSymbol(market.symbol);
    const info = symbols.find((row) => row.symbol === symbol);
    assert.ok(info, market.symbol);
    assert.ok(info.quote_tick > 0);
    assert.equal(info.quote_tick, tick);
    assert.equal(info.quote_dp, digits);
    assert.equal(info.quote_min, tick);
    const row = rows.find((row) => row.symbol === symbol)!;
    assert.equal(row.quote_tick, info.quote_tick);
    assert.equal(row.quote_dp, info.quote_dp);
    assert.equal(row.mark_price, price);
    assert.equal(row.index_price, price);
    assert.equal(row.last_price, price);
  }
}

async function marketListKeepsItsRowsWithoutTheCatalog() {
  mode = "empty";
  const empty = await catalog.loadPerpsMarkets();
  assert.deepEqual(empty.catalog, []);
  runtime.updateNativeSnapshot({ marketCatalog: empty.catalog });
  for (const price of [60000, 0.5, 0.004, 1, 0.01]) assertBuiltInRows(price);

  mode = "failure";
  await assert.rejects(
    catalog.loadPerpsMarkets(),
    /Market catalog unavailable/,
  );
  for (const price of [60000, 0.5, 0.004]) assertBuiltInRows(price);

  mode = "late";
  const pending = catalog.loadPerpsMarkets();
  await until(() => waiting.size === 1);
  let answered = false;
  void pending.then(() => {
    answered = true;
  });
  for (const price of [60000, 0.5, 0.004]) assertBuiltInRows(price);
  assert.equal(answered, false);
  for (const response of waiting)
    response.end(JSON.stringify({ data: { markets: [] } }));
  await pending;
}

async function catalogRefinesTheRowsWhenItArrives() {
  runtime.updateNativeSnapshot({ marketCatalog: [] });
  assertBuiltInRows(0.5);
  mode = "catalog";
  const answer = await catalog.loadPerpsMarkets();
  runtime.updateNativeSnapshot({ marketCatalog: answer.catalog });
  const symbols = metadata.nativeSymbols();
  const btc = symbols.find((row) => row.symbol === "PERP_BTC_USDC")!;
  assert.equal(btc.quote_tick, 0.05);
  assert.equal(btc.quote_dp, 2);
  assert.equal(btc.quote_min, 0.05);
  assert.equal(btc.max_leverage, 50);
  assert.equal(btc.base_imr, 1 / 50);
  assert.equal(
    symbols.some((row) => row.symbol === "PERP_ETH_USDC"),
    false,
  );
  const sol = symbols.find((row) => row.symbol === "PERP_SOL_USDC")!;
  assert.equal(sol.quote_tick, 0.0001);
  assert.equal(sol.quote_dp, 4);
  const added = symbols.find((row) => row.symbol === "PERP_CATALOG_USDC")!;
  assert.equal(added.quote_tick, 0.000005);
  assert.equal(added.quote_dp, 6);
  assert.equal(added.marketId, 100);
  assert.equal(symbols.length, config.PERPS_MARKETS.length);
  assert.deepEqual(
    metadata.nativeMarketRows().map((row) => row.symbol),
    symbols.map((row) => row.symbol),
  );

  runtime.updateNativeSnapshot({
    address: "0x0000000000000000000000000000000000000001",
    connected: true,
  });
  runtime.updateNativeSnapshot({ address: null, connected: false });
  assert.deepEqual(metadata.nativeSymbols(), symbols);
}

async function catalogReadIsTriedAgainAfterAFailure(t: TestContext) {
  mode = "failure";
  reads = [];
  runtime.updateNativeSnapshot({ marketCatalog: [] });
  assertBuiltInRows(60000);
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const client = new queries.QueryClient({
    defaultOptions: { queries: { gcTime: Infinity } },
  });
  queries.focusManager.setFocused(true);
  queries.onlineManager.setOnline(true);
  client.mount();
  const observer = new queries.QueryObserver(
    client,
    catalog.perpsMarketsQueryOptions(false),
  );
  const stop = observer.subscribe((result) => {
    if (result.data)
      runtime.updateNativeSnapshot({ marketCatalog: result.data.catalog });
  });
  t.after(() => {
    stop();
    client.unmount();
    client.clear();
    queries.focusManager.setFocused(undefined);
    queries.onlineManager.setOnline(true);
    t.mock.timers.reset();
  });
  const state = () => observer.getCurrentResult();
  async function advance(ms: number) {
    t.mock.timers.tick(ms);
    await setImmediate();
  }
  await advance(30000);
  assert.deepEqual(reads, []);
  observer.setOptions(catalog.perpsMarketsQueryOptions(true));
  await until(() => state().failureCount === 1);
  assert.deepEqual(reads, [30000]);
  for (const [i, delay] of [2000, 5000, 15000, 30000].entries()) {
    await advance(delay - 1);
    assert.equal(reads.length, i + 1);
    await advance(1);
    await until(() => state().failureCount === i + 2);
    assert.equal(reads.length, i + 2);
  }
  assert.deepEqual(reads, [30000, 32000, 37000, 52000, 82000]);

  queries.focusManager.setFocused(false);
  observer.setOptions(catalog.perpsMarketsQueryOptions(false));
  await advance(30000);
  await until(() => state().fetchStatus === "paused");
  await advance(60000);
  assert.equal(reads.length, 5);
  assertBuiltInRows(60000);
  // Public metadata remains available as the account changes.
  runtime.updateNativeSnapshot({ connected: true });
  runtime.updateNativeSnapshot({ connected: false });
  config.INDEXER_GRAPHQL.authGeneration =
    (config.INDEXER_GRAPHQL.authGeneration ?? 0) + 1;
  mode = "catalog";
  queries.focusManager.setFocused(true);
  observer.setOptions(catalog.perpsMarketsQueryOptions(true));
  await until(() => state().isSuccess && state().fetchStatus === "idle");
  assert.equal(reads.length, 6);
  assert.equal(
    metadata.nativeSymbols().find((row) => row.symbol === "PERP_BTC_USDC")!
      .quote_tick,
    0.05,
  );
  const good = state().data;
  mode = "failure";
  void observer.refetch();
  await until(() => state().failureCount === 1);
  assert.equal(state().data, good);
  assert.equal(
    metadata.nativeSymbols().find((row) => row.symbol === "PERP_BTC_USDC")!
      .quote_tick,
    0.05,
  );

  // Rotating authentication during a public read also permits recovery.
  mode = "late";
  await advance(2000);
  await until(() => waiting.size === 1);
  config.INDEXER_GRAPHQL.authGeneration =
    (config.INDEXER_GRAPHQL.authGeneration ?? 0) + 1;
  await until(() => state().failureCount === 2);
  mode = "catalog";
  await advance(5000);
  await until(() => state().isSuccess && state().fetchStatus === "idle");
  assert.equal(
    metadata.nativeSymbols().find((row) => row.symbol === "PERP_BTC_USDC")!
      .quote_tick,
    0.05,
  );
}

test(marketListKeepsItsRowsWithoutTheCatalog);
test(catalogRefinesTheRowsWhenItArrives);
test(catalogReadIsTriedAgainAfterAFailure);
