import { describe, it, test } from "node:test";
import { strict as assert } from "node:assert";

async function loadConfig() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://gateway.example.test";
  return import("../src/core/perps/config");
}

async function configHasNoSpotDesignExports() {
  const config = await loadConfig();
  const { PERPS_MARKETS } = config;
  const exported = Object.keys(config);
  for (const name of [
    "SPOT_MODE_LABELS",
    "POFQ_TIER_LABELS",
    "SPOT_MODE_BANNER_COPY",
  ]) {
    assert.equal(exported.includes(name), false, `${name} is exported`);
  }

  assert.equal(PERPS_MARKETS.length, 18);
  assert.deepEqual(
    PERPS_MARKETS.map((market) => market.marketId),
    Array.from({ length: 18 }, (_, i) => i),
  );
  for (const market of PERPS_MARKETS) {
    assert.equal("maxLeverage" in market, false, `${market.symbol} maxLeverage`);
  }
}

test(configHasNoSpotDesignExports);

describe("perps configuration", () => {
  it("marketIdsAreSequential", async () => {
    const { PERPS_MARKETS } = await loadConfig();
    assert.equal(PERPS_MARKETS.length, 18);
    assert.deepEqual(
      PERPS_MARKETS.map((m) => m.marketId),
      Array.from({ length: 18 }, (_, i) => i),
    );
    const link = PERPS_MARKETS.find((m) => m.symbol === "LINK");
    assert.equal(link?.marketId, 4);
  });

  it("enumLabelsFollowPerpsNumbering", async () => {
    const {
      FEE_TYPE_LABELS,
      ORDER_TYPE_COLORS,
      ORDER_TYPE_LABELS,
      TRADE_TYPE_LABELS,
    } = await loadConfig();
    assert.deepEqual(ORDER_TYPE_LABELS, {
      0: "Limit",
      1: "Stop-Limit",
      2: "Take Profit",
      3: "Stop Loss",
      4: "Market",
      5: "Stop Market",
    });
    assert.deepEqual(
      Object.keys(ORDER_TYPE_COLORS),
      Object.keys(ORDER_TYPE_LABELS),
    );
    assert.equal(ORDER_TYPE_COLORS[0], "text-blue-400");
    assert.equal(ORDER_TYPE_COLORS[1], "text-amber-400");
    assert.equal(ORDER_TYPE_COLORS[2], "text-emerald-500");
    assert.equal(ORDER_TYPE_COLORS[3], "text-red-500");
    assert.equal(ORDER_TYPE_COLORS[4], "text-zinc-300");
    assert.equal(ORDER_TYPE_COLORS[5], "text-orange-400");
    assert.deepEqual(TRADE_TYPE_LABELS, {
      0: "Open",
      1: "Close",
      2: "Liquidation",
      3: "PartialClose",
      4: "Increase",
      5: "ADL",
      6: "Backstop",
    });
    assert.deepEqual(FEE_TYPE_LABELS, {
      0: "Trading",
      1: "Borrowing",
      2: "Funding",
      3: "Liquidation",
      4: "Maker Rebate",
    });
  });

  it("diamondSpotTablesRemoved", async () => {
    const config = await loadConfig();
    const { POSITION_ACTION_COPY } = config;
    const exported = Object.keys(config);
    for (const name of [
      "SPOT_TOKENS",
      "SPOT_MARKETS_META",
      "getPerpsMarketIdForSpot",
      "getSpotMarketMeta",
    ]) {
      assert.equal(exported.includes(name), false, `${name} is exported`);
    }
    const copyKeys = Object.keys(POSITION_ACTION_COPY);
    for (const key of [
      "transferMargin",
      "successTransferMargin",
      "errorTransferMargin",
    ]) {
      assert.equal(copyKeys.includes(key), false, `${key} copy remains`);
    }
  });
});
