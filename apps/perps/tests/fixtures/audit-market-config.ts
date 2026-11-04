import type { ContractFunctionReturnType } from "viem";
import type { PerpsPrecompileABI } from "../../src/core/network/abi/PerpsPrecompile";

// Chain-facing integer leverage units, before the production form's WAD conversion.
export const auditMarketConfig: ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getMarketConfig"> = {
  marketId: 0n, symbol: "BTC", name: "Bitcoin", oracleSymbol: "BTC",
  szDecimals: 3, pxDecimals: 2, tickSize: 10n ** 16n, lotSize: 10n ** 18n,
  maxLeverage: 50n, tierClass: 0, impactNotionalUsd: 10n ** 24n,
  oiCapUsd: 10n ** 26n, fundingCapPpmPerHour: 100n, enabled: true, listedAt: 0n,
};
export const auditMarginTiers: ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getMarginTiers"> = [
  { lowerBoundUsd: 0n, maxLeverage: 20n, maintenanceBps: 100n, deductionUsd: 0n },
  { lowerBoundUsd: 10_000n * 10n ** 18n, maxLeverage: 10n, maintenanceBps: 200n, deductionUsd: 0n },
];
