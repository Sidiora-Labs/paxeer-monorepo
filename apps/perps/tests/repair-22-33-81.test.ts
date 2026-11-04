import assert from "node:assert/strict";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { after, before, test } from "node:test";
import {
  createPublicClient,
  decodeFunctionData,
  encodeFunctionData,
  encodeFunctionResult,
  http,
} from "viem";

import type { AccountState } from "../src/core/perps/calls";
import type { OrderFormState } from "../src/core/perps/orderPlan";
import type { MarginTier } from "../src/core/perps/marketConfig";
import type { ParsedPosition } from "../src/core/perps/pnlHelpers";

const WAD = 10n ** 18n;
const ACCOUNT = "0x0000000000000000000000000000000000000022";
const PERPS = "0x0000000000000000000000000000000000001015";
const tiers: readonly MarginTier[] = [
  { lowerBoundUsd: 0n, maxLeverage: 20n * WAD, maintenanceBps: 500n, deductionUsd: 0n },
];

// ABI account snapshots: one position, another position's loss, deposit,
// withdrawal, below maintenance, exact maintenance, and negative equity.
const snapshots: readonly AccountState[] = [
  {
    collateral: 1_000_000_000n,
    equityUsd: 1000n * WAD,
    initialMarginUsd: 100n * WAD,
    maintenanceMarginUsd: 50n * WAD,
    withdrawable: 900_000_000n,
    feeTier: 0,
    volume14dUsd: 0n,
    positionCount: 1n,
    openOrderCount: 0n,
  },
  {
    collateral: 1_000_000_000n,
    equityUsd: 800n * WAD,
    initialMarginUsd: 200n * WAD,
    maintenanceMarginUsd: 150n * WAD,
    withdrawable: 600_000_000n,
    feeTier: 0,
    volume14dUsd: 0n,
    positionCount: 2n,
    openOrderCount: 0n,
  },
  {
    collateral: 1_500_000_000n,
    equityUsd: 1300n * WAD,
    initialMarginUsd: 200n * WAD,
    maintenanceMarginUsd: 150n * WAD,
    withdrawable: 1_100_000_000n,
    feeTier: 0,
    volume14dUsd: 0n,
    positionCount: 2n,
    openOrderCount: 0n,
  },
  {
    collateral: 500_000_000n,
    equityUsd: 300n * WAD,
    initialMarginUsd: 200n * WAD,
    maintenanceMarginUsd: 150n * WAD,
    withdrawable: 100_000_000n,
    feeTier: 0,
    volume14dUsd: 0n,
    positionCount: 2n,
    openOrderCount: 0n,
  },
  {
    collateral: 500_000_000n,
    equityUsd: 149n * WAD,
    initialMarginUsd: 200n * WAD,
    maintenanceMarginUsd: 150n * WAD,
    withdrawable: 0n,
    feeTier: 0,
    volume14dUsd: 0n,
    positionCount: 2n,
    openOrderCount: 0n,
  },
  {
    collateral: 500_000_000n,
    equityUsd: 150n * WAD,
    initialMarginUsd: 200n * WAD,
    maintenanceMarginUsd: 150n * WAD,
    withdrawable: 0n,
    feeTier: 0,
    volume14dUsd: 0n,
    positionCount: 2n,
    openOrderCount: 0n,
  },
  {
    collateral: 500_000_000n,
    equityUsd: -25n * WAD,
    initialMarginUsd: 200n * WAD,
    maintenanceMarginUsd: 150n * WAD,
    withdrawable: 0n,
    feeTier: 0,
    volume14dUsd: 0n,
    positionCount: 2n,
    openOrderCount: 0n,
  },
];

let abi: typeof import("../src/core/network/abi/PerpsPrecompile").PerpsPrecompileABI;
let origin: string;
let nextSnapshot = 0;
const requests: unknown[] = [];
const server = createServer(async (request, response) => {
  try {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const rpc = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    requests.push({ method: rpc.method, params: rpc.params });
    assert.equal(rpc.method, "eth_call");
    assert.equal(rpc.params[0].to.toLowerCase(), PERPS);
    assert.equal(rpc.params[0].data, encodeFunctionData({ abi, functionName: "getAccount", args: [ACCOUNT] }));
    assert.equal(rpc.params[1], "latest");
    const snapshot = snapshots[nextSnapshot++];
    assert.ok(snapshot, "every RPC read must consume a declared account snapshot");
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify({
      jsonrpc: "2.0",
      id: rpc.id,
      result: encodeFunctionResult({ abi, functionName: "getAccount", result: snapshot }),
    }));
  } catch (error) {
    response.statusCode = 500;
    response.end(String(error));
  }
});

