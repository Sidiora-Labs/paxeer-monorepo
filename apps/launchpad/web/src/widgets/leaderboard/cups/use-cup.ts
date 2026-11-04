'use client';
import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/core/query-keys';
import { marketCachePolicy } from '@/core/cache-policy';
import { useOperations } from '@/core/market/operations';
import { loadable, type Loadable } from '@/core/market/unsupported';
import { cupOperations, cupStandingsLimit, fetchCupResults, fetchCupStandings, fetchCurrentCup, validateStandings, type Cup, type CupResults, type CupStanding, type CupStandingsPage } from './cups-contract';


/**
 * IF_CUP_CURRENT `GET /v2/cups/current`: typed in the SDK, no serving handler
 * yet. The real 404 becomes an Unsupported marker; a served `cup: null` is a
 * genuine "no competition scheduled" answer.
 */
export function useCurrentCup(): Loadable<Cup | null> {
  const operations = useOperations();
  const query = useQuery({
    queryKey: queryKeys.cupCurrent(),
    queryFn: ({ signal }) => fetchCurrentCup(operations, signal),
    ...marketCachePolicy.cups,
  });
  return loadable(query, cupOperations.current);
}

export function useCupResults(cupId: string): Loadable<CupResults> {
  const operations = useOperations();
  const query = useQuery({
    queryKey: queryKeys.cupResults(cupId),
    queryFn: ({ signal }) => fetchCupResults(operations, cupId, signal),
    ...marketCachePolicy.cups,
  });
  return loadable(query, cupOperations.results);
}

export function useCupStandings(cupId: string, cursor: string | null = null, limit: number = cupStandingsLimit): Loadable<CupStandingsPage> {
  const operations = useOperations();
  const query = useQuery({
    queryKey: queryKeys.cupStandings(cupId, cursor, limit),
    queryFn: ({ signal }) => fetchCupStandings(operations, cupId, cursor, limit, signal),
    ...marketCachePolicy.cups,
  });
  return loadable(query, cupOperations.standings);
}

interface StandingsContinuation {
  cursor: string | null;
  items: CupStanding[];
  cursors: string[];
}

export function useCupStandingsPages(cupId: string) {
  const operations = useOperations();
  return useInfiniteQuery({
    queryKey: [...queryKeys.cupStandings(cupId, null, cupStandingsLimit), 'pages'],
    initialPageParam: { cursor: null, items: [], cursors: [] } as StandingsContinuation,
    queryFn: async ({ pageParam, signal }) => {
      const page = await fetchCupStandings(operations, cupId, pageParam.cursor, cupStandingsLimit, signal);
      validateStandings([...pageParam.items, ...page.items], Number.MAX_SAFE_INTEGER);
      const previousRank = pageParam.items[pageParam.items.length - 1]?.rank ?? 0;
      if (page.items.some(row => row.rank <= previousRank)) throw new Error('Cup standings page order disagreement');
      if (page.nextCursor !== null && (page.nextCursor.length === 0 || page.nextCursor === pageParam.cursor || pageParam.cursors.includes(page.nextCursor))) {
        throw new Error('Cup standings cursor disagreement');
      }
      return page;
    },
    getNextPageParam: (page, pages, _lastParam, params): StandingsContinuation | undefined => page.nextCursor === null ? undefined : ({
      cursor: page.nextCursor,
      items: pages.flatMap(item => item.items),
      cursors: params.flatMap(param => param.cursor === null ? [] : [param.cursor]),
    }),
    ...marketCachePolicy.cups,
  });
}

/** Re-request every cup query (the Unsupported marker has no refetch handle of its own). */
export function useCupRetry(cupId: string | null = null): () => void {
  const client = useQueryClient();
  return () => {
    void client.invalidateQueries({ queryKey: queryKeys.cups() });
    if (cupId) void client.invalidateQueries({ queryKey: queryKeys.cup(cupId) });
  };
}
