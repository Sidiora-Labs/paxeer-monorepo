'use client';
import { useQueryClient } from '@tanstack/react-query';
import { marketCachePolicy } from '@/core/cache-policy';
import { allTimeHighRefreshMs, allTimeHighWindow, loadAllTimeHighHistory, type AllTimeHighHistory } from './all-time-high';
import { useMarketQuery } from './use-market-query';

interface AllTimeHighRead { history: AllTimeHighHistory; readAt: number }

export function useAllTimeHigh(poolAddress: string, createdAt: string | null | undefined, inView: boolean) {
  const client = useQueryClient();
  const pool = /^0x[0-9a-fA-F]{40}$/.test(poolAddress) ? poolAddress.toLowerCase() : null;
  const covered = allTimeHighWindow(createdAt) !== null;
  const queryKey = ['production-market', 'all-time-high', pool, createdAt] as const;
  const query = useMarketQuery({
    queryKey,
    queryFn: async ({ signal }): Promise<AllTimeHighRead> => {
      const cached = client.getQueryData<AllTimeHighRead>(queryKey);
      if (cached && Date.now() - cached.readAt < allTimeHighRefreshMs) return cached;
      const history = await loadAllTimeHighHistory(pool as string, createdAt, { signal });
      return { history, readAt: Date.now() };
    },
    select: (read: AllTimeHighRead) => read.history,
    ...marketCachePolicy.candles,
    staleTime: allTimeHighRefreshMs,
    enabled: pool !== null && covered && inView,
    refetchInterval: query => Math.max(1, allTimeHighRefreshMs - (Date.now() - (query.state.data?.readAt ?? Date.now()))),
    refetchIntervalInBackground: false,
  });
  return { ...query, covered };
}
