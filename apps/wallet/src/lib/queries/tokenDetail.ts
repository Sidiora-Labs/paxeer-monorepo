'use client';

/**
 * Token-detail queries — metadata, pool data, OHLC candles.
 *
 * The Sidiora-token resolution chain runs in two stages:
 *   1. `useTokenMetaQuery(tokenId)`     — parallel: chain metadata + public metadata
 *   2. `useTokenPoolDataQuery(poolAddr)` — only enabled once stage 1 returns a pool address
 *
 * `useTokenCandlesQuery` picks the right OHLC source based on token type:
 *   - PAX / WPAX  → `fetchPaxHistory`
 *   - SID         → `fetchSidCandles`
 *   - Sidiora pool → `fetchSidioraCandles`
 *   - other       → no candles (returns empty array)
 */

import { type UseQueryOptions } from '@tanstack/react-query';
import { useScopedQuery } from './scope';
import {
  fetchTokenInfo,
  fetchTokenMetadataPublic,
  fetchSidioraPoolStats,
  fetchSidioraPoolHolders,
  fetchPaxHistory,
  fetchSidCandles,
  fetchSidioraCandles,
  fetchCrossverseHistory,
  fetchCrossversePrice,
  getCrossverseSymbol,
  SID_TOKEN_ADDRESS,
  type OhlcCandle,
  type SidioraPoolStats,
  type HolderDistribution,
  type TokenMetadataPublic,
  type PaxPriceLatest,
} from '@/lib/api';
import { BlockscoutApiError, type WalletTokenMetadata } from '@/lib/data';

export type TimePeriod = '1H' | '1D' | '1W' | '1M' | 'ALL';

export interface TimePeriodConfig {
  id: TimePeriod;
  label: string;
  resolution: string;
  countback: number;
  paxRes: number;
  paxCount: number;
}

export const TIME_PERIODS: readonly TimePeriodConfig[] = [
  { id: '1H', label: '1H', resolution: '1', countback: 60, paxRes: 1, paxCount: 60 },
  { id: '1D', label: '1D', resolution: '15', countback: 96, paxRes: 15, paxCount: 96 },
  { id: '1W', label: '1W', resolution: '60', countback: 168, paxRes: 60, paxCount: 168 },
  { id: '1M', label: '1M', resolution: '240', countback: 180, paxRes: 240, paxCount: 180 },
  { id: 'ALL', label: 'ALL', resolution: '1D', countback: 365, paxRes: 1440, paxCount: 365 },
] as const;

export interface TokenMetaResult {
  chain: WalletTokenMetadata | null;
  public: TokenMetadataPublic | null;
}

export const TOKEN_META_NEGATIVE_TTL = 30_000;
export const TOKEN_META_TTL = 5 * 60_000;

async function metadataOrNotFound<T>(
  producer: () => Promise<T>,
  publicLookup = false
): Promise<T | null> {
  try {
    return await producer();
  } catch (error) {
    // The public API currently exposes only an HTTP status message.
    if (
      (error instanceof BlockscoutApiError && error.status === 404) ||
      (publicLookup && error instanceof Error && /^404(?:\s|$)/.test(error.message))
    )
      return null;
    throw error;
  }
}

function validDecimals(value: unknown): boolean {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0 && value <= 255;
}

async function loadTokenMetadata(tokenId: string, isWpax: boolean): Promise<TokenMetaResult> {
  const [chain, publicMetadata] = await Promise.all([
    metadataOrNotFound(() => fetchTokenInfo(tokenId)),
    isWpax
      ? Promise.resolve(null)
      : metadataOrNotFound(async () => {
          const metadata = await fetchTokenMetadataPublic(tokenId);
          if (!metadata || typeof metadata !== 'object')
            throw new TypeError('Invalid public token metadata');
          return metadata;
        }, true),
  ]);
  if (
    chain &&
    (typeof chain.address !== 'string' ||
      chain.address.toLowerCase() !== tokenId.toLowerCase() ||
      (chain.decimals !== null && !validDecimals(chain.decimals)))
  ) {
    throw new TypeError('Invalid chain token metadata');
  }
  if (
    publicMetadata &&
    (typeof publicMetadata.token_address !== 'string' ||
      publicMetadata.token_address.toLowerCase() !== tokenId.toLowerCase() ||
      (publicMetadata.pool_address != null &&
        !/^0x[0-9a-f]{40}$/i.test(publicMetadata.pool_address)) ||
      (publicMetadata.decimals !== undefined && !validDecimals(publicMetadata.decimals)))
  ) {
    throw new TypeError('Invalid public token metadata');
  }
  return { chain, public: publicMetadata };
}

