import { afterEach, beforeEach, describe, it, mock, test } from "node:test";
import { strict as assert } from "node:assert";
import type { Address } from "viem";

import type { AllowanceSource, PositionTuple } from "../src/core/perps/positionWrites";
import type { AddSizeInput, LimitOrderInput, OpenPositionInput } from "../src/core/perps/calls";
import { OrderType, TimeInForce } from "../src/core/network/abi/PerpsPrecompile";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";

const {
  planAddSize,
  planClose,
  planCollateral,
  planOpen,
  planOrder,
  planPartialClose,
  positionFromTuple,
} = require("../src/core/perps/positionWrites") as typeof import("../src/core/perps/positionWrites");
const {
  buildAddCollateral,
  buildAddSize,
  buildClosePosition,
  buildOpenPosition,
  buildPartialClose,
  buildPlaceLimitOrder,
  buildRemoveCollateral,
  ladderCloid,
} = require("../src/core/perps/calls") as typeof import("../src/core/perps/calls");

const PERPS: Address = "0x0000000000000000000000000000000000001015";
const USDL: Address = "0x85FcD13735F4309833A503EE804ea32395851479";
const ALICE: Address = "0x00000000000000000000000000000000000A11CE";
const E18 = 10n ** 18n;
const REFERENCE_SECONDS = 1_000_000;
const NO_NATIVE = new Set<Address>();

/** An allowance source that counts its reads. */
function source(allowance: bigint) {
  const reads = { count: 0 };
  const src: AllowanceSource = {
    allowance: async () => {
      reads.count += 1;
      return allowance;
    },
    native: NO_NATIVE,
  };
  return { src, reads };
}

const OPEN: OpenPositionInput = {
  marketId: 0n,
  isLong: true,
  collateralAmount: 100_000_000n,
  leverage: 10n * E18,
  quote: { entryPrice: 60_000n * E18, sizeUsd: 1_000n * E18 },
  takeProfitPrice: 66_000n * E18,
  stopLossPrice: 57_000n * E18,
};

const ADD_SIZE: AddSizeInput = {
  position: { marketId: 1n, isLong: false },
  additionalCollateral: 25_000_000n,
  leverage: 20n * E18,
  quote: { entryPrice: 3_000n * E18, sizeUsd: 500n * E18 },
};

function limit(overrides: Partial<LimitOrderInput> = {}): LimitOrderInput {
  return {
    marketId: 0n,
    isLong: true,
    limitPrice: 59_000n * E18,
    sizeUsd: 6_000n * E18,
    leverage: 20n * E18,
    collateralAmount: 300_000_000n,
    timeInForce: TimeInForce.GTC,
    reduceOnly: false,
    cloid: ladderCloid(7n, 0),
    ...overrides,
  };
}

function approve(amount: bigint) {
  return { address: USDL, functionName: "approve", args: [PERPS, amount] };
}

beforeEach(() => {
  mock.method(Date, "now", () => REFERENCE_SECONDS * 1000);
});

afterEach(() => {
  mock.restoreAll();
});

async function planOpenIsThePlaceOrderAlone() {
  const plan = await planOpen(OPEN);
  assert.equal(plan.some((step) => step.functionName === "approve"), false);
  assert.deepEqual(plan, [buildOpenPosition(OPEN)]);
  const step = plan[0];
  assert.ok(step.functionName === "placeOrder");
  assert.equal(step.address, PERPS);
  const p = step.args[0];
  assert.equal(p.orderType, OrderType.MARKET);
  assert.equal(p.timeInForce, TimeInForce.IOC);
  assert.equal(p.collateralAmount, 100_000_000n);
  assert.equal(p.acceptablePrice, 60_300n * E18);
  assert.equal(p.deadline, BigInt(REFERENCE_SECONDS + 60));
  assert.equal(p.takeProfitPrice, 66_000n * E18);
  assert.equal(p.stopLossPrice, 57_000n * E18);
  assert.equal(p.marketId, OPEN.marketId);
  assert.equal(p.isLong, OPEN.isLong);
  assert.equal(p.sizeUsd, OPEN.quote.sizeUsd);
  assert.equal(p.leverage, OPEN.leverage);
  assert.equal(p.limitPrice, 0n);
  assert.equal(p.triggerPrice, 0n);
  assert.equal(p.reduceOnly, false);
  assert.equal(p.cloid, "0x00000000000000000000000000000000");
}
test(planOpenIsThePlaceOrderAlone);