before(async function startAccountTranscript() {
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = origin;
  process.env.NEXT_PUBLIC_PAXSCAN_API_URL = `${origin}/api/v2`;
  process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = `${origin.replace("http:", "ws:")}/socket`;
  ({ PerpsPrecompileABI: abi } = await import("../src/core/network/abi/PerpsPrecompile"));
});

after(async function stopAccountTranscript() {
  await new Promise<void>((resolve, reject) => server.close((error) => (error ? reject(error) : resolve())));
});

function protectedLadder(overrides: Partial<OrderFormState> = {}): OrderFormState {
  return {
    orderType: "scaled", marketId: 1n, isLong: true,
    collateralAmount: "", leverage: "10", triggerPrice: "", limitPrice: "", sizeUsd: "6000",
    scaledStartPrice: "90", scaledEndPrice: "110", scaledTotalOrders: "3", scaledDistribution: "flat",
    tpslEnabled: true, tpPrice: "120", slPrice: "80", reduceOnly: false,
    postOnly: false, ioc: false, fok: false, ladderId: 0x223381n,
    ...overrides,
  };
}

async function scaledOrderProtectionSurvivesPlaceOrderCalldata() {
  const { buildOrderPlan } = await import("../src/core/perps/orderPlan");
  const { ladderCloid } = await import("../src/core/perps/calls");
  const { planOrder } = await import("../src/core/perps/positionWrites");
  const limits = { tickSize: WAD / 2n, lotSize: 1n, maxLeverage: 20n * WAD };
  for (const isLong of [true, false]) {
    for (const scaledDistribution of ["flat", "asc", "desc"] as const) {
      for (const reverse of [false, true]) {
        const state = protectedLadder({
          isLong, scaledDistribution,
          scaledStartPrice: reverse ? "110" : "90",
          scaledEndPrice: reverse ? "90" : "110",
          tpPrice: isLong ? "120" : "80", slPrice: isLong ? "80" : "120",
        });
        const plan = buildOrderPlan(state, null, limits);
        assert.equal(plan.error, null);
        const writes = await planOrder(plan.calls);
        assert.equal(writes.length, 3);
        const rungs = writes.map((call, i) => {
          assert.ok(call.functionName === "placeOrder", "a ladder submits only placeOrder calls");
          assert.equal(call.address.toLowerCase(), PERPS);
          const decoded = decodeFunctionData({ abi: call.abi, data: encodeFunctionData(call) });
          assert.equal(decoded.functionName, "placeOrder");
          if (decoded.functionName !== "placeOrder") throw new Error("Expected placeOrder calldata");
          const [rung] = decoded.args;
          assert.equal(rung.takeProfitPrice, (isLong ? 120n : 80n) * WAD);
          assert.equal(rung.stopLossPrice, (isLong ? 80n : 120n) * WAD);
          assert.equal(rung.isLong, isLong);
          assert.equal(rung.cloid, ladderCloid(state.ladderId, i));
          assert.equal(rung.orderType, 0);
          assert.equal(rung.limitPrice, BigInt(reverse ? 110 - 10 * i : 90 + 10 * i) * WAD);
          return rung;
        });
        assert.equal(rungs.reduce((total, rung) => total + rung.sizeUsd, 0n), 6000n * WAD);
      }
    }
  }
  for (const override of [
    { tpPrice: "105" }, { slPrice: "95" },
    { isLong: false, tpPrice: "95", slPrice: "120" },
    { isLong: false, tpPrice: "80", slPrice: "105" },
  ]) {
    const refused = buildOrderPlan(protectedLadder(override), null, limits);
    assert.equal(refused.error?.code, "TRADE_TP_SL_INVALID");
    assert.deepEqual(refused.calls, [], "a protection invalid on any rung refuses the entire ladder");
  }
  for (const override of [{ tpPrice: "" }, { slPrice: "" }, { tpslEnabled: false }]) {
    const state = protectedLadder(override);
    const plan = buildOrderPlan(state, null, limits);
    assert.equal(plan.error, null);
    assert.equal(plan.calls.length, 3);
    for (const call of plan.calls) {
      const decoded = decodeFunctionData({ abi: call.abi, data: encodeFunctionData(call) });
      if (decoded.functionName !== "placeOrder") throw new Error("Expected placeOrder calldata");
      assert.equal(decoded.args[0].takeProfitPrice, state.tpslEnabled && state.tpPrice !== "" ? 120n * WAD : 0n);
      assert.equal(decoded.args[0].stopLossPrice, state.tpslEnabled && state.slPrice !== "" ? 80n * WAD : 0n);
    }
  }
}

