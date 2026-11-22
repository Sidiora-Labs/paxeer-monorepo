import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import type { FeedTicker } from "../packages/paxeer-backend/src/nativeFeedMarkets";

async function marketModules() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";
  return {
    ...(await import("../packages/paxeer-backend/src/hooks/perps/useMarketsStream")),
    ...(await import("../packages/paxeer-backend/src/nativeFeedMarkets")),
    ...(await import("../packages/paxeer-backend/src/hooks/perps/useRealtimePrices")),
    ...(await import("../packages/paxeer-backend/src/nativeMetadata")),
    ...(await import("../packages/paxeer-backend/src/runtime")),
  };
}

function streamRow() {
  return {
    market_id: "1",
    symbol: "BTC",
    symbol_full: "Bitcoin",
    price: 110,
    change_24h: 0.8,
    price_24h: 100,
    volume_24h: 550,
    open_interest: 220,
    funding_8h: null,
    last_funding_rate_hourly: null,
    volume_7d: 1100,
    volume_30d: 2200,
    trades_count: 5,
    last_updated: Date.now(),
    perp_mark_price: 111,
    perp_index_price: 110,
    perp_basis_bps: 90.909,
    perp_funding_rate: null,
    perp_next_funding_ms: Date.now() + 3600000,
    perp_long_short_ratio: null,
    perp_liq_volume_24h: 0,
    perp_oi_contracts: 2,
    perp_oi_usd: 220,
    perp_volume_24h_contracts: 5,
    perp_volume_24h_usd: 550,
  };
}

function tickerRow(updated_at: number | null = Date.now()): FeedTicker {
  return {
    symbol: "PERP_BTC_USDC",
    paxeerSymbol: "BTC",
    mark_price: 110,
    index_price: 109,
    price_24h: 100,
    updated_at,
    open_interest: 218,
    last_funding_rate_hourly: "0.000001",
    last_funding_rate: 0.000001,
    "24h_open": 40,
    "24h_close": 50,
    "24h_high": 60,
    "24h_low": 30,
    "24h_amount": 5,
    "24h_volume": 550,
  };
}

async function streamChangeUsesThePriceAtTheStartOfTheWindow() {
  const { normalizeMarket } = await marketModules();
  for (const change_24h of [0.8, -0.5, null, undefined]) {
    const market = normalizeMarket({ ...streamRow(), change_24h });
    assert.ok(market);
    assert.ok(Math.abs(market.change_24h! - 10) < 1e-12);
    assert.equal(market.price_24h, 100);
  }
  for (const price_24h of [null, undefined, 0, -1]) {
    const market = normalizeMarket({ ...streamRow(), price_24h });
    assert.ok(market);
    assert.equal(market.change_24h, null);
    assert.equal(market.price_24h, 0);
  }
  for (const price of [null, undefined, 0, -1]) {
    const market = normalizeMarket({ ...streamRow(), price });
    assert.ok(market);
    assert.equal(market.change_24h, null);
  }
}

async function streamMetricsUseTheServiceNamesAndUnits() {
  const { normalizeMarket, tickerFromSnapshot } = await marketModules();
  const row = streamRow();
  const market = normalizeMarket(row);
  assert.ok(market);
  assert.equal(market.price_updated_at, row.last_updated);
  assert.equal(market.volume_24h, 550);
  assert.equal(market.amount_24h, 5);
  assert.equal(market.open_interest, 220);
  assert.equal(market.high_24h, null);
  assert.equal(market.low_24h, null);
  const ticker = tickerFromSnapshot(market);
  assert.equal(ticker["24h_open"], 100);
  assert.equal(ticker["24h_amount"], 5);
  assert.equal(ticker["24h_volume"], 550);
  assert.equal(ticker["24h_high"], null);
  assert.equal(ticker["24h_low"], null);
  const absent = normalizeMarket({ ...row, price: null, last_updated: null });
  assert.ok(absent);
  assert.equal(absent.price, null);
  assert.equal(absent.price_updated_at, null);
}

async function freshTickersUseTheMarkPriceWindowAndKeepNullMetrics() {
  const { normalizeFeedMarkets, tickersFromWire } = await marketModules();
  const now = Date.now();
  const row = tickerRow(now);
  const [market] = normalizeFeedMarkets([row], now);
  assert.ok(market);
  assert.ok(Math.abs(market.change_24h! - 10) < 1e-12);
  assert.equal(market.price_24h, 100);
  assert.equal(market.price_updated_at, now);
  assert.equal(market.volume_24h, 550);
  assert.equal(market.amount_24h, 5);
  assert.equal(market.open_interest, 218);
  assert.equal(market.last_funding_rate_hourly, 1000000000000n);
  assert.equal(market.funding_8h, 8000000000000n);
  const tickers = tickersFromWire([row]);
  assert.equal(tickers.BTC["24h_open"], 100);
  assert.equal(tickers.BTC.index_price, 109);
  assert.equal(tickers.BTC["24h_high"], 60);
  const [absent] = normalizeFeedMarkets(
    [
      {
        ...row,
        price_24h: null,
        index_price: null,
        open_interest: null,
        last_funding_rate: null,
        last_funding_rate_hourly: null,
        "24h_open": null,
        "24h_close": null,
        "24h_high": null,
        "24h_low": null,
      },
    ],
    now,
  );
  assert.ok(absent);
  assert.equal(absent.change_24h, null);
  assert.equal(absent.price_24h, 0);
  assert.equal(absent.index_price, null);
  assert.equal(absent.open_interest, 0);
  assert.equal(absent.high_24h, null);
  assert.equal(absent.low_24h, null);
  assert.equal(absent.funding_8h, null);
}

