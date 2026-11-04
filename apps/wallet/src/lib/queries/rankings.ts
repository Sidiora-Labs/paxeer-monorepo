'use client';

import { useScopedQuery } from './scope';

import { fetchEnrichedRankings, type EnrichedRankedToken, type RankingCategory } from '@/lib/api';
import { queryKeys } from './keys';

export type { EnrichedRankedToken, RankingCategory };

export function useRankingsQuery(category: RankingCategory, limit = 20) {
  return useScopedQuery<EnrichedRankedToken[]>({
    queryKey: queryKeys.rankings(category, limit),
    queryFn: () => fetchEnrichedRankings(category, limit),
    staleTime: 30_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
  });
}
