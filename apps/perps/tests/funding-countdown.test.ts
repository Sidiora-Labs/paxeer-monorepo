import assert from "node:assert/strict";
import test from "node:test";
import type { FeedTicker } from "../packages/paxeer-backend/src/nativeFeedMarkets";

async function fundingModules() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";
  return {
    ...(await import("../packages/paxeer-backend/src/hooks/perps/useMarketsStream")),
    ...(await import("../packages/paxeer-backend/src/nativeFeedMarkets")),
    ...(await import("../packages/paxeer-backend/src/nativeMetadata")),
    ...(await import("../packages/paxeer-backend/src/runtime")),
    ...(await import("../packages/hooks/src/paxeer-network/fundingRateDisplay")),
  };
}

const now = 1_747_526_400_000;
const next = now + 3_723_456;

function marketRecord() {
  return {
    symbol: "BTC",
    price: 110,
    price_24h: 100,
    last_updated: now,
    funding_8h: "-0.0000987654312",
    perp_funding_rate: "-0.0000123456789",
    last_funding_rate_hourly: "-0.0000123456789",
    perp_next_funding_ms: next,
  };
}

async function marketFundingTimeIsNullableWithoutDroppingTheRecord() {
  const { normalizeMarket } = await fundingModules();
  const present = normalizeMarket(marketRecord());
  assert.ok(present);
  assert.equal(present.perp_next_funding_ms, next);
  assert.equal(present.last_funding_rate_hourly, -12_345_678_900_000n);

  const { perp_next_funding_ms: _, ...absent } = marketRecord();
  const missing = normalizeMarket(absent);
  assert.ok(missing);
  assert.equal(missing.perp_next_funding_ms, null);

  for (const value of [
    undefined,
    null,
    0,
    -1,
    next + 0.5,
    NaN,
    Infinity,
    -Infinity,
    Number.MAX_SAFE_INTEGER + 1,
    String(next),
    "invalid",
    true,
    false,
    [],
    {},
  ]) {
    const market = normalizeMarket({
      ...marketRecord(),
      perp_next_funding_ms: value,
    });
    assert.ok(market, String(value));
    assert.equal(market.perp_next_funding_ms, null, String(value));
    assert.equal(market.price, 110);
    assert.equal(
      market.last_funding_rate_hourly,
      present.last_funding_rate_hourly,
    );
  }
  assert.equal(
    normalizeMarket({ ...marketRecord(), perp_next_funding_ms: 1 })
      ?.perp_next_funding_ms,
    1,
  );
}

async function nativeRowsCarryTheMarketTimeAndSettledHourlyRate() {
  const {
    normalizeMarket,
    nativeMarketRows,
    getNativeSnapshot,
    updateNativeSnapshot,
    fundingRateDisplay,
  } = await fundingModules();
  const previous = getNativeSnapshot().markets;
  try {
    const market = normalizeMarket(marketRecord());
    assert.ok(market);
    updateNativeSnapshot({ markets: [market] });
    const row = nativeMarketRows().find(
      (item) => item.symbol === "PERP_BTC_USDC",
    );
    assert.ok(row);
    assert.equal(row.next_funding_time, next);
    assert.equal(row.est_funding_rate, undefined);
    assert.equal(row.last_funding_rate_hourly, -12_345_678_900_000n);
    assert.equal(row.last_funding_rate, -98_765_431_200_000n);
    assert.deepEqual(fundingRateDisplay(row, now), {
      est_funding_rate: "-0.0012",
      countDown: "01:02:03",
    });
    assert.deepEqual(fundingRateDisplay(row, next + 1), {
      est_funding_rate: "-0.0012",
      countDown: "00:00:00",
    });

    const missing = normalizeMarket({
      ...marketRecord(),
      perp_next_funding_ms: null,
      last_funding_rate_hourly: null,
      perp_funding_rate: null,
      funding_8h: null,
    });
    assert.ok(missing);
    updateNativeSnapshot({ markets: [missing] });
    const unavailable = nativeMarketRows().find(
      (item) => item.symbol === row.symbol,
    );
    assert.ok(unavailable);
    assert.equal(unavailable.next_funding_time, null);
    assert.equal(unavailable.last_funding_rate_hourly, null);
    assert.deepEqual(fundingRateDisplay(unavailable, now), {
      est_funding_rate: null,
      countDown: "00:00:00",
    });
  } finally {
    updateNativeSnapshot({ markets: previous });
  }
}