async function tickersWithoutAFreshPriceTimeAreDropped() {
  const { normalizeFeedMarkets, tickersFromWire } = await marketModules();
  const now = Date.now();
  for (const updated_at of [now - 30001, now + 60000, null, undefined]) {
    const row = tickerRow(updated_at ?? null);
    if (updated_at === undefined) Reflect.deleteProperty(row, "updated_at");
    assert.deepEqual(normalizeFeedMarkets([row], now), []);
    assert.deepEqual(tickersFromWire([row]), {});
  }
  assert.deepEqual(normalizeFeedMarkets([tickerRow(now + 5001)], now), []);
  assert.equal(normalizeFeedMarkets([tickerRow(now - 30000)], now).length, 1);
  assert.equal(normalizeFeedMarkets([tickerRow(now + 5000)], now).length, 1);
}

async function workspaceRowsPreserveThePriceWindowAndNotionalUnits() {
  const {
    normalizeFeedMarkets,
    nativeMarketRows,
    updateNativeSnapshot,
    getNativeSnapshot,
  } = await marketModules();
  const previous = getNativeSnapshot().markets;
  try {
    updateNativeSnapshot({ markets: normalizeFeedMarkets([tickerRow()]) });
    const row = nativeMarketRows().find(
      (market) => market.symbol === "PERP_BTC_USDC",
    );
    assert.ok(row);
    assert.equal(row["24h_open"], 100);
    assert.equal(row["24h_volume"], 5);
    assert.equal(row["24h_amount"], 550);
    assert.equal(row.open_interest * row.index_price, 218);
    assert.equal(
      row.timestamp,
      getNativeSnapshot().markets[0].price_updated_at,
    );
    updateNativeSnapshot({
      markets: normalizeFeedMarkets([{ ...tickerRow(), price_24h: null }]),
    });
    assert.equal(
      nativeMarketRows().find((market) => market.symbol === row.symbol)?.[
        "24h_open"
      ],
      undefined,
    );
  } finally {
    updateNativeSnapshot({ markets: previous });
  }
}

function serviceFieldListsCarryThePriceWindowAndTickerTime() {
  const records = readFileSync(
    new URL(
      "../../../services/platform/perps-data/internal/records/records.go",
      import.meta.url,
    ),
    "utf8",
  );
  const rest = readFileSync(
    new URL(
      "../../../services/platform/perps-data/internal/rest/venue.go",
      import.meta.url,
    ),
    "utf8",
  );
  const feeds = readFileSync(
    new URL(
      "../../../services/platform/perps-data/internal/feeds/topics.go",
      import.meta.url,
    ),
    "utf8",
  );
  function fields(source: string, name: string) {
    const body = source.match(
      new RegExp(`type ${name} struct \\{([\\s\\S]*?)\\n\\}`),
    )?.[1];
    assert.ok(body, name);
    return new Set(
      [...body.matchAll(/json:"([^"]+)"/g)].map((match) => match[1]),
    );
  }
  const marketFields = fields(records, "MarketRecord");
  for (const key of Object.keys(streamRow()))
    assert.ok(marketFields.has(key), key);
  assert.ok(marketFields.has("price_24h"));
  assert.ok(marketFields.has("last_updated"));
  assert.match(feeds, /r\.Price24h = prices\[r\.MarketID\]\.Price/);
  for (const name of ["venueTicker", "venueTickerEntry"]) {
    const tickerFields = fields(rest, name);
    assert.ok(tickerFields.has("price_24h"));
    assert.ok(tickerFields.has("updated_at"));
    if (name === "venueTickerEntry") {
      for (const key of Object.keys(tickerRow()))
        assert.ok(tickerFields.has(key), key);
    }
  }
  assert.match(rest, /entry\.Price24h = prices\[market\]\.MarkPrice/);
  assert.match(rest, /entry\.UpdatedAt = &ms/);
  assert.match(rest, /ms := at\.Time\.UnixMilli\(\)/);
}

test(streamChangeUsesThePriceAtTheStartOfTheWindow);
test(streamMetricsUseTheServiceNamesAndUnits);
test(freshTickersUseTheMarkPriceWindowAndKeepNullMetrics);
test(tickersWithoutAFreshPriceTimeAreDropped);
test(workspaceRowsPreserveThePriceWindowAndNotionalUnits);
test(serviceFieldListsCarryThePriceWindowAndTickerTime);