async function planAddSizeIsThePlaceOrderAlone() {
  const plan = await planAddSize(ADD_SIZE);
  assert.equal(plan.some((step) => step.functionName === "approve"), false);
  assert.deepEqual(plan, [buildAddSize(ADD_SIZE)]);
  const step = plan[0];
  assert.ok(step.functionName === "placeOrder");
  assert.equal(step.address, PERPS);
  const p = step.args[0];
  assert.equal(p.marketId, 1n);
  assert.equal(p.isLong, false);
  assert.equal(p.orderType, OrderType.MARKET);
  assert.equal(p.timeInForce, TimeInForce.IOC);
  assert.equal(p.acceptablePrice, 2_985n * E18);
  assert.equal(p.collateralAmount, 25_000_000n);
  assert.equal(p.sizeUsd, ADD_SIZE.quote.sizeUsd);
  assert.equal(p.leverage, ADD_SIZE.leverage);
  assert.equal(p.limitPrice, 0n);
  assert.equal(p.triggerPrice, 0n);
  assert.equal(p.reduceOnly, false);
  assert.equal(p.takeProfitPrice, 0n);
  assert.equal(p.stopLossPrice, 0n);
  assert.equal(p.deadline, BigInt(REFERENCE_SECONDS + 60));
  assert.equal(p.cloid, "0x00000000000000000000000000000000");
}
test(planAddSizeIsThePlaceOrderAlone);

async function planOrderIsTheLadderAlone() {
  const rungs = [
    buildPlaceLimitOrder(limit({ collateralAmount: 100_000_000n, cloid: ladderCloid(7n, 0) })),
    buildPlaceLimitOrder(limit({ collateralAmount: 150_000_000n, cloid: ladderCloid(7n, 1) })),
    buildPlaceLimitOrder(limit({ collateralAmount: 50_000_001n, cloid: ladderCloid(7n, 2) })),
  ];
  const plan = await planOrder(rungs);
  assert.equal(plan.some((step) => step.functionName === "approve"), false);
  assert.deepEqual(plan, rungs);
  for (const [index, step] of plan.entries()) {
    assert.ok(step.functionName === "placeOrder");
    assert.equal(step.address, PERPS);
    assert.deepEqual(step.args[0], rungs[index].args[0]);
    const p = step.args[0];
    assert.equal(p.marketId, 0n);
    assert.equal(p.isLong, true);
    assert.equal(p.orderType, OrderType.LIMIT);
    assert.equal(p.timeInForce, TimeInForce.GTC);
    assert.equal(p.sizeUsd, 6_000n * E18);
    assert.equal(p.leverage, 20n * E18);
    assert.equal(p.limitPrice, 59_000n * E18);
    assert.equal(p.triggerPrice, 0n);
    assert.equal(p.acceptablePrice, 0n);
    assert.equal(p.collateralAmount, [100_000_000n, 150_000_000n, 50_000_001n][index]);
    assert.equal(p.reduceOnly, false);
    assert.equal(p.takeProfitPrice, 0n);
    assert.equal(p.stopLossPrice, 0n);
    assert.equal(p.deadline, 0n);
    assert.equal(p.cloid, ladderCloid(7n, index));
  }
}
test(planOrderIsTheLadderAlone);

