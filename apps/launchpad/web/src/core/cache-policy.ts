import { retryTransientOnly } from './transient-retry';
import { marketRetryPace } from './retry-pace';

type RetryQuery = { state: { error: unknown; fetchFailureReason: unknown } };
const marketRetryPolicy = {
  ...retryTransientOnly,
  retry: (failureCount: number, error: Error) => {
    const pace = marketRetryPace.state(error);
    return (!pace || pace.failures < 5 && marketRetryPace.waiting(error) > 0) && retryTransientOnly.retry(failureCount, error);
  },
  retryDelay: (failureCount: number, error: Error) => Math.max(retryTransientOnly.retryDelay(failureCount, error), marketRetryPace.delay(error) || 0),
  refetchInterval: (query: RetryQuery) => marketRetryPace.delay(query.state.error ?? query.state.fetchFailureReason),
  refetchOnWindowFocus: (query: RetryQuery) => {
    const error = query.state.error ?? query.state.fetchFailureReason;
    marketRetryPace.retryNow(error);
    return true;
  },
};

/**
 * Explicit stale-data tolerance policy (T03.2).
 *
 * On a trading UI, rendering stale data as if it were fresh is a correctness
 * bug — a price that is 60s old can mislead a trade decision. Previously each
 * `useQuery` picked its own `staleTime` as an undocumented magic number. This
 * module makes the tolerance an EXPLICIT, named decision so every query states
 * how stale its data may be, and the rationale lives in one place.
 *
 * Field meanings (TanStack Query semantics):
 *   - staleTime       how long fetched data is considered FRESH (no refetch on
 *                     mount / window-focus / reconnect while fresh).
 *   - gcTime          how long an UNUSED cache entry is kept before garbage
 *                     collection (formerly `cacheTime`).
 *   - refetchInterval background poll cadence; omitted = no polling.
 *
 * Tiers are ordered by how financially sensitive the data is:
 *
 *   REALTIME  Prices / pool stats that drive trade decisions. The live
 *             WebSocket stream is the primary source; this REST poll is the
 *             backstop, so the stale window is short and it polls.
 *   FAST      Frequently-changing aggregates (holders, recent transactions).
 *   STANDARD  User-action-driven data (profiles, balances) — refetch on focus.
 *   SLOW      Rarely-changing data (rankings, leaderboards).
 *   STATIC    Effectively immutable for a session (token metadata, config).
 *
 * Note: `refetchIntervalInBackground` is intentionally left `false` at the
 * QueryClient default so hidden tabs never poll; the WsManager already handles
 * resync-on-visible for live streams.
 */
export const cachePolicy = {
  REALTIME: { staleTime: 5_000, gcTime: 60_000, refetchInterval: 10_000 },
  FAST: { staleTime: 10_000, gcTime: 120_000, refetchInterval: 30_000 },
  STANDARD: { staleTime: 30_000, gcTime: 5 * 60_000 },
  SLOW: { staleTime: 5 * 60_000, gcTime: 30 * 60_000 },
  STATIC: { staleTime: Number.POSITIVE_INFINITY, gcTime: Number.POSITIVE_INFINITY },
} as const;

export type CacheTier = keyof typeof cachePolicy;

/**
 * Per-family tolerance for the typed market data layer (`core/market/*`).
 * Every family retries only failures the server marks transient (retryable
 * 503 / 429 with retryAfterMs, a dropped connection) on the server's delay,
 * bounded by core/transient-retry.ts. Any other failed contract request is
 * surfaced at once as the real error (or an Unsupported marker).
 * Live deltas refresh healthy pool-scoped families; failed resources retry
 * on the shared resource pace without discarding their last good values.
 */
export const marketCachePolicy = {
  poolSnapshot: { staleTime: 10_000, gcTime: 30_000, ...marketRetryPolicy },
  poolDetail: { staleTime: 10_000, gcTime: 30_000, ...marketRetryPolicy },
  statsBatch: { staleTime: 10_000, gcTime: 30_000, ...marketRetryPolicy },
  rankings: { staleTime: 10_000, gcTime: 30_000, ...marketRetryPolicy },
  candles: { staleTime: 15_000, gcTime: 60_000, ...marketRetryPolicy },
  sparkline: { staleTime: 60_000, gcTime: 5 * 60_000, ...marketRetryPolicy },
  holders: { staleTime: 10_000, gcTime: 30_000, ...marketRetryPolicy },
  profile: { staleTime: 60_000, gcTime: 10 * 60_000, ...marketRetryPolicy },
  walletIdentity: { staleTime: 5 * 60_000, gcTime: 30 * 60_000, ...marketRetryPolicy },
  leaderboard: {
    staleTime: 30_000, gcTime: 60_000, ...marketRetryPolicy,
    refetchOnReconnect: false, retryOnMount: false,
  },
  feed: { staleTime: 30_000, gcTime: 60_000, ...marketRetryPolicy },
  callouts: { staleTime: 30_000, gcTime: 60_000, ...marketRetryPolicy },
  watchlistRows: { staleTime: 10_000, gcTime: 30_000, ...marketRetryPolicy, retry: false },
  metadata: { staleTime: 30_000, gcTime: 30_000, ...marketRetryPolicy },
  bounties: { staleTime: 30_000, gcTime: 60_000, ...marketRetryPolicy },
  holderRewards: { staleTime: 30_000, gcTime: 60_000, ...marketRetryPolicy },
  cups: { staleTime: 30_000, gcTime: 60_000, ...marketRetryPolicy },
  profileReplies: { staleTime: 30_000, gcTime: 60_000, ...marketRetryPolicy },
  calloutHistory: { staleTime: 60_000, gcTime: 5 * 60_000, ...marketRetryPolicy },
} as const;

export type MarketCacheFamily = keyof typeof marketCachePolicy;
