import { strict as assert } from "node:assert";
import { test } from "node:test";

import type { OrderFormState, OrderLimits, OrderPlan } from "../src/core/perps/orderPlan";
import type { OpenPositionQuote } from "../src/hooks/perps/useQuoter";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://perps-gateway.invalid";

const E18 = 10n ** 18n;
const LIMITS: OrderLimits = { tickSize: E18 / 2n, lotSize: E18, maxLeverage: 20n * E18 };

function form(overrides: Partial<OrderFormState> = {}): OrderFormState {
  return {
    orderType: "limit",
    marketId: 0n,
    isLong: true,
    collateralAmount: "100",
    leverage: "10",
    triggerPrice: "100.3",
    limitPrice: "100.5",
    sizeUsd: "1000",
    scaledStartPrice: "100.3",
    scaledEndPrice: "101.1",
    scaledTotalOrders: "3",
    scaledDistribution: "flat",
    tpslEnabled: false,
    tpPrice: "",
    slPrice: "",
    reduceOnly: false,
    postOnly: false,
    ioc: false,
    fok: false,
    ladderId: 1n,
    ...overrides,
  };
}

function refusedWith(plan: OrderPlan | null, code: string): void {
  assert.ok(plan);
  assert.equal(plan.error?.code, code);
  assert.deepEqual(plan.calls, []);
}

async function offTickPriceRefused(): Promise<void> {
  const { buildOrderPlan, checkOrderLimits } = await import("../src/core/perps/orderPlan");
  refusedWith(checkOrderLimits(form(), LIMITS), "VALIDATION_PRECISION");
  refusedWith(buildOrderPlan(form(), null, LIMITS), "VALIDATION_PRECISION");

  const aligned = form({ triggerPrice: "100.5" });
  assert.equal(checkOrderLimits(aligned, LIMITS), null);
  const accepted = buildOrderPlan(aligned, null, LIMITS);
  assert.equal(accepted.error, null);
  assert.equal(accepted.calls.length, 1);
  assert.equal(accepted.calls[0].args[0].limitPrice, 1005n * E18 / 10n);

  const offLot = { ...aligned, sizeUsd: "1000.5" };
  refusedWith(checkOrderLimits(offLot, LIMITS), "VALIDATION_PRECISION");
  refusedWith(buildOrderPlan(offLot, null, LIMITS), "VALIDATION_PRECISION");

  for (const prices of [{ triggerPrice: "100.3", limitPrice: "100.5" }, { triggerPrice: "100.5", limitPrice: "100.3" }]) {
    const stop = form({ orderType: "stop-limit", ...prices });
    refusedWith(checkOrderLimits(stop, LIMITS), "VALIDATION_PRECISION");
    refusedWith(buildOrderPlan(stop, null, LIMITS), "VALIDATION_PRECISION");
  }

  const market = form({ orderType: "market", sizeUsd: "" });
  const quote: OpenPositionQuote = {
    entryPrice: 100n * E18,
    sizeUsd: 10005n * E18 / 10n,
    collateralUsd: 100n * E18,
    leverage: 10n * E18,
    tradingFee: 0n,
    tradingFeeUsd: 0n,
    priceImpact: 0n,
    liquidationPrice: 90n * E18,
    estimatedFunding24h: 0n,
    maintenanceMarginBps: 5n,
  };
  refusedWith(buildOrderPlan(market, quote, LIMITS), "VALIDATION_PRECISION");
  const marketPlan = buildOrderPlan(market, { ...quote, sizeUsd: 1000n * E18 }, LIMITS);
  assert.equal(marketPlan.error, null);
  assert.equal(marketPlan.calls.length, 1);
  assert.equal(marketPlan.calls[0].args[0].sizeUsd, 1000n * E18);
}

async function leverageAboveChainMaxRefused(): Promise<void> {
  const { buildOrderPlan, checkOrderLimits, parseLeverage } = await import("../src/core/perps/orderPlan");
  assert.equal(parseLeverage("25"), 25n * E18);
  assert.ok(parseLeverage("25") > LIMITS.maxLeverage);
  for (const orderType of ["limit", "stop-limit", "scaled", "market"] as const) {
    const state = form({ orderType, triggerPrice: "100.5", leverage: "25" });
    refusedWith(checkOrderLimits(state, LIMITS), "VALIDATION_ABOVE_MAX");
    refusedWith(buildOrderPlan(state, null, LIMITS), "VALIDATION_ABOVE_MAX");
  }
  const atMax = form({ triggerPrice: "100.5", leverage: "20" });
  assert.equal(checkOrderLimits(atMax, LIMITS), null);
  const plan = buildOrderPlan(atMax, null, LIMITS);
  assert.equal(plan.error, null);
  assert.equal(plan.calls.length, 1);
  assert.equal(plan.calls[0].args[0].leverage, LIMITS.maxLeverage);
  const aboveByOne = form({ triggerPrice: "100.5", leverage: "20.000000000000000001" });
  refusedWith(checkOrderLimits(aboveByOne, LIMITS), "VALIDATION_ABOVE_MAX");
  refusedWith(buildOrderPlan(aboveByOne, null, LIMITS), "VALIDATION_ABOVE_MAX");
}

async function alignToTickRoundsBySide(): Promise<void> {
  const { alignToTick, buildOrderPlan } = await import("../src/core/perps/orderPlan");
  const price = 1003n * E18 / 10n;
  assert.equal(alignToTick(price, LIMITS.tickSize, "down"), 100n * E18);
  assert.equal(alignToTick(price, LIMITS.tickSize, "up"), 1005n * E18 / 10n);
  for (const side of ["down", "up"] as const) {
    assert.equal(alignToTick(100n * E18, LIMITS.tickSize, side), 100n * E18);
    assert.equal(alignToTick(0n, LIMITS.tickSize, side), 0n);
    assert.throws(() => alignToTick(price, 0n, side), RangeError);
    assert.throws(() => alignToTick(price, -1n, side), RangeError);
  }

  for (const isLong of [true, false]) {
    for (const scaledDistribution of ["flat", "asc", "desc"] as const) {
      for (const reverse of [false, true]) {
        const state = form({
          orderType: "scaled",
          isLong,
          sizeUsd: "600",
          scaledDistribution,
          scaledStartPrice: reverse ? "101.1" : "100.3",
          scaledEndPrice: reverse ? "100.3" : "101.1",
        });
        const plan = buildOrderPlan(state, null, LIMITS);
        assert.equal(plan.error, null);
        assert.equal(plan.calls.length, 3);
        const expected = isLong
          ? [100n * E18, 1005n * E18 / 10n, 101n * E18]
          : [1005n * E18 / 10n, 101n * E18, 1015n * E18 / 10n];
        assert.deepEqual(plan.calls.map((call) => call.args[0].limitPrice), reverse ? expected.reverse() : expected);
        for (const call of plan.calls) {
          assert.equal(call.args[0].limitPrice % LIMITS.tickSize, 0n);
          assert.equal(call.args[0].sizeUsd % LIMITS.lotSize, 0n);
        }
        assert.equal(plan.calls.reduce((sum, call) => sum + call.args[0].sizeUsd, 0n), 600n * E18);
      }
    }
  }
  refusedWith(buildOrderPlan(form({ orderType: "scaled", sizeUsd: "100" }), null, LIMITS), "VALIDATION_PRECISION");
}

test(offTickPriceRefused);
test(leverageAboveChainMaxRefused);
test(alignToTickRoundsBySide);
