'use client';
import { useQuery, useQueryClient, type QueryClient, type UseQueryResult } from '@tanstack/react-query';
import { CapabilityUnavailableError, HttpError, type Operations, type V2ContractTypes } from '@kindle/sdk';
import { queryKeys } from '@/core/query-keys';
import { marketCachePolicy } from '@/core/cache-policy';
import { marketAddress, marketLimits } from '@/page-modules/token/market-session';
import { scaleRawInteger } from './market-units';
import { useMarketClient } from './client';
import { useOperations } from './operations';

/** v2 Pool; core/api also emits the optional registry `createdAt` (catalog Pool fields `createdAt:UTC?`). */
export type Pool = V2ContractTypes['Pool'] & { createdAt?: string };
export type PoolListSort = 'new' | 'volume' | 'trending';

/**
 * The immutable registry facts of a pool as IF_POOL_LIST / IF_POOL_DETAIL
 * serve them (core/api/internal/rest/pool_reads.go): its token, its creator
 * and its creation time. They never change, so they are cached per pool for
 * the session.
 */
export interface PoolListing {
  poolAddress: string;
  tokenAddress: string;
  creator: string;
  /** Block time of the creation event in ms; null when the registry row carries none. */
  createdAtMs: number | null;
}

/** Fresh IF_POOL_DETAIL read: the listing plus the current spot price derived from the served reserves. */
export interface PoolDetailListing extends PoolListing {
  /** Decimal USDL per whole token (effectivePricingUsdlReserveRaw / tokenReserveRaw at their decimals); null without verified token decimals or reserve. */
  spotPrice: string | null;
}

const isPool = (value: string) => /^0x[0-9a-fA-F]{40}$/.test(value);
const PRICE_DECIMALS = 18;

export function poolListing(pool: Pool): PoolListing {
  const created = pool.createdAt ? Date.parse(pool.createdAt) : NaN;
  return {
    poolAddress: marketAddress(pool.poolAddress),
    tokenAddress: marketAddress(pool.tokenAddress),
    creator: marketAddress(pool.creator),
    createdAtMs: Number.isFinite(created) && created > 0 ? created : null,
  };
}

/**
 * Spot price of a bonding-curve pool in decimal USDL per whole token, exact to
 * 18 places: effective USDL reserve (quoteAssetDecimals) over token reserve
 * (token decimals). Null when the token decimals are not verified or the
 * token reserve is zero.
 */
export function poolSpotPrice(pool: Pool, tokenDecimals: number | null): string | null {
  if (tokenDecimals === null || !Number.isSafeInteger(tokenDecimals) || tokenDecimals < 0 || tokenDecimals > 77) return null;
  const reserve = BigInt(pool.tokenReserveRaw);
  if (reserve === 0n) return null;
  const numerator = BigInt(pool.effectivePricingUsdlReserveRaw) * 10n ** BigInt(tokenDecimals + PRICE_DECIMALS);
  const denominator = reserve * 10n ** BigInt(pool.quoteAssetDecimals);
  return scaleRawInteger((numerator / denominator).toString(), PRICE_DECIMALS);
}

/** One IF_POOL_DETAIL `GET /v2/pools/{p}` with the identity and integrity guards. */
export async function fetchPoolDetailListing(operations: Operations, poolAddress: string, signal?: AbortSignal): Promise<PoolDetailListing> {
  const pool = marketAddress(poolAddress);
  const response = await operations.call('IF_POOL_DETAIL', { path: { poolAddress: pool } }, { signal });
  if (response.quality === 'quarantined' || response.watermark?.integrity === 'quarantined' || response.data.stats.watermark.integrity === 'quarantined') throw new Error('Pool integrity quarantined');
  const detail = response.data;
  if (detail.pool.chainId !== '125' || marketAddress(detail.stats.poolAddress) !== pool || marketAddress(detail.pool.poolAddress) !== pool || marketAddress(detail.token.address) !== marketAddress(detail.pool.tokenAddress)) throw new Error('Pool identity disagreement');
  const decimals = detail.token.decimalsQuality === 'verified' ? detail.token.decimals : null;
  return { ...poolListing(detail.pool), spotPrice: poolSpotPrice(detail.pool, decimals) };
}

