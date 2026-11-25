'use client';
import { useMarketQuery } from './use-market-query';
import type { SdkClient } from '@kindle/sdk';
import { queryKeys } from '@/core/query-keys';
import { marketCachePolicy } from '@/core/cache-policy';
import { browserStatsClient } from './client';
import { loadable, type Loadable } from './unsupported';
import { holdersHostUnset, readHoldersPage, type HoldersPage } from './holders-reader';

export { fetchCanonicalHolders, fetchLegacyShare, holdersHostUnset } from './holders-reader';
export type { HoldersResponse, HolderEntry, LegacyHolderShare, HoldersPage } from './holders-reader';

export async function loadHoldersPage(poolAddress: string, cursor: string | null, signal?: AbortSignal, statsClient: SdkClient | null = browserStatsClient()): Promise<HoldersPage> {
  return readHoldersPage(poolAddress, cursor, signal, statsClient);
}

/**
 * Holders for a pool from the stats host only: transfer-coverage page plus
 * share rows. Unsupported (`holdersHostUnset`) when no stats host is
 * configured, and when the stats host answers neither read.
 */
export function useHolders(poolAddress: string | null | undefined, cursor: string | null = null): Loadable<HoldersPage> {
  const statsClient = browserStatsClient();
  const pool = poolAddress && /^0x[0-9a-fA-F]{40}$/.test(poolAddress) ? poolAddress.toLowerCase() : null;
  const query = useMarketQuery({
    queryKey: queryKeys.holders(statsClient?.config.apiOrigin ?? '', pool ?? '', cursor),
    queryFn: ({ signal }) => loadHoldersPage(pool as string, cursor, signal, statsClient),
    enabled: pool !== null && statsClient !== null,
    ...marketCachePolicy.holders,
    retryOnMount: false,
  });
  if (statsClient === null) return holdersHostUnset;
  return loadable(query, 'IF_HOLDERS');
}