/** Metadata failures remain retryable and retain the last successful query data. */
export function useTokenMetaQuery(
  tokenId: string,
  isPax: boolean,
  isWpax: boolean,
  options?: Partial<UseQueryOptions<TokenMetaResult>>
) {
  return useScopedQuery<TokenMetaResult>({
    queryKey: ['tokenMeta', tokenId.toLowerCase(), isWpax],
    queryFn: () => loadTokenMetadata(tokenId, isWpax),
    enabled: Boolean(tokenId) && !isPax,
    ...options,
    staleTime: (query) =>
      !query.state.data?.chain || (!isWpax && !query.state.data.public)
        ? TOKEN_META_NEGATIVE_TTL
        : TOKEN_META_TTL,
    gcTime: TOKEN_META_TTL,
    refetchOnMount: true,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  });
}

export interface PoolDataResult {
  stats: SidioraPoolStats | null;
  holders: HolderDistribution | null;
}

/**
 * Sidiora pool stats + holder distribution. Only enabled once a pool address
 * is known (typically resolved upstream by `useTokenMetaQuery`).
 */
export function useTokenPoolDataQuery(
  poolAddress: string | null | undefined,
  options?: Partial<UseQueryOptions<PoolDataResult>>
) {
  return useScopedQuery<PoolDataResult>({
    queryKey: ['tokenPoolData', (poolAddress ?? '').toLowerCase()],
    queryFn: async () => {
      const [stats, holders] = await Promise.all([
        fetchSidioraPoolStats(poolAddress as string).catch(() => null),
        fetchSidioraPoolHolders(poolAddress as string).catch(() => null),
      ]);
      return { stats, holders };
    },
    enabled: Boolean(poolAddress),
    staleTime: 60_000,
    ...options,
  });
}

/**
 * OHLC candles routed by token type. Returns an empty array for unknown
 * tokens with no pool — the consumer renders a "no chart data" placeholder.
 */
export function useTokenCandlesQuery(
  tokenId: string,
  period: TimePeriod,
  poolAddress: string | null | undefined,
  isPax: boolean,
  isWpax: boolean,
  options?: Partial<UseQueryOptions<OhlcCandle[]>>
) {
  const isSid = tokenId.toLowerCase() === SID_TOKEN_ADDRESS.toLowerCase();

  return useScopedQuery<OhlcCandle[]>({
    queryKey: ['tokenCandles', tokenId.toLowerCase(), period, poolAddress ?? null],
    queryFn: async () => {
      const cfg = TIME_PERIODS.find((t) => t.id === period)!;
      try {
        if (isPax || isWpax) {
          const ohlc = await fetchPaxHistory(isWpax ? 'WPAX' : 'PAX', cfg.paxRes, cfg.paxCount);
          return (ohlc as any).ohlc || [];
        }
        if (isSid) {
          const ohlc = await fetchSidCandles(cfg.resolution, cfg.countback);
          return (ohlc as any).ohlc || [];
        }
        const cvSymbol = getCrossverseSymbol(tokenId);
        if (cvSymbol) {
          const ohlc = await fetchCrossverseHistory(cvSymbol, cfg.paxRes, cfg.paxCount);
          return ohlc.ohlc;
        }
        if (poolAddress) {
          const ohlc = await fetchSidioraCandles(poolAddress, cfg.resolution, cfg.countback);
          return (ohlc as any).ohlc || [];
        }
      } catch {
        /* chart load failed — fall through to empty array */
      }
      return [];
    },
    // Don't fetch until we know whether this is a Sidiora token (poolAddress
    // resolution is async). For PAX/WPAX/SID we don't need a pool address.
    enabled:
      Boolean(tokenId) &&
      (isPax || isWpax || isSid || Boolean(poolAddress) || Boolean(getCrossverseSymbol(tokenId))),
    staleTime: 30_000,
    ...options,
  });
}

/**
 * Live price for crossverse-registered tokens (SOL, ETH, BNB, PAX ERC-20).
 * Disabled when cvSymbol is null (non-crossverse tokens).
 */
export function useCrossversePriceQuery(
  cvSymbol: string | null,
  options?: Partial<UseQueryOptions<PaxPriceLatest>>
) {
  return useScopedQuery<PaxPriceLatest>({
    queryKey: ['crossversePrice', cvSymbol ?? ''],
    queryFn: () => fetchCrossversePrice(cvSymbol!),
    enabled: Boolean(cvSymbol),
    staleTime: 30_000,
    ...options,
  });
}