/** Session-cached listing of one pool (IF_POOL_DETAIL on a miss). */
export async function fetchPoolListing(qc: QueryClient, operations: Operations, origin: string, pool: string, signal?: AbortSignal): Promise<PoolListing> {
  return qc.fetchQuery({
    queryKey: queryKeys.poolListing(origin, pool),
    queryFn: async () => {
      const { spotPrice: _price, ...listing } = await fetchPoolDetailListing(operations, pool, signal);
      return listing;
    },
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

function seed(qc: QueryClient, origin: string, listing: PoolListing): void {
  qc.setQueryData(queryKeys.poolListing(origin, listing.poolAddress), listing);
}

export interface PoolListingOmission {
  state: 'not_found' | 'unavailable';
  status: 404 | 503;
}

export interface PoolListingBatch<T extends PoolListing = PoolListing> {
  listings: Record<string, T>;
  omissions: Record<string, PoolListingOmission>;
}

export type PoolListingsQueryResult<T extends PoolListing = PoolListing> = UseQueryResult<Record<string, T>> & {
  omissions: Record<string, PoolListingOmission>;
};

function omissionFor(error: unknown): PoolListingOmission | null {
  if (
    (error instanceof CapabilityUnavailableError && error.status === 404) ||
    (error instanceof HttpError && error.status === 404 && error.code === 'NOT_FOUND')
  ) return { state: 'not_found', status: 404 };
  if (error instanceof HttpError && error.status === 503 && error.code === 'DEPENDENCY_UNAVAILABLE') return { state: 'unavailable', status: 503 };
  return null;
}

function classifyListings<T extends PoolListing>(wanted: readonly string[], settled: readonly PromiseSettledResult<T>[], signal?: AbortSignal): PoolListingBatch<T> {
  signal?.throwIfAborted();
  const listings: Record<string, T> = {};
  const omissions: Record<string, PoolListingOmission> = {};
  let firstUnavailable: unknown;
  for (const [index, result] of settled.entries()) {
    if (result.status === 'fulfilled') listings[wanted[index]] = result.value;
    else {
      const omission = omissionFor(result.reason);
      if (!omission) throw result.reason;
      omissions[wanted[index]] = omission;
      firstUnavailable ??= result.reason;
    }
  }
  if (wanted.length > 0 && Object.keys(listings).length === 0) throw firstUnavailable;
  return { listings, omissions };
}

/** Classified list/detail reads; only declared 404/503 omissions tolerate partial success. */
export async function fetchPoolListingsWithStatus(qc: QueryClient, operations: Operations, origin: string, pools: readonly string[], signal?: AbortSignal, sort?: PoolListSort): Promise<PoolListingBatch> {
  const wanted = [...new Set(pools.map(marketAddress))].sort();
  if (wanted.length === 0) return { listings: {}, omissions: {} };
  const missing = () => wanted.filter(pool => qc.getQueryData(queryKeys.poolListing(origin, pool)) === undefined);
  if (sort && missing().length > 1) {
    try {
      const response = await operations.call('IF_POOL_LIST', { query: { sort, limit: 2 * marketLimits.rankingPage } }, { signal });
      if (response.quality === 'quarantined' || response.watermark?.integrity === 'quarantined') throw new Error('Pool list integrity quarantined');
      const listings = response.data.items.map(pool => {
        if (pool.chainId !== '125') throw new Error('Pool identity disagreement');
        return poolListing(pool);
      });
      for (const listing of listings) seed(qc, origin, listing);
    } catch (error) {
      if (signal?.aborted || !omissionFor(error)) throw error;
    }
  }
  const settled = await Promise.allSettled(wanted.map(pool => fetchPoolListing(qc, operations, origin, pool, signal)));
  return classifyListings(wanted, settled, signal);
}

/** Compatibility projection for consumers that only need the successful rows. */
export async function fetchPoolListings(qc: QueryClient, operations: Operations, origin: string, pools: readonly string[], signal?: AbortSignal, sort?: PoolListSort): Promise<Record<string, PoolListing>> {
  return (await fetchPoolListingsWithStatus(qc, operations, origin, pools, signal, sort)).listings;
}

/** React binding retaining omission status alongside the existing address-indexed data. */
export function usePoolListings(pools: readonly string[], sort?: PoolListSort): PoolListingsQueryResult {
  const client = useMarketClient();
  const operations = useOperations();
  const qc = useQueryClient();
  const wanted = [...new Set(pools.filter(isPool).map(value => value.toLowerCase()))].sort();
  const queryKey = queryKeys.poolListings(client.config.apiOrigin, wanted);
  const query = useQuery({
    queryKey,
    queryFn: ({ signal }) => fetchPoolListingsWithStatus(qc, operations, client.config.apiOrigin, wanted, signal, sort),
    select: result => result.listings,
    enabled: wanted.length > 0,
    ...marketCachePolicy.statsBatch,
  });
  return Object.assign(query, { omissions: qc.getQueryData<PoolListingBatch>(queryKey)?.omissions ?? {} });
}

/** Fresh detail reads share the same error classification as session-cached listings. */
export async function fetchPoolDetailListings(qc: QueryClient, operations: Operations, origin: string, pools: readonly string[], signal?: AbortSignal): Promise<PoolListingBatch<PoolDetailListing>> {
  const wanted = [...new Set(pools.map(marketAddress))].sort();
  const settled = await Promise.allSettled(wanted.map(pool => fetchPoolDetailListing(operations, pool, signal)));
  const result = classifyListings(wanted, settled, signal);
  for (const { spotPrice: _price, ...listing } of Object.values(result.listings)) seed(qc, origin, listing);
  return result;
}

/** Fresh IF_POOL_DETAIL reads for a short pool list, retaining declared omissions. */
export function usePoolDetailListings(pools: readonly string[]): PoolListingsQueryResult<PoolDetailListing> {
  const client = useMarketClient();
  const operations = useOperations();
  const qc = useQueryClient();
  const origin = client.config.apiOrigin;
  const wanted = [...new Set(pools.filter(isPool).map(value => value.toLowerCase()))].sort();
  const queryKey = queryKeys.poolDetailListings(origin, wanted);
  const query = useQuery({
    queryKey,
    queryFn: ({ signal }) => fetchPoolDetailListings(qc, operations, origin, wanted, signal),
    select: result => result.listings,
    enabled: wanted.length > 0,
    ...marketCachePolicy.poolDetail,
  });
  return Object.assign(query, { omissions: qc.getQueryData<PoolListingBatch<PoolDetailListing>>(queryKey)?.omissions ?? {} });
}
