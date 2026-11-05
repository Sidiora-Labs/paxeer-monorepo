import { useEffect, useMemo, useState } from 'react';
import type { Session, User } from '@supabase/supabase-js';
import { PaxeerWallet } from './index.js';
import type {
  ChainInfo,
  FundedSelfResponse,
  FundedTier,
  PaxeerWalletConfig,
  PublicWallet,
} from './types.js';

/**
 * React bindings — opt-in. Pure functions, no context provider required.
 *
 * Pattern:
 *   const wallet = useMemo(() => new PaxeerWallet(config), [config]);
 *   const { user, session } = useSession(wallet);
 *   const { wallet: walletInfo } = useWallet(wallet);
 */

export interface UseSessionResult {
  session: Session | null;
  user: User | null;
  loading: boolean;
}

export function useSession(wallet: PaxeerWallet): UseSessionResult {
  const [session, setSession] = useState<Session | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    void (async () => {
      const s = await wallet.getSession();
      if (!alive) return;
      setSession(s);
      setUser(s?.user ?? null);
      setLoading(false);
    })();
    const unsub = wallet.onAuthStateChange((_event, s) => {
      setSession(s);
      setUser(s?.user ?? null);
    });
    return () => {
      alive = false;
      unsub();
    };
  }, [wallet]);

  return { session, user, loading };
}

export interface UseWalletResult {
  wallet: PublicWallet | null;
  chain: ChainInfo | null;
  loading: boolean;
  error: Error | null;
  /** Force a re-fetch (e.g. after a successful tx). */
  refresh: () => void;
}

function useAuthenticatedAccount(
  client: PaxeerWallet,
  mode: 'wallet' | 'standard',
  pollMs?: number
): UseStandardAccountResult {
  const [data, setData] = useState<{ wallet: PublicWallet; chain: ChainInfo } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let alive = true;
    let generation = 0;
    let request = 0;
    let principal: string | null = null;
    setData(null);
    setError(null);
    setLoading(true);

    async function fetchOnce(): Promise<void> {
      const currentGeneration = generation;
      const currentRequest = ++request;
      if (!principal) return;
      setLoading(true);
      const current = () => alive && generation === currentGeneration && request === currentRequest;
      try {
        const result = await (mode === 'wallet' ? client.getWallet() : client.getStandardSelf());
        if (!current()) return;
        setData(result);
        setError(null);
      } catch (err) {
        if (current()) {
          setData(null);
          setError(err as Error);
        }
      } finally {
        if (current()) setLoading(false);
      }
    }

    function acceptSession(session: Session | null): void {
      if (!alive) return;
      generation += 1;
      principal = session?.user.id ?? null;
      setData(null);
      setError(null);
      setLoading(principal !== null);
      // Supabase invokes auth callbacks while holding its session lock.
      // Start reads after the callback returns, and discard superseded work.
      const acceptedGeneration = generation;
      void Promise.resolve().then(() => {
        if (alive && generation === acceptedGeneration) return fetchOnce();
      });
    }

    const unsub = client.onAuthStateChange((_event, session) => acceptSession(session));
    const initialGeneration = generation;
    void client.getSession().then(
      (session) => {
        if (alive && generation === initialGeneration) acceptSession(session);
      },
      (err: Error) => {
        if (alive && generation === initialGeneration) {
          setError(err);
          setLoading(false);
        }
      }
    );
    const timer = pollMs && pollMs > 0 ? setInterval(() => void fetchOnce(), pollMs) : null;
    return () => {
      alive = false;
      generation += 1;
      unsub();
      if (timer) clearInterval(timer);
    };
  }, [client, mode, tick, pollMs]);

  return useMemo(
    () => ({ data, loading, error, refresh: () => setTick((t) => t + 1) }),
    [data, loading, error]
  );
}

export function useWallet(client: PaxeerWallet): UseWalletResult {
  const { data, loading, error, refresh } = useAuthenticatedAccount(client, 'wallet');
  return useMemo(
    () => ({ wallet: data?.wallet ?? null, chain: data?.chain ?? null, loading, error, refresh }),
    [data, loading, error, refresh]
  );
}

