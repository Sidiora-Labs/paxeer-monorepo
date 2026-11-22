'use client';
import { useMemo } from 'react';
import { dehydrate, HydrationBoundary, QueryClient } from '@tanstack/react-query';
import type { PoolSnapshot } from '@/core/market/pool-resolver';
import { queryKeys } from '@/core/query-keys';
import type { HoldersPage } from '@/core/market/holders-reader';
import type { InitialPoolStreams } from '@/core/clients/livestream-api';
import type { InitialChainRevenue } from '@/widgets/holder-rewards/chain-revenue-reader';
import TokenMarketLayout from '@/widgets/trading/token-market-layout';
import { snapshotError, type CoinSnapshotFailure } from './snapshot-failure';

export type { CoinSnapshotFailure } from './snapshot-failure';
export interface CoinInitialSnapshot {
  data: PoolSnapshot | null;
  failure: CoinSnapshotFailure | null;
  updatedAt: number;
  holders?: { origin: string; data: HoldersPage | null; failure: CoinSnapshotFailure | null };
  streams?: InitialPoolStreams;
  revenue?: InitialChainRevenue;
}
interface TokenModuleProps { poolAddress: string; initialSnapshot?: CoinInitialSnapshot; }

/** Coin page module; the layout resolves the pool or token address through usePoolSnapshot. */
export default function TokenModule({ poolAddress, initialSnapshot }: TokenModuleProps) {
  const state = useMemo(() => {
    if (!initialSnapshot) return undefined;
    const cache = new QueryClient();
    const queryKey = queryKeys.poolSnapshot(poolAddress);
    if (initialSnapshot.data) {
      cache.setQueryData(queryKey, initialSnapshot.data, { updatedAt: initialSnapshot.updatedAt });
      const metadata = initialSnapshot.data.metadata;
      if (metadata) cache.setQueryData(['metadata-v2', 125, initialSnapshot.data.pool.tokenAddress], metadata, { updatedAt: initialSnapshot.updatedAt });
    } else if (initialSnapshot.failure) {
      cache.getQueryCache().build(cache, { queryKey }).setState({
        status: 'error', error: snapshotError(initialSnapshot.failure, poolAddress), errorUpdatedAt: initialSnapshot.updatedAt, fetchStatus: 'idle',
      });
    }
    const holders = initialSnapshot.holders;
    if (holders && initialSnapshot.data) {
      const key = queryKeys.holders(holders.origin, initialSnapshot.data.pool.poolAddress, null);
      if (holders.data) cache.setQueryData(key, holders.data, { updatedAt: initialSnapshot.updatedAt });
      else if (holders.failure) cache.getQueryCache().build(cache, { queryKey: key }).setState({
        status: 'error', error: snapshotError(holders.failure, poolAddress), errorUpdatedAt: initialSnapshot.updatedAt, fetchStatus: 'idle',
      });
    }
    return dehydrate(cache, { shouldDehydrateQuery: () => true });
  }, [poolAddress, initialSnapshot]);
  return <HydrationBoundary state={state}><TokenMarketLayout poolAddress={poolAddress} initialStreams={initialSnapshot?.streams} initialRevenue={initialSnapshot?.revenue} /></HydrationBoundary>;
}
