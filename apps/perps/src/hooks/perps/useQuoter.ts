import { fetchWithPolicy as fetch } from "@/core/net/fetch";
import type { ContractFunctionReturnType } from "viem";
import { PerpsPrecompileABI } from "@/core/network/abi/PerpsPrecompile";
import { PRECOMPILES } from "@/core/network/precompiles";
import { buildGetMarketConfig, buildGetMarginTiers, buildGetMarkPriceDetails } from "@/core/perps/calls";
import { decodeMarketConfig, type MarketConfig, type MarginTier } from "@/core/perps/marketConfig";
import {
  type MarketQuote,
  type PartialCloseQuote,
  type PrecompileMarketQuote,
  type PrecompilePartialCloseQuote,
  marketQuoteFromPrecompile,
  partialCloseQuoteFromPrecompile,
  tickerQuoteWithConfig,
} from "@/core/perps/quoteFallback";
import { type VenueTicker, parseFundingRateHourly } from "@/core/perps/fundingRate";
import { MARKET_DATA_URL as REST_API_URL } from "@/core/feeds";
import { readContract } from "./useContract";

export interface OpenPositionQuote {
  entryPrice: bigint;
  sizeUsd: bigint;
  collateralUsd: bigint;
  leverage: bigint;
  tradingFee: bigint;
  tradingFeeUsd: bigint;
  priceImpact: bigint;
  liquidationPrice: bigint;
  estimatedFunding24h: bigint;
  maintenanceMarginBps: bigint;
}

export interface ClosePositionQuote {
  exitPrice: bigint;
  unrealizedPnl: bigint;
  tradingFee: bigint;
  tradingFeeUsd: bigint;
  fundingOwed: bigint;
  netPnl: bigint;
  estimatedPayout: bigint;
}

export type { MarketQuote, PartialCloseQuote };

/**
 * fundingRate24h on the venue path is the last settled hourly rate ×24, a
 * projection; lastSettledFundingRate1h is that hourly rate, null when the
 * quote came from the on-chain fallback.
 */
export interface QuoterMarketQuote extends MarketQuote {
  lastSettledFundingRate1h: bigint | null;
}

// ── Open-interest singleton cache ──────────────────────────────────
interface OiEntry {
  paxeerSymbol: string;
  long_oi: number;
  short_oi: number;
}
let _oiCache: OiEntry[] | null = null;
let _oiLastFetch = 0;
let _oiInflight: Promise<OiEntry[]> | null = null;
const OI_CACHE_TTL = 60_000;

async function _fetchOi(): Promise<OiEntry[]> {
  if (_oiCache && Date.now() - _oiLastFetch < OI_CACHE_TTL) return _oiCache;
  if (_oiInflight) return _oiInflight;
  _oiInflight = fetch(`${REST_API_URL}/open-interest`)
    .then((r) => r.json())
    .then((json) => {
      if (json.success) {
        _oiCache = json.data ?? [];
        _oiLastFetch = Date.now();
      }
      return _oiCache ?? [];
    })
    .catch(() => _oiCache ?? [])
    .finally(() => {
      _oiInflight = null;
    });
  return _oiInflight;
}

function getSymbolForMarket(marketId: bigint, config: MarketConfig): string {
  if (config.marketId !== marketId || !config.symbol.trim()) {
    throw new Error(`Unknown market: ${marketId}`);
  }
  return config.symbol;
}

export function useQuoter() {
  /**
   * Fetches market data from the REST /api/ticker/:symbol endpoint.
   * Returns a MarketQuote with values scaled to 18 decimals (bigint)
   * for backward compatibility with consumers that expect bigints.
   */
  async function quoteMarket(marketId: bigint): Promise<QuoterMarketQuote> {
    const config = decodeMarketConfig(
      (await readContract(buildGetMarketConfig(marketId))) as ContractFunctionReturnType<
        typeof PerpsPrecompileABI,
        "view",
        "getMarketConfig"
      >,
    );
    const symbol = getSymbolForMarket(marketId, config);
    try {
      const [res, tiers, markDetails] = await Promise.all([
        fetch(`${REST_API_URL}/ticker/${encodeURIComponent(symbol)}`),
        readContract(buildGetMarginTiers(marketId)),
        readContract(buildGetMarkPriceDetails(marketId)),
      ]);
      if (!res.ok) throw new Error(`Ticker fetch failed: ${res.status}`);
      const json = await res.json();
      if (!json.success) throw new Error("Ticker response unsuccessful");
      const t: VenueTicker = json.data;
      const lastSettledFundingRate1h = parseFundingRateHourly(t.last_funding_rate_hourly);
      const [markPrice, indexPrice, midPrice, fresh] = markDetails as ContractFunctionReturnType<
        typeof PerpsPrecompileABI,
        "view",
        "getMarkPriceDetails"
      >;

      // Also fetch OI breakdown — cached, refreshed at most every 60 s
      let longOI = 0;
      let shortOI = 0;
      try {
        const oiData = await _fetchOi();
        const entry = oiData.find((d) => d.paxeerSymbol === symbol);
        if (entry) {
          longOI = entry.long_oi ?? 0;
          shortOI = entry.short_oi ?? 0;
        }
      } catch {}

      return {
        ...tickerQuoteWithConfig(
          t,
          config,
          tiers as readonly MarginTier[],
          { markPrice, indexPrice, midPrice, fresh },
          longOI,
          shortOI,
        ),
        lastSettledFundingRate1h,
      };
    } catch (err) {
      // Fallback to on-chain if REST fails
      console.warn("[useQuoter] REST ticker failed, falling back to on-chain:", err);
      const result = await readContract({
        address: PRECOMPILES.perps,
        abi: PerpsPrecompileABI,
        functionName: "quoteMarket",
        args: [marketId],
      });
      return { ...marketQuoteFromPrecompile(result as PrecompileMarketQuote), lastSettledFundingRate1h: null };
    }
  }

  async function quoteOpenPosition(params: {
    marketId: bigint;
    collateralToken: `0x${string}`;
    collateralAmount: bigint;
    leverage: bigint;
    isLong: boolean;
  }): Promise<OpenPositionQuote> {
    const result = await readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteOpenPosition",
      args: [params.marketId, params.collateralToken, params.collateralAmount, params.leverage, params.isLong],
    });
    return result as unknown as OpenPositionQuote;
  }

  async function quoteClosePosition(positionId: bigint): Promise<ClosePositionQuote> {
    const result = await readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteClosePosition",
      args: [positionId],
    });
    return result as unknown as ClosePositionQuote;
  }

  async function quotePartialClose(positionId: bigint, closeSizeUsd: bigint): Promise<PartialCloseQuote> {
    const result = await readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quotePartialClose",
      args: [positionId, closeSizeUsd],
    });
    return partialCloseQuoteFromPrecompile(result as PrecompilePartialCloseQuote);
  }

  return { quoteMarket, quoteOpenPosition, quoteClosePosition, quotePartialClose };
}
