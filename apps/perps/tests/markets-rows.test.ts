import assert from "node:assert/strict";
import { test } from "node:test";
import type { GatewayMarketRow } from "../src/core/perps/marketCatalog";

const NOW = Date.parse("2026-10-27T12:00:00.000Z");
const DAY_MS = 24 * 60 * 60 * 1000;

function gatewayMarket(
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

async function newFilterUsesListingTime() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://gateway.example.test";
  const { catalogFromGateway } = await import("../src/core/perps/marketCatalog");
  const { buildMarketsRows } = await import("../src/widgets/markets/useMarketsRows");
  const { filterAndSort } = await import("../src/widgets/markets");
  const listedDaysAgo = (days: number) => new Date(NOW - days * DAY_MS).toISOString();
  const markets = [
    gatewayMarket(0, "BTC", "Bitcoin", listedDaysAgo(30)),
    gatewayMarket(5, "TSLA", "Tesla", listedDaysAgo(30)),
    gatewayMarket(6, "NVDA", "NVIDIA", listedDaysAgo(30)),
    gatewayMarket(7, "NAS100", "Nasdaq 100", listedDaysAgo(30)),
    gatewayMarket(8, "XAU", "Gold", listedDaysAgo(30)),
    gatewayMarket(9, "SPX500", "S&P 500", listedDaysAgo(30)),
    gatewayMarket(10, "GOOGL", "Alphabet", listedDaysAgo(30)),
    gatewayMarket(18, "DOGE", "Dogecoin", listedDaysAgo(3)),
  ];

  function rowsFromGateway(source: GatewayMarketRow[]) {
    const { tradeable } = catalogFromGateway({ markets: source }, NOW);
    return buildMarketsRows(
      tradeable.map((market) => {
        if (market.maxLeverage === null) {
          throw new Error("Expected leverage for enabled market");
        }
        return { ...market, baseToken: market.symbol, maxLeverage: market.maxLeverage };
      }),
      [],
    );
  }

  const rows = rowsFromGateway(markets);
  const newRows = filterAndSort(rows, { category: "new" });
  assert.deepEqual(newRows.map((row) => row.symbol), ["DOGE"]);
  assert.equal(newRows[0].marketId, 18);
  assert.equal(newRows[0].isNew, true);
  assert.equal(newRows[0].category, "new");
  assert.equal(newRows[0].maxLeverage, 20);
  assert.deepEqual(
    rows.filter((row) => row.marketId >= 5 && row.marketId <= 10)
      .map((row) => [row.marketId, row.isNew]),
    [[5, false], [6, false], [7, false], [8, false], [9, false], [10, false]],
  );
  assert.equal(filterAndSort(rows, { category: "all" }).length, markets.length);
  assert.equal(filterAndSort(rows, { category: "crypto" }).some((row) => row.symbol === "DOGE"), true);
  assert.deepEqual(filterAndSort(rows, { category: "new", search: "dogecoin" }), newRows);

  const boundaryRows = rowsFromGateway([
    ...markets,
    gatewayMarket(19, "BOUNDARY", "Listed exactly 14 days ago", listedDaysAgo(14)),
    gatewayMarket(20, "OLDER", "Listed before the window", new Date(NOW - 14 * DAY_MS - 1).toISOString()),
    gatewayMarket(21, "FUTURE", "Future listing", new Date(NOW + 1).toISOString()),
    gatewayMarket(22, "UNKNOWN", "Unknown listing time", null),
  ]);
  assert.deepEqual(
    filterAndSort(boundaryRows, { category: "new", sortKey: "symbol", sortDir: "asc" })
      .map((row) => row.symbol),
    ["BOUNDARY", "DOGE"],
  );
  assert.deepEqual(filterAndSort(rowsFromGateway([]), { category: "new" }), []);
}

test(newFilterUsesListingTime);