export function usePaxeerWallet(config: PaxeerWalletConfig): PaxeerWallet {
  return useMemo(
    () => new PaxeerWallet(config),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [config.apiUrl, config.supabaseUrl, config.supabaseAnonKey]
  );
}

/* ============================================================================
 * Standard wallet (passive read)
 * ========================================================================== */

export interface UseStandardAccountOptions {
  pollMs?: number;
}

export interface UseStandardAccountResult {
  /** `null` if the user is signed in but has no standard wallet yet. */
  data: { wallet: PublicWallet; chain: ChainInfo } | null;
  loading: boolean;
  error: Error | null;
  /** Force a re-fetch (e.g. immediately after `provisionStandardWallet`). */
  refresh: () => void;
}

/**
 * Like `useWallet` but does NOT auto-provision. Returns `null` if the signed-in
 * user has no standard wallet, so the calling UI can offer an explicit "create"
 * action instead of silently producing one.
 *
 * Use this whenever the wallet creation needs to be a deliberate user choice
 * (e.g. a chooser screen between standard and funded accounts).
 */
export function useStandardAccount(
  client: PaxeerWallet,
  options: UseStandardAccountOptions = {}
): UseStandardAccountResult {
  return useAuthenticatedAccount(client, 'standard', options.pollMs);
}

/* ============================================================================
 * Funded accounts
 * ========================================================================== */

export interface UseFundedTiersResult {
  tiers: FundedTier[];
  loading: boolean;
  error: Error | null;
}

/**
 * Fetch the public list of active funded tiers + their whitelists. Pure
 * server data; we only fetch once on mount (tiers are stable; refresh by
 * remounting if you change them mid-session).
 */
export function useFundedTiers(client: PaxeerWallet): UseFundedTiersResult {
  const [tiers, setTiers] = useState<FundedTier[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const res = await client.listFundedTiers();
        if (!alive) return;
        setTiers(res.tiers);
        setError(null);
      } catch (err) {
        if (alive) setError(err as Error);
      } finally {
        if (alive) setLoading(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, [client]);

  return { tiers, loading, error };
}

export interface UseFundedAccountOptions {
  /**
   * Auto-refresh interval in ms. Set to a positive number (e.g. 5000) to
   * watch the evaluator update peak / status / drawdown headroom live;
   * leave undefined or 0 to fetch once and require explicit `refresh()`.
   */
  pollMs?: number;
}

export interface UseFundedAccountResult {
  /** `null` if the user has not provisioned a funded account yet. */
  data: FundedSelfResponse | null;
  loading: boolean;
  error: Error | null;
  /** Force a re-fetch (e.g. immediately after `provisionFundedAccount`). */
  refresh: () => void;
}

/**
 * React hook that mirrors `client.getFundedSelf()` and optionally polls so
 * the UI stays in sync with the evaluator (which ticks every 10s server-side).
 *
 * Returns `data: null` when the user is signed in but has no funded account
 * yet — that's the cue to show the "Provision Funded Account" CTA.
 */
export function useFundedAccount(
  client: PaxeerWallet,
  options: UseFundedAccountOptions = {}
): UseFundedAccountResult {
  const { pollMs } = options;
  const [data, setData] = useState<FundedSelfResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let alive = true;
    let timer: ReturnType<typeof setInterval> | null = null;

    async function fetchOnce(): Promise<void> {
      try {
        // Cheap guard: if the user is signed out we don't burn an API call.
        const session = await client.getSession();
        if (!session) {
          if (alive) {
            setData(null);
            setError(null);
            setLoading(false);
          }
          return;
        }
        const res = await client.getFundedSelf();
        if (!alive) return;
        setData(res);
        setError(null);
      } catch (err) {
        if (alive) setError(err as Error);
      } finally {
        if (alive) setLoading(false);
      }
    }

    void fetchOnce();
    if (pollMs && pollMs > 0) {
      timer = setInterval(() => {
        void fetchOnce();
      }, pollMs);
    }
    return () => {
      alive = false;
      if (timer) clearInterval(timer);
    };
  }, [client, tick, pollMs]);

  return useMemo(
    () => ({ data, loading, error, refresh: () => setTick((t) => t + 1) }),
    [data, loading, error]
  );
}
