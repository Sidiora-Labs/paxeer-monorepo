import type { ContractFunctionReturnType } from "viem";
import type { PerpsPrecompileABI } from "@/core/network/abi/PerpsPrecompile";

export interface MarketConfig {
  marketId: bigint;
  symbol: string;
  name: string;
  oracleSymbol: string;
  szDecimals: number;
  pxDecimals: number;
  tickSize: bigint;
  lotSize: bigint;
  maxLeverage: bigint;
  tierClass: number;
  impactNotionalUsd: bigint;
  oiCapUsd: bigint;
  fundingCapPpmPerHour: bigint;
  enabled: boolean;
  listedAt: bigint;
}

export type MarginTier = ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getMarginTiers">[number];

export function decodeMarketConfig(
  result: ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "getMarketConfig">,
): MarketConfig {
  return {
    marketId: result.marketId,
    symbol: result.symbol,
    name: result.name,
    oracleSymbol: result.oracleSymbol,
    szDecimals: result.szDecimals,
    pxDecimals: result.pxDecimals,
    tickSize: result.tickSize,
    lotSize: result.lotSize,
    maxLeverage: result.maxLeverage,
    tierClass: result.tierClass,
    impactNotionalUsd: result.impactNotionalUsd,
    oiCapUsd: result.oiCapUsd,
    fundingCapPpmPerHour: result.fundingCapPpmPerHour,
    enabled: result.enabled,
    listedAt: result.listedAt,
  };
}

function tierForNotional(notionalUsd: bigint, tiers: readonly MarginTier[]): MarginTier {
  let selected: MarginTier | undefined;
  for (const tier of tiers) {
    if (tier.lowerBoundUsd <= notionalUsd && (!selected || tier.lowerBoundUsd > selected.lowerBoundUsd)) {
      selected = tier;
    }
  }
  if (!selected) throw new RangeError("No margin tier covers this notional");
  return selected;
}

export function maxLeverageForNotional(notionalUsd: bigint, tiers: readonly MarginTier[]): bigint {
  return tierForNotional(notionalUsd, tiers).maxLeverage;
}

export function maintenanceMargin(notionalUsd: bigint, tiers: readonly MarginTier[]): bigint {
  const tier = tierForNotional(notionalUsd, tiers);
  return (notionalUsd * tier.maintenanceBps) / 10_000n - tier.deductionUsd;
}
