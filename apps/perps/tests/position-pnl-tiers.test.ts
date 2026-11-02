import { strict as assert } from "node:assert";
import test from "node:test";

import type { MarginTier } from "../src/core/perps/marketConfig";
import {
  computePositionPnl,
  maintenanceMargin,
  WAD,
  type ParsedPosition,
} from "../src/core/perps/pnlHelpers";

function liquidationPriceUsesTiers() {
  const tiers: readonly MarginTier[] = [
    { lowerBoundUsd: 0n, maxLeverage: 1000n, maintenanceBps: 5n, deductionUsd: 0n },
    { lowerBoundUsd: 1_000_000n * WAD, maxLeverage: 20n, maintenanceBps: 250n, deductionUsd: 1000n * WAD },
    { lowerBoundUsd: 5_000_000n * WAD, maxLeverage: 10n, maintenanceBps: 500n, deductionUsd: 126_000n * WAD },
  ];
  const position: ParsedPosition = {
    positionId: 1n,
    marketId: 1,
    symbol: "BTC",
    isLong: true,
    sizeUsd: 2_000_000n * WAD,
    collateralUsd: 200_000n * WAD,
    leverage: 10n * WAD,
    entryPrice: 50_000n * WAD,
    collateralToken: "",
    collateralAmount: 0n,
    fundingPaidUsd: 0n,
    feesUsd: 0n,
    openedAt: undefined,
    openTxHash: undefined,
  };

  assert.equal(maintenanceMargin(position.sizeUsd, tiers), 49_000n * WAD);
  const long = computePositionPnl(position, position.entryPrice, tiers);
  assert.equal(long.initialCollateral, 200_000n * WAD);
  assert.equal(long.liqPrice, 46_225n * WAD);
  assert.notEqual(long.liqPrice, 45_500n * WAD);
  assert.equal(computePositionPnl(position, long.liqPrice, tiers).remainingCollateral, 49_000n * WAD);
  assert.equal(computePositionPnl(position, long.liqPrice, tiers).liqSafetyBps, 0n);
  assert.ok(computePositionPnl(position, long.liqPrice - WAD, tiers).remainingCollateral < 49_000n * WAD);
  assert.ok(computePositionPnl(position, long.liqPrice + WAD, tiers).remainingCollateral > 49_000n * WAD);

  const unsorted = [tiers[2], tiers[0], tiers[1]];
  assert.equal(computePositionPnl(position, position.entryPrice, unsorted).liqPrice, long.liqPrice);
  const boundaries = [
    [500_000n, 250n],
    [1_000_000n, 24_000n],
    [5_000_000n, 124_000n],
    [6_000_000n, 174_000n],
  ] as const;
  for (const [size, expectedMaintenance] of boundaries) {
    const pos = { ...position, sizeUsd: size * WAD };
    const pnl = computePositionPnl(pos, pos.entryPrice, tiers);
    assert.equal(maintenanceMargin(pos.sizeUsd, tiers), expectedMaintenance * WAD);
    assert.equal(computePositionPnl(pos, pnl.liqPrice, tiers).remainingCollateral, expectedMaintenance * WAD);
  }

  for (const isLong of [true, false]) {
    const pos = { ...position, isLong };
    const pnl = computePositionPnl(pos, pos.entryPrice, tiers);
    assert.equal(pnl.liqPrice, (isLong ? 46_225n : 53_775n) * WAD);
    assert.equal(computePositionPnl(pos, pnl.liqPrice, tiers).remainingCollateral, 49_000n * WAD);

    const charged = { ...pos, fundingPaidUsd: 3000n * WAD, feesUsd: 2000n * WAD };
    const chargedPnl = computePositionPnl(charged, charged.entryPrice, tiers);
    assert.equal(chargedPnl.liqPrice, (isLong ? 46_350n : 53_650n) * WAD);
    assert.equal(computePositionPnl(charged, chargedPnl.liqPrice, tiers).remainingCollateral, 49_000n * WAD);

    const received = { ...pos, fundingPaidUsd: -3000n * WAD };
    const receivedPnl = computePositionPnl(received, received.entryPrice, tiers);
    assert.equal(receivedPnl.liqPrice, (isLong ? 46_150n : 53_850n) * WAD);
    assert.equal(computePositionPnl(received, receivedPnl.liqPrice, tiers).remainingCollateral, 49_000n * WAD);

    const depleted = { ...pos, feesUsd: 160_000n * WAD };
    const depletedPnl = computePositionPnl(depleted, depleted.entryPrice, tiers);
    assert.equal(depletedPnl.liqPrice, (isLong ? 50_225n : 49_775n) * WAD);
    assert.equal(depletedPnl.liqSafetyBps, 0n);

    const fractional = { ...pos, entryPrice: 7n * WAD, leverage: 3n * WAD, sizeUsd: pos.sizeUsd + 1n, fundingPaidUsd: 1n };
    const fractionalPnl = computePositionPnl(fractional, fractional.entryPrice, tiers);
    const atLiq = computePositionPnl(fractional, fractionalPnl.liqPrice, tiers);
    const maintenance = maintenanceMargin(fractional.sizeUsd, tiers);
    const onePriceWei = (fractional.sizeUsd + fractional.entryPrice - 1n) / fractional.entryPrice;
    assert.ok(atLiq.remainingCollateral >= maintenance);
    assert.ok(atLiq.remainingCollateral - maintenance <= onePriceWei);
    const pastLiq = fractionalPnl.liqPrice + (isLong ? -1n : 1n);
    assert.ok(computePositionPnl(fractional, pastLiq, tiers).remainingCollateral <= maintenance);
  }

  assert.throws(() => computePositionPnl(position, position.entryPrice, []), RangeError);
  assert.throws(() => computePositionPnl(position, position.entryPrice), RangeError);
  assert.throws(() => computePositionPnl(position, position.entryPrice, [tiers[2]]), RangeError);
}

test(liquidationPriceUsesTiers);
