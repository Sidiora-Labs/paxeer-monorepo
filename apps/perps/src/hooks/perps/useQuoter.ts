import { z } from "zod";
import { fetchWithPolicy as fetch } from "@/core/net/fetch";
import type { ContractFunctionReturnType } from "viem";
import { PerpsPrecompileABI } from "@/core/network/abi/PerpsPrecompile";
import { PRECOMPILES } from "@/core/network/precompiles";
import {
  buildGetMarketConfig,
  buildGetMarginTiers,
  buildGetMarkPriceDetails,
} from "@/core/perps/calls";
import {
  decodeMarketConfig,
  type MarketConfig,
  type MarginTier,
} from "@/core/perps/marketConfig";
import {
  type MarketQuote,
  type PartialCloseQuote,
  type PrecompileMarketQuote,
  marketQuoteFromPrecompile,
  partialCloseQuoteFromPrecompile,
  tickerQuoteWithConfig,
} from "@/core/perps/quoteFallback";
import {
  type VenueTicker,
  parseFundingRateHourly,
} from "@/core/perps/fundingRate";
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

const uint256 = z
  .bigint()
  .nonnegative()
  .max((1n << 256n) - 1n);
const int256 = z
  .bigint()
  .min(-(1n << 255n))
  .max((1n << 255n) - 1n);
const openPositionQuoteSchema = z.object({
  entryPrice: uint256.positive(),
  sizeUsd: uint256.positive(),
  collateralUsd: uint256.positive(),
  leverage: uint256.min(10n ** 18n),
  tradingFee: uint256,
  tradingFeeUsd: uint256,
  priceImpact: uint256,
  liquidationPrice: uint256,
  estimatedFunding24h: int256,
  maintenanceMarginBps: uint256.max(10_000n),
});

export function decodeOpenPositionQuote(result: unknown): OpenPositionQuote {
  return openPositionQuoteSchema.parse(result);
}

/** The exact descriptors quoteMarket dispatches, built anew for every request. */
export function quoterMarketReads(marketId: bigint) {
  return {
    config: buildGetMarketConfig(marketId),
    tiers: buildGetMarginTiers(marketId),
    mark: buildGetMarkPriceDetails(marketId),
  };
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
  openInterest: OpenInterestProvenance;
}

export interface OpenInterestProvenance {
  /** Normalized venue base URL; null for the on-chain quote. */
  origin: string | null;
  source: "fresh" | "cache" | "stale" | "unavailable" | "on-chain";
  fetchedAt: number | null;
  ageMs: number | null;
}

// Open-interest data and in-flight work belong to one normalized venue base URL.
interface OiEntry {
  paxeerSymbol: string;
  long_oi: number;
  short_oi: number;
}
interface OiResult {
  rows: OiEntry[];
  provenance: OpenInterestProvenance;
}
interface OiCache {
  rows: OiEntry[] | null;
  fetchedAt: number | null;
  inflight: Promise<OiResult> | null;
}
const oiCaches = new Map<string, OiCache>();
const OI_CACHE_TTL = 60_000;
const OI_FALLBACK_MAX_AGE = 300_000;
const oiResponse = z.object({
  success: z.literal(true),
  data: z.array(
    z.object({
      paxeerSymbol: z.string().min(1),
      long_oi: z.number().finite().nonnegative(),
      short_oi: z.number().finite().nonnegative(),
    }),
  ),
});

function normalizeMarketDataUrl(value: string): string {
  const url = new URL(value);
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.username ||
    url.password ||
    url.search ||
    url.hash
  ) {
    throw new Error("Invalid market-data base URL");
  }
  return url.href.replace(/\/+$/, "");
}

async function _fetchOi(origin: string): Promise<OiResult> {
  let cache = oiCaches.get(origin);
  if (!cache) {
    cache = { rows: null, fetchedAt: null, inflight: null };
    oiCaches.set(origin, cache);
  }
  const selected = cache;
  const result = (source: OpenInterestProvenance["source"]): OiResult => ({
    rows: source === "unavailable" ? [] : (selected.rows ?? []),
    provenance: {
      origin,
      source,
      fetchedAt: selected.fetchedAt,
      ageMs:
        selected.fetchedAt === null
          ? null
          : Math.max(0, Date.now() - selected.fetchedAt),
    },
  });
  if (
    selected.rows &&
    selected.fetchedAt !== null &&
    Date.now() - selected.fetchedAt < OI_CACHE_TTL
  )
    return result("cache");
  if (selected.inflight) return selected.inflight;
  selected.inflight = fetch(`${origin}/open-interest`)
    .then(async (response) => {
      if (!response.ok)
        throw new Error(`Open-interest fetch failed: ${response.status}`);
      const json = oiResponse.parse(await response.json());
      selected.rows = json.data;
      selected.fetchedAt = Date.now();
      return result("fresh");
    })
    .catch(() =>
      result(
        selected.rows &&
          selected.fetchedAt !== null &&
          Date.now() - selected.fetchedAt <= OI_FALLBACK_MAX_AGE
          ? "stale"
          : "unavailable",
      ),
    )
    .finally(() => {
      selected.inflight = null;
    });
  return selected.inflight;
}

