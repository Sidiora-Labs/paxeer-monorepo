import { strict as assert } from "node:assert";
import test from "node:test";

import { decodeFunctionResult, encodeFunctionResult, type Address } from "viem";

import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { PRECOMPILES } from "../src/core/network/precompiles";
import {
  decodeMarketConfig,
  maintenanceMargin,
  maxLeverageForNotional,
  type MarginTier,
  type MarketConfig,
} from "../src/core/perps/marketConfig";

const E18 = 10n ** 18n;

function decodedTiers(): readonly MarginTier[] {
  return decodeFunctionResult({
    abi: PerpsPrecompileABI,
    functionName: "getMarginTiers",
    data: encodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getMarginTiers",
      result: [
        { lowerBoundUsd: 0n, maxLeverage: 1000n, maintenanceBps: 5n, deductionUsd: 0n },
        { lowerBoundUsd: 1_000_000n * E18, maxLeverage: 500n, maintenanceBps: 10n, deductionUsd: 500n * E18 },
        { lowerBoundUsd: 5_000_000n * E18, maxLeverage: 200n, maintenanceBps: 25n, deductionUsd: 8_000n * E18 },
      ],
    }),
  });
}

async function decodeMarketConfigFromAbi() {
  const fixture = {
    marketId: 18n,
    symbol: "DOGE",
    name: "Dogecoin",
    oracleSymbol: "DOGE/USD",
    szDecimals: 3,
    pxDecimals: 7,
    tickSize: 123_456_789n,
    lotSize: 987_654_321_012_345_678n,
    maxLeverage: 1000n,
    tierClass: 4,
    impactNotionalUsd: 25_000n * E18,
    oiCapUsd: 500_000_000n * E18,
    fundingCapPpmPerHour: 1250n,
    enabled: true,
    listedAt: 1_750_000_123n,
  } satisfies MarketConfig;

  for (const enabled of [true, false]) {
    const result = { ...fixture, enabled };
    const decoded = decodeFunctionResult({
      abi: PerpsPrecompileABI,
      functionName: "getMarketConfig",
      data: encodeFunctionResult({ abi: PerpsPrecompileABI, functionName: "getMarketConfig", result }),
    });
    assert.deepEqual(decodeMarketConfig(decoded), result);
  }

  const originalGatewayUrl = process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "http://127.0.0.1:3000";
  try {
    const { buildGetFeeTier, buildGetMarginTiers, buildGetMarketConfig, buildGetMarkPriceDetails } = await import(
      "../src/core/perps/calls"
    );
    const account: Address = "0x1234567890123456789012345678901234567890";
    const reads = [
      [buildGetMarketConfig(18n), "getMarketConfig", [18n]],
      [buildGetMarginTiers(18n), "getMarginTiers", [18n]],
      [buildGetMarkPriceDetails(18n), "getMarkPriceDetails", [18n]],
      [buildGetFeeTier(account), "getFeeTier", [account]],
    ] as const;
    for (const [call, functionName, args] of reads) {
      assert.deepEqual(call, { address: PRECOMPILES.perps, abi: PerpsPrecompileABI, functionName, args });
    }
  } finally {
    if (originalGatewayUrl === undefined) delete process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL;
    else process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = originalGatewayUrl;
  }
}

function maxLeverageByTier() {
  const tiers = decodedTiers();
  const cases = [
    [0n, 1000n],
    [1n, 1000n],
    [1_000_000n * E18 - 1n, 1000n],
    [1_000_000n * E18, 500n],
    [1_000_000n * E18 + 1n, 500n],
    [5_000_000n * E18 - 1n, 500n],
    [5_000_000n * E18, 200n],
    [5_000_000n * E18 + 1n, 200n],
    [100_000_000n * E18, 200n],
  ] as const;
  for (const [notional, expected] of cases) {
    assert.equal(maxLeverageForNotional(notional, tiers), expected);
  }
  assert.equal(maxLeverageForNotional(2_000_000n * E18, [tiers[2], tiers[0], tiers[1]]), 500n);
  assert.throws(() => maxLeverageForNotional(0n, []), RangeError);
  assert.throws(() => maxLeverageForNotional(-1n, tiers), RangeError);
  assert.throws(() => maxLeverageForNotional(1n, tiers.slice(1)), RangeError);
}

function maintenanceMarginTiered() {
  const tiers = decodedTiers();
  const cases = [
    [0n, 0n],
    [100_000n * E18, 50n * E18],
    [1_000_000n * E18, 500n * E18],
    [2_000_000n * E18, 1500n * E18],
    [5_000_000n * E18, 4500n * E18],
    [10_000_000n * E18, 17_000n * E18],
    [10_000_000n * E18 + 401n, 17_000n * E18 + 1n],
  ] as const;
  for (const [notional, expected] of cases) {
    assert.equal(maintenanceMargin(notional, tiers), expected);
  }
  for (const notional of [1_000_000n * E18 - 1n, 5_000_000n * E18 - 1n, 5_000_000n * E18 + 1n]) {
    const tier = notional < tiers[1].lowerBoundUsd ? tiers[0] : notional < tiers[2].lowerBoundUsd ? tiers[1] : tiers[2];
    assert.equal(maintenanceMargin(notional, tiers), (notional * tier.maintenanceBps) / 10_000n - tier.deductionUsd);
  }
  assert.equal(maintenanceMargin(2_000_000n * E18, [tiers[2], tiers[0], tiers[1]]), 1500n * E18);
  assert.throws(() => maintenanceMargin(0n, []), RangeError);
  assert.throws(() => maintenanceMargin(-1n, tiers), RangeError);
}

test(decodeMarketConfigFromAbi);
test(maxLeverageByTier);
test(maintenanceMarginTiered);