test(scaledOrderProtectionSurvivesPlaceOrderCalldata);

async function crossMarginRiskUsesAccountSnapshotAcrossPositionsAndCollateral() {
  const { accountViewFromState } = await import("../src/core/perps/accountView");
  const { buildGetAccount } = await import("../src/core/perps/calls");
  const { computePositionPnl } = await import("../src/core/perps/pnlHelpers");
  const client = createPublicClient({ transport: http(origin, { retryCount: 0 }) });
  const position: ParsedPosition = {
    positionId: 1n, marketId: 1, symbol: "BTC", isLong: true,
    sizeUsd: 1000n * WAD, collateralUsd: 100n * WAD, leverage: 10n * WAD,
    entryPrice: 100n * WAD, collateralToken: "", collateralAmount: 100_000_000n,
    fundingPaidUsd: 0n, feesUsd: 0n, openedAt: undefined, openTxHash: undefined,
  };
  const baselineEstimate = computePositionPnl(position, position.entryPrice, tiers);
  assert.equal(baselineEstimate.accountRisk, null, "missing account state must remain explicitly unavailable");
  assert.equal(baselineEstimate.estimateScope, "position-only");
  assert.equal(baselineEstimate.liquidationPriceLabel, "Position-only liquidation estimate");
  const risks = [];
  for (const snapshot of snapshots) {
    const state = await client.readContract(buildGetAccount(ACCOUNT));
    assert.deepEqual(state, snapshot, "signed equity and all margin fields survive the ABI transport");
    const account = accountViewFromState(state);
    const pnl = computePositionPnl(position, position.entryPrice, tiers, account);
    const risk = pnl.accountRisk;
    assert.ok(risk);
    assert.equal(risk.display.label, "Cross-margin account");
    assert.equal(risk.equityUsd, snapshot.equityUsd);
    assert.equal(risk.initialMarginUsd, snapshot.initialMarginUsd);
    assert.equal(risk.maintenanceMarginUsd, snapshot.maintenanceMarginUsd);
    assert.equal(pnl.liqPrice, baselineEstimate.liqPrice, "position estimate stays separate from changing account risk");
    assert.equal(pnl.remainingCollateral, baselineEstimate.remainingCollateral);
    assert.equal("liqPrice" in risk, false, "account risk cannot imply one market liquidation price");
    assert.deepEqual(computePositionPnl(position, 0n, tiers, account).accountRisk, risk);
    risks.push(risk);
  }
  assert.equal(requests.length, snapshots.length);
  assert.equal(nextSnapshot, snapshots.length);
  assert.deepEqual(risks.slice(0, 4).map((risk) => risk.headroomUsd), [950n, 650n, 1150n, 150n].map((n) => n * WAD));
  assert.deepEqual(risks.slice(0, 4).map((risk) => risk.display.maintenanceUsage), ["5%", "18.75%", "11.53%", "50%"]);
  assert.deepEqual(risks.slice(0, 4).map((risk) => risk.display.headroom), ["950 USD", "650 USD", "1150 USD", "150 USD"]);
  assert.deepEqual(risks.slice(0, 4).map((risk) => risk.display.equity), ["1000 USD", "800 USD", "1300 USD", "300 USD"]);
  assert.equal(risks[4].status, "below-maintenance");
  assert.equal(risks[4].headroomUsd, -WAD);
  assert.equal(risks[5].status, "at-maintenance");
  assert.equal(risks[5].display.maintenanceUsage, "100%");
  assert.equal(risks[6].status, "below-maintenance");
  assert.equal(risks[6].display.equity, "-25 USD");
  assert.equal(risks[6].headroomUsd, -175n * WAD);
  assert.equal(risks[6].maintenanceUsageBps, null);
  assert.equal(risks[6].display.maintenanceUsage, "Unavailable (non-positive equity)");
}

test(crossMarginRiskUsesAccountSnapshotAcrossPositionsAndCollateral);