function getSymbolForMarket(marketId: bigint, config: MarketConfig): string {
  if (config.marketId !== marketId || !config.symbol.trim()) {
    throw new Error(`Unknown market: ${marketId}`);
  }
  return config.symbol;
}

const closeQuoteSchema = z.object({
  exitPrice: uint256.positive(),
  unrealizedPnl: int256,
  tradingFee: uint256,
  tradingFeeUsd: uint256,
  fundingOwed: int256,
  netPnl: int256,
  estimatedPayout: uint256,
});
const partialQuoteSchema = z.tuple([
  uint256.positive(),
  int256,
  uint256,
  uint256,
]);

export function useQuoter(marketDataUrl = REST_API_URL) {
  const origin = normalizeMarketDataUrl(marketDataUrl);
  /**
   * Fetches market data from the REST /api/ticker/:symbol endpoint.
   * Returns a MarketQuote with values scaled to 18 decimals (bigint)
   * for backward compatibility with consumers that expect bigints.
   */
  async function quoteMarket(marketId: bigint): Promise<QuoterMarketQuote> {
    const reads = quoterMarketReads(marketId);
    const config = decodeMarketConfig(
      (await readContract(reads.config)) as ContractFunctionReturnType<
        typeof PerpsPrecompileABI,
        "view",
        "getMarketConfig"
      >,
    );
    const symbol = getSymbolForMarket(marketId, config);
    try {
      const [res, tiers, markDetails] = await Promise.all([
        fetch(`${origin}/ticker/${encodeURIComponent(symbol)}`),
        readContract(reads.tiers),
        readContract(reads.mark),
      ]);
      if (!res.ok) throw new Error(`Ticker fetch failed: ${res.status}`);
      const json = await res.json();
      if (!json.success) throw new Error("Ticker response unsuccessful");
      const t: VenueTicker = json.data;
      const lastSettledFundingRate1h = parseFundingRateHourly(
        t.last_funding_rate_hourly,
      );
      const [markPrice, indexPrice, midPrice, fresh] =
        markDetails as ContractFunctionReturnType<
          typeof PerpsPrecompileABI,
          "view",
          "getMarkPriceDetails"
        >;

      // Also fetch OI breakdown — cached, refreshed at most every 60 s
      let longOI = 0;
      let shortOI = 0;
      const oiData = await _fetchOi(origin);
      const entry = oiData.rows.find((d) => d.paxeerSymbol === symbol);
      if (entry) {
        longOI = entry.long_oi;
        shortOI = entry.short_oi;
      }

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
        openInterest: entry
          ? oiData.provenance
          : { ...oiData.provenance, source: "unavailable" },
      };
    } catch (err) {
      // Fallback to on-chain if REST fails
      console.warn(
        "[useQuoter] REST ticker failed, falling back to on-chain:",
        err,
      );
      const result = await readContract({
        address: PRECOMPILES.perps,
        abi: PerpsPrecompileABI,
        functionName: "quoteMarket",
        args: [marketId],
      });
      return {
        ...marketQuoteFromPrecompile(result as PrecompileMarketQuote),
        lastSettledFundingRate1h: null,
        openInterest: {
          origin: null,
          source: "on-chain",
          fetchedAt: null,
          ageMs: null,
        },
      };
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
      args: [
        params.marketId,
        params.collateralToken,
        params.collateralAmount,
        params.leverage,
        params.isLong,
      ],
    });
    return decodeOpenPositionQuote(result);
  }

  async function quoteClosePosition(
    positionId: bigint,
  ): Promise<ClosePositionQuote> {
    const result = await readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quoteClosePosition",
      args: [positionId],
    });
    return closeQuoteSchema.parse(result);
  }

  async function quotePartialClose(
    positionId: bigint,
    closeSizeUsd: bigint,
  ): Promise<PartialCloseQuote> {
    const result = await readContract({
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      functionName: "quotePartialClose",
      args: [positionId, closeSizeUsd],
    });
    return partialCloseQuoteFromPrecompile(partialQuoteSchema.parse(result));
  }

  return {
    quoteMarket,
    quoteOpenPosition,
    quoteClosePosition,
    quotePartialClose,
  };
}
