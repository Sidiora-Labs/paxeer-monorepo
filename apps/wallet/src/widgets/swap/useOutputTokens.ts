'use client';

/**
 * Swap **output** token list — paxscan top tokens + launchpad tokens, merged
 * with the static SWAP_TOKENS fallback. Native PAX is always first.
 *
 * Cached by TanStack Query so navigating away and back doesn't refetch.
 */

import { useQuery } from '@tanstack/react-query';
import { PAX_ICON_URL } from '@/lib/constants';
import { fetchTopTokens } from '@/lib/api';
import {
  fetchLaunchpadTokens,
  getCachedLaunchpadTokens,
  SWAP_TOKENS,
  type SwapToken,
} from '@/lib/swap';
import { paxscanToSwapToken, supportedTokenDecimals } from './util';

const QUERY_KEY = ['swapOutputTokens'] as const;
const LIST_TTL = 60 * 60_000; // must match hlpmm-v2.ts LIST_TTL

async function loadOutputTokens(): Promise<SwapToken[]> {
  const [paxscanTokens, launchpad] = await Promise.all([
    fetchTopTokens(),
    fetchLaunchpadTokens().catch(() => []),
  ]);

  return mergeOutputTokens(
    paxscanTokens.map(paxscanToSwapToken).filter((token): token is SwapToken => token !== null),
    launchpad
  );
}

export function mergeOutputTokens(catalogue: SwapToken[], launchpad: SwapToken[]): SwapToken[] {
  const seen = new Set<string>(['native']);
  const merged: SwapToken[] = [
    {
      symbol: 'PAX',
      name: 'Paxeer',
      address: '',
      decimals: 18,
      isNative: true,
      iconUrl: PAX_ICON_URL,
    },
  ];
  for (const t of [...catalogue, ...launchpad, ...SWAP_TOKENS]) {
    const key = t.isNative ? 'native' : t.address.toLowerCase();
    if (!seen.has(key) && supportedTokenDecimals(t.decimals) !== null) {
      seen.add(key);
      merged.push(t);
    }
  }
  return merged;
}

export function useOutputTokens(): SwapToken[] {
  const query = useQuery<SwapToken[]>({
    queryKey: QUERY_KEY,
    queryFn: loadOutputTokens,
    // Token lists are semi-static — revalidate at most once per hour.
    staleTime: LIST_TTL,
    gcTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    // Serve the localStorage-persisted list immediately so the UI renders
    // without any network round-trip. TanStack treats this as fresh until
    // initialDataUpdatedAt + staleTime has elapsed.
    initialData: () => {
      const cached = getCachedLaunchpadTokens();
      return cached ? mergeOutputTokens([], cached.tokens) : undefined;
    },
    initialDataUpdatedAt: () => getCachedLaunchpadTokens()?.updatedAt ?? 0,
  });
  return query.data ?? SWAP_TOKENS;
}