describe("position write planners", () => {
  it("planCloseSendsClosePositionWithParamsAndNeverApproves", () => {
    const input = { positionId: 7n, isLong: true, exitPrice: 61_000n * E18 };
    const plan = planClose(input);
    assert.deepEqual(plan, [buildClosePosition(input)]);
    assert.equal(plan[0].address, PERPS);
    assert.equal(plan[0].functionName, "closePositionWithParams");
    assert.deepEqual(plan[0].args, [7n, 0n, 60_695n * E18, BigInt(REFERENCE_SECONDS + 60)]);
  });

  it("planPartialCloseBoundsTheChosenSize", () => {
    const input = { positionId: 9n, isLong: false, closeSizeUsd: 250n * E18, exitPrice: 3_000n * E18 };
    const plan = planPartialClose(input);
    assert.deepEqual(plan, [buildPartialClose(input)]);
    assert.deepEqual(plan[0].args, [9n, 250n * E18, 3_015n * E18, BigInt(REFERENCE_SECONDS + 60)]);
  });

  it("planCollateralApprovesAddAndNeverRemove", async () => {
    const add = await planCollateral({ action: "add", positionId: 7n, amount: 20_000_000n }, source(5n).src);
    assert.deepEqual(add, [approve(20_000_000n), buildAddCollateral(7n, 20_000_000n)]);
    const enough = await planCollateral({ action: "add", positionId: 7n, amount: 20_000_000n }, source(20_000_000n).src);
    assert.deepEqual(enough, [buildAddCollateral(7n, 20_000_000n)]);
    const { src, reads } = source(0n);
    const remove = await planCollateral({ action: "remove", positionId: 7n, amount: 20_000_000n }, src);
    assert.deepEqual(remove, [buildRemoveCollateral(7n, 20_000_000n)]);
    assert.equal(reads.count, 0);
  });

  it("noApprovalExceedsTheCallAmount", async () => {
    for (const plan of [
      await planOpen(OPEN),
      await planAddSize(ADD_SIZE),
      await planCollateral({ action: "add", positionId: 1n, amount: 1n }, source(0n).src),
    ]) {
      const approvals = plan.filter((step) => step.functionName === "approve");
      const call = plan[plan.length - 1];
      assert.ok(call.functionName !== "approve");
      if (call.functionName === "addCollateral") {
        assert.deepEqual(approvals, [approve(call.args[1])]);
        assert.equal(plan.length, 2);
      } else {
        assert.equal(call.functionName, "placeOrder");
        assert.deepEqual(approvals, []);
        assert.equal(plan.length, 1);
      }
    }
  });

  it("refusesBeforeReadingTheAllowance", async () => {
    const { src, reads } = source(0n);
    await assert.rejects(
      () => planOpen({ ...OPEN, collateralAmount: 0n }),
      (err: { code?: string }) => err.code === "VALIDATION_AMOUNT_INVALID",
    );
    await assert.rejects(
      () => planOpen({ ...OPEN, takeProfitPrice: 59_000n * E18 }),
      (err: { code?: string }) => err.code === "TRADE_TP_SL_INVALID",
    );
    await assert.rejects(
      () => planCollateral({ action: "add", positionId: 1n, amount: 0n }, src),
      (err: { code?: string }) => err.code === "VALIDATION_AMOUNT_INVALID",
    );
    assert.equal(reads.count, 0);
  });
});

describe("getPosition by name", () => {
  it("readsCollateralTokenByOutputName", () => {
    const tuple: PositionTuple = [
      ALICE,
      3n,
      true,
      1_000n * E18,
      100n * E18,
      USDL,
      100_000_000n,
      60_000n * E18,
      1_700_000_000n,
      true,
    ];
    const position = positionFromTuple(tuple);
    assert.equal(position.collateralToken, USDL);
    assert.equal(position.user, ALICE);
    assert.equal(position.marketId, 3n);
    assert.equal(position.isLong, true);
    assert.equal(position.sizeUsd, 1_000n * E18);
    assert.equal(position.collateralAmount, 100_000_000n);
    assert.equal(position.entryPrice, 60_000n * E18);
    assert.equal(position.active, true);
  });
});
