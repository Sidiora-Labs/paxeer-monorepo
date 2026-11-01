import assert from "node:assert/strict";
import { test } from "node:test";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";

const NOW = Date.parse("2026-10-26T12:00:00.000Z");
const DAY_MS = 24 * 60 * 60 * 1000;

async function loadCatalog() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://gateway.example.test";
  const catalog = await import("../src/core/perps/marketCatalog");
  const config = await import("../src/core/perps/config");
  return { ...catalog, ...config };
}

function gatewayRow(
  marketId: number,
  symbol: string,
  name: string,
  createdAt: string | null,
): GatewayMarketRow {
  return {
    marketId,
    symbol,
    name,
    oracleSymbol: symbol,
    szDecimals: 3,
    pxDecimals: 4,
    tickSize: "100000000000000",
    lotSize: "1000000000000000",
    tierClass: 0,
    maxLeverage: "20",
    enabled: true,
    createdAt,
    latestPrice: null,
    fundingRate: null,
  };
}

async function catalogIncludesChainListedMarket() {
  const { catalogFromGateway, PERPS_MARKETS } = await loadCatalog();
  assert.equal(PERPS_MARKETS.some((market) => market.symbol === "DOGE"), false);
  const listedAt = new Date(NOW - 30 * DAY_MS).toISOString();
  const doge = gatewayRow(18, "DOGE", "Dogecoin", listedAt);
  const rows: GatewayMarketRow[] = [
    doge,
    ...PERPS_MARKETS.map((market) =>
      gatewayRow(market.marketId, market.symbol, market.name, listedAt),
    ).reverse(),
  ];
  const originalOrder = rows.map((row) => row.marketId);
  const { catalog, tradeable } = catalogFromGateway({ markets: rows }, NOW);

  assert.equal(catalog.length, 19);
  assert.deepEqual(
    catalog.map((entry) => entry.marketId),
    Array.from({ length: 19 }, (_, i) => i),
  );
  assert.equal(catalog[17].marketId, 17);
  assert.deepEqual(catalog[18], {
    marketId: 18,
    symbol: "DOGE",
    name: "Dogecoin",
    szDecimals: 3,
    pxDecimals: 4,
    maxLeverage: 20,
    enabled: true,
    listedAt,
    category: "crypto",
  });
  assert.deepEqual(tradeable, catalog);
  assert.deepEqual(rows.map((row) => row.marketId), originalOrder);

  const disabledRows = rows.map((row) =>
    row.marketId === 18 ? { ...row, enabled: false } : row,
  );
  const disabled = catalogFromGateway({ markets: disabledRows }, NOW);
  assert.equal(disabled.catalog.length, 19);
  assert.deepEqual(disabled.catalog[18], { ...catalog[18], enabled: false });
  assert.deepEqual(
    disabled.tradeable.map((entry) => entry.marketId),
    Array.from({ length: 18 }, (_, i) => i),
  );
  assert.equal(disabledRows[0].enabled, false);
  assert.deepEqual(catalogFromGateway({ markets: [] }, NOW), {
    catalog: [],
    tradeable: [],
  });
}

async function catalogMarksRecentListingNew() {
  const { catalogFromGateway, getMarketCategory } = await loadCatalog();
  const listedDaysAgo = (days: number) =>
    new Date(NOW - days * DAY_MS).toISOString();
  const rows: GatewayMarketRow[] = [
    gatewayRow(18, "DOGE", "Dogecoin", listedDaysAgo(3)),
    gatewayRow(5, "TSLA", "Tesla", listedDaysAgo(30)),
    gatewayRow(7, "NAS100", "Nasdaq 100", listedDaysAgo(30)),
    gatewayRow(8, "XAU", "Gold", listedDaysAgo(30)),
    gatewayRow(0, "BTC", "Bitcoin", listedDaysAgo(14)),
    gatewayRow(
      1, "ETH", "Ethereum", new Date(NOW - 14 * DAY_MS - 1).toISOString(),
    ),
    gatewayRow(2, "SOL", "Solana", new Date(NOW + 1).toISOString()),
    {
      ...gatewayRow(3, "AVAX", "Avalanche", null),
      szDecimals: null,
      pxDecimals: null,
      maxLeverage: null,
    },
    gatewayRow(4, "LINK", "Chainlink", new Date(NOW).toISOString()),
  ];
  const { catalog } = catalogFromGateway({ markets: rows }, NOW);
  const byId = new Map(catalog.map((entry) => [entry.marketId, entry]));
  assert.equal(byId.get(18)?.category, "new");
  assert.equal(byId.get(18)?.listedAt, listedDaysAgo(3));
  for (const id of [5, 7, 8, 1, 2, 3]) {
    assert.equal(byId.get(id)?.category, getMarketCategory(id));
  }
  assert.equal(byId.get(0)?.category, "new");
  assert.equal(byId.get(4)?.category, "new");
  assert.equal(byId.get(3)?.listedAt, null);
  assert.equal(byId.get(3)?.szDecimals, null);
  assert.equal(byId.get(3)?.pxDecimals, null);
  assert.equal(byId.get(3)?.maxLeverage, null);
  assert.throws(
    () => catalogFromGateway({ markets: [
      gatewayRow(18, "DOGE", "Dogecoin", "invalid-time"),
    ] }, NOW),
    /Invalid listing time for market 18/,
  );
  assert.throws(
    () => catalogFromGateway({ markets: [
      { ...rows[0], maxLeverage: "invalid-leverage" },
    ] }, NOW),
    /Invalid max leverage for market 18/,
  );
  assert.throws(
    () => catalogFromGateway({ markets: [] }, NaN),
    /Invalid catalog time/,
  );
}

test(catalogIncludesChainListedMarket);
test(catalogMarksRecentListingNew);
