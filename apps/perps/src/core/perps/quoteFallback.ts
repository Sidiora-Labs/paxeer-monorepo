import type { ContractFunctionReturnType } from "viem";
import type { PerpsPrecompileABI } from "@/core/network/abi/PerpsPrecompile";
import { type VenueTicker, parseFundingRateHourly, projectFunding24h } from "./fundingRate";
import type { MarketConfig, MarginTier } from "./marketConfig";
import { type MarkPriceDetails, markStatus } from "./markStatus";

export interface MarketQuote {
  indexPrice: bigint;
  markPrice: bigint;
  oracleTWAP: bigint;
  fundingRatePerSecond: bigint;
  fundingRate24h: bigint;
  longOI: bigint;
  shortOI: bigint;
  maxLeverage: bigint;
  maintenanceMarginBps: bigint;
  enabled: boolean;
  priceStale: boolean;
}

export interface PartialCloseQuote {
  exitPrice: bigint;
  closedPnl: bigint;
  fee: bigint;
  estimatedPayout: bigint;
}

export type PrecompileMarketQuote = ContractFunctionReturnType<typeof PerpsPrecompileABI, "view", "quoteMarket">;
export type PrecompilePartialCloseQuote = ContractFunctionReturnType<
  typeof PerpsPrecompileABI,
  "view",
  "quotePartialClose"
>;

function toBigInt18(value: number): bigint {
  return BigInt(Math.round(value * 1e18));
}

export function tickerQuoteWithConfig(
  ticker: VenueTicker,
  config: MarketConfig,
  tiers: readonly MarginTier[],
  details: MarkPriceDetails,
  longOI: number,
  shortOI: number,
): MarketQuote {
  const tier = tiers[0];
  if (!tier) throw new Error(`Missing margin tiers for market ${config.marketId}`);

  return {
    indexPrice: toBigInt18(ticker.index_price ?? 0),
    markPrice: toBigInt18(ticker.mark_price ?? 0),
    oracleTWAP: toBigInt18(ticker.index_price ?? 0),
    fundingRatePerSecond: 0n,
    fundingRate24h: projectFunding24h(parseFundingRateHourly(ticker.last_funding_rate_hourly)),
    longOI: toBigInt18(longOI),
    shortOI: toBigInt18(shortOI),
    maxLeverage: config.maxLeverage,
    maintenanceMarginBps: tier.maintenanceBps,
    enabled: markStatus(details, config.enabled) !== "disabled",
    priceStale: markStatus(details, true) === "stale",
  };
}

/** Map the 0x1015 quoteMarket tuple to the hook quote, keeping the market's own leverage cap and margin. */
export function marketQuoteFromPrecompile(result: PrecompileMarketQuote): MarketQuote {
  return {
    indexPrice: result.indexPrice,
    markPrice: result.markPrice,
    oracleTWAP: result.oracleTWAP,
    fundingRatePerSecond: result.fundingRatePerSecond,
    fundingRate24h: result.fundingRate24h,
    longOI: result.longOI,
    shortOI: result.shortOI,
    maxLeverage: result.maxLeverage,
    maintenanceMarginBps: result.maintenanceMarginBps,
    enabled: result.enabled,
    priceStale: result.priceStale,
  };
}

/** Map the four positional outputs of 0x1015 quotePartialClose to a named quote. */
export function partialCloseQuoteFromPrecompile(result: PrecompilePartialCloseQuote): PartialCloseQuote {
  const [exitPrice, closedPnl, fee, estimatedPayout] = result;
  return { exitPrice, closedPnl, fee, estimatedPayout };
}
