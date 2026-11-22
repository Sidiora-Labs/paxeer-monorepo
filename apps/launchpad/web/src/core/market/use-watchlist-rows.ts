'use client';
import { useMemo } from 'react';
import { useQueries } from '@tanstack/react-query';
import { queryKeys } from '@/core/query-keys';
import { marketCachePolicy } from '@/core/cache-policy';
import { useMarketClient } from './client';
import { useOperations } from './operations';
import { useWatchlist } from '@/hooks/ui/use-watchlist';
import { useTokenMetadataBatch } from '@/hooks/market/use-token-metadata';
import type { TokenMetadata } from '@/widgets/home/types';
import { marketLimits } from '@/page-modules/token/market-session';
import { fetchStatsBatch, statsByPool, type PoolStats } from './use-stats-batch';
import { usePoolListings, type PoolListing } from './pool-listings';
import { loadable, type Loadable } from './unsupported';

export interface WatchlistRow {
  poolAddress: string;
  tokenAddress: string | null;
  stats: PoolStats | null;
  /** Registry facts (token, creation time) from IF_POOL_DETAIL. */
  listing: PoolListing | null;
  metadata: TokenMetadata | null;
}

export interface WatchlistRows {
  rows: WatchlistRow[];
  pools: readonly string[];
  isLocal: boolean;
  account: ReturnType<typeof useWatchlist>;
  retrySources(): void;
  loading: boolean;
  stats: Loadable<ReadonlyMap<string, PoolStats>>;
  listings: Loadable<Record<string, PoolListing>>;
  metadata: Loadable<Record<string, TokenMetadata | null>>;
  toggle(poolAddress: string): void;
  check(poolAddress: string): boolean;
}

/**
 * Read-only join of the account (or local) watchlist with IF_STATS_BATCH,
 * the pool listings (IF_POOL_DETAIL, session-cached: pool -> token, creation
 * time) and the token metadata batch. Each source carries its own Loadable so
 * one unavailable source never blanks the others.
 */
export function useWatchlistRows(): WatchlistRows {
  const watchlist = useWatchlist();
  const client = useMarketClient();
  const operations = useOperations();
  const pools = useMemo(() => [...new Set(watchlist.list.map(value => value.toLowerCase()))].filter(value => /^0x[0-9a-f]{40}$/.test(value)), [watchlist.list]);
  const chunks = useMemo(() => {
    const result: string[][] = [];
    for (let start = 0; start < pools.length; start += marketLimits.rankingPage) result.push(pools.slice(start, start + marketLimits.rankingPage));
    return result;
  }, [pools]);
  const stats = useQueries({
    queries: chunks.map(chunk => ({
      queryKey: queryKeys.statsBatch(client.config.apiOrigin, chunk),
      queryFn: ({ signal }: { signal: AbortSignal }) => fetchStatsBatch(operations, chunk, signal),
      ...marketCachePolicy.statsBatch,
    })),
  });
  const listings = usePoolListings(pools);
  const tokens = useMemo(() => pools.map(pool => listings.data?.[pool]?.tokenAddress ?? null).filter((value): value is string => value !== null), [pools, listings.data]);
  const metadata = useTokenMetadataBatch(tokens);
  const byPool = new Map<string, PoolStats>();
  for (const query of stats) for (const [pool, row] of statsByPool(query.data)) byPool.set(pool, row);
  const statsStates = stats.map(query => loadable(query, 'IF_STATS_BATCH'));
  const statsState = statsStates.find(source => source.state === 'error' || source.state === 'unsupported')
    ?? statsStates.find(source => source.state === 'loading');
  const rows: WatchlistRow[] = pools.map(pool => {
    const listing = listings.data?.[pool] ?? null;
    const token = listing?.tokenAddress ?? null;
    return { poolAddress: pool, tokenAddress: token, stats: byPool.get(pool) ?? null, listing, metadata: token ? metadata.data?.[token] ?? null : null };
  });
  return {
    rows, pools, isLocal: watchlist.isLocal, loading: watchlist.loading, account: watchlist,
    retrySources() {
      for (const query of stats) void query.refetch();
      if (pools.length) void listings.refetch();
      if (tokens.length) void metadata.refetch();
    },
    stats: statsState ?? { state: 'ready', data: byPool },
    listings: pools.length === 0 ? { state: 'ready', data: {} } : loadable(listings, 'IF_POOL_DETAIL'),
    metadata: tokens.length === 0 ? { state: 'ready', data: {} } : loadable(metadata, 'LEGACY_METADATA_BATCH'),
    toggle: watchlist.toggle, check: watchlist.check,
  };
}