async function tickerSnapshotsHaveNoNextFundingTime() {
  const { normalizeFeedMarkets } = await fundingModules();
  const ticker: FeedTicker = {
    paxeerSymbol: "BTC",
    symbol: "PERP_BTC_USDC",
    mark_price: 110,
    index_price: 110,
    price_24h: 100,
    open_interest: 220,
    last_funding_rate: 0.0000125,
    last_funding_rate_hourly: "0.0000125",
    updated_at: now,
    "24h_open": 100,
    "24h_close": 110,
    "24h_high": null,
    "24h_low": null,
    "24h_amount": 5,
    "24h_volume": 550,
  };
  const [market] = normalizeFeedMarkets([ticker], now);
  assert.ok(market);
  assert.equal(market.perp_next_funding_ms, null);
  assert.equal(market.last_funding_rate_hourly, 12_500_000_000_000n);
}

async function fundingFigureAndCountdownUseIndependentMarketFields() {
  const { fundingRateDisplay } = await fundingModules();
  const rows = [
    {
      est_funding_rate: 0.0000123456789,
      last_funding_rate_hourly: -12_345_678_900_000n,
    },
    { last_funding_rate_hourly: -12_345_678_900_000n },
    { last_funding_rate_hourly: null },
  ];
  const figures = ["0.0012", "-0.0012", null];
  for (const [index, row] of rows.entries()) {
    assert.deepEqual(
      fundingRateDisplay({ ...row, next_funding_time: next }, now),
      {
        est_funding_rate: figures[index],
        countDown: "01:02:03",
      },
    );
    assert.deepEqual(
      fundingRateDisplay({ ...row, next_funding_time: now - 1 }, now),
      {
        est_funding_rate: index === 0 ? null : figures[index],
        countDown: "00:00:00",
      },
    );
    for (const next_funding_time of [null, undefined]) {
      assert.deepEqual(fundingRateDisplay({ ...row, next_funding_time }, now), {
        est_funding_rate: figures[index],
        countDown: "00:00:00",
      });
    }
    assert.deepEqual(
      fundingRateDisplay({ ...row, next_funding_time: now }, now),
      {
        est_funding_rate: figures[index],
        countDown: "00:00:00",
      },
    );
  }
  assert.deepEqual(fundingRateDisplay(undefined, now), {
    est_funding_rate: null,
    countDown: "00:00:00",
  });
  assert.deepEqual(fundingRateDisplay(null, now), {
    est_funding_rate: null,
    countDown: "00:00:00",
  });
  assert.equal(
    fundingRateDisplay(
      {
        est_funding_rate: 0,
        last_funding_rate_hourly: -12_345_678_900_000n,
        next_funding_time: next,
      },
      now,
    ).est_funding_rate,
    "0.0000",
  );
  assert.equal(
    fundingRateDisplay(
      { est_funding_rate: null, last_funding_rate_hourly: 0n },
      now,
    ).est_funding_rate,
    "0.0000",
  );
  assert.equal(
    fundingRateDisplay({ last_funding_rate_hourly: 19_999_999_999_999n }, now)
      .est_funding_rate,
    "0.0019",
  );
  assert.equal(
    fundingRateDisplay(
      { est_funding_rate: -0.000019999999999999, next_funding_time: next },
      now,
    ).est_funding_rate,
    "-0.0019",
  );
  assert.equal(
    fundingRateDisplay({ last_funding_rate_hourly: -19_999_999_999_999n }, now)
      .est_funding_rate,
    "-0.0019",
  );
  assert.equal(
    fundingRateDisplay({ last_funding_rate_hourly: 1n }, now).est_funding_rate,
    "0.0000",
  );
  assert.equal(
    fundingRateDisplay({ last_funding_rate_hourly: -1n }, now).est_funding_rate,
    "-0.0000",
  );
  assert.equal(
    fundingRateDisplay({ next_funding_time: now + 999 }, now).countDown,
    "00:00:00",
  );
}

test(marketFundingTimeIsNullableWithoutDroppingTheRecord);
test(nativeRowsCarryTheMarketTimeAndSettledHourlyRate);
test(tickerSnapshotsHaveNoNextFundingTime);
test(fundingFigureAndCountdownUseIndependentMarketFields);
