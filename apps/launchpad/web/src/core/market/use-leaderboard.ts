'use client';
import { useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { OPERATION_READ_TIMEOUT_FLOOR_MS, type OperationInput, type V2ContractTypes } from '@kindle/sdk';
import { queryKeys } from '@/core/query-keys';
import { marketCachePolicy } from '@/core/cache-policy';
import { useMarketClient } from './client';
import { loadable, toUnsupported, type Loadable } from './unsupported';

export type LeaderboardsResponse = V2ContractTypes['LeaderboardsResponse'];
export type LeaderboardCategory = OperationInput<'IF_LEADERBOARDS'>['path']['category'];
export type LeaderboardWindow = OperationInput<'IF_LEADERBOARDS'>['query']['window'];

export const leaderboardLimit = 100;

/** Quarantine, category/window and wallet/rank identity guards for an IF_LEADERBOARDS response. */
export function validateLeaderboard(response: LeaderboardsResponse, category: LeaderboardCategory, window: LeaderboardWindow, limit: number): LeaderboardsResponse {
  const snapshot = response.data.snapshot;
  const rows = snapshot.entries;
  if (response.quality === 'quarantined' || response.watermark?.integrity === 'quarantined' || snapshot.watermark.integrity === 'quarantined') throw new Error('Leaderboard integrity quarantined');
  if (snapshot.category !== category || snapshot.window !== window) throw new Error('Leaderboard category or window disagreement');
  if (rows.length > limit || new Set(rows.map(row => row.wallet.toLowerCase())).size !== rows.length || new Set(rows.map(row => row.rank)).size !== rows.length) throw new Error('Leaderboard identity or rank disagreement');
  for (const row of rows) if (!/^0x[0-9a-fA-F]{40}$/.test(row.wallet) || !/^(0|[1-9][0-9]{0,77})$/.test(row.metricRaw)) throw new Error('Leaderboard entry shape disagreement');
  return response;
}

/**
 * IF_LEADERBOARDS shows the first unavailable answer while the cache retries
 * in the background. The cache owns retries so each attempt is one request.
 */
export function useLeaderboard(category: LeaderboardCategory, window: LeaderboardWindow, limit: number = leaderboardLimit, cursor?: string): Loadable<LeaderboardsResponse> {
  const client = useMarketClient();
  const queryKey = [...queryKeys.leaderboard(category, window, limit), ...(cursor ? [cursor] : [])];
  const key = JSON.stringify(queryKey);
  const settled = useRef<{ client: typeof client; key: string; value: Loadable<LeaderboardsResponse> } | null>(null);
  if (!Number.isSafeInteger(limit) || limit < 1 || limit > leaderboardLimit) throw new RangeError('Invalid leaderboard page bound');
  const query = useQuery({
    queryKey,
    queryFn: async ({ signal }) => {
      const params = new URLSearchParams({ window, limit: String(limit), ...(cursor ? { cursor } : {}) });
      const response = await client.requestV2(`/v2/leaderboards/${category}?${params}`, 'LeaderboardsResponse', {
        signal, idempotentRead: false, timeoutMs: OPERATION_READ_TIMEOUT_FLOOR_MS,
      });
      return validateLeaderboard(response, category, window, limit);
    },
    ...marketCachePolicy.leaderboard,
  });
  const previous = settled.current?.client === client && settled.current.key === key ? settled.current.value : null;
  const unavailable = query.status === 'pending' ? toUnsupported('IF_LEADERBOARDS', query.failureReason) : null;
  const current = unavailable ?? loadable(query, 'IF_LEADERBOARDS');
  const shown = current.state === 'loading' && previous ? previous : current;
  useEffect(() => {
    settled.current = shown.state === 'loading' ? null : { client, key, value: shown };
  }, [client, key, shown]);
  return shown;
}
