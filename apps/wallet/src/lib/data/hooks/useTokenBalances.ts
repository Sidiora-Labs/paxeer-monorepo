/** React hook — fetch all token balances for an address from Blockscout */

import { useState, useCallback, useRef, useEffect } from 'react';
import { getAddressTokenBalances } from '../api/addresses';
import type { TokenBalance } from '../types/token';

export interface UseTokenBalancesResult {
  balances: TokenBalance[];
  loading: boolean;
  error: Error | null;
  fetch: (addressHash: string) => Promise<void>;
  refresh: () => Promise<void>;
}

export function useTokenBalances(): UseTokenBalancesResult {
  const [balances, setBalances] = useState<TokenBalance[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const lastHashRef = useRef<string>('');
  const requestIdRef = useRef(0);

  useEffect(
    () => () => {
      requestIdRef.current += 1;
    },
    []
  );

  const fetchData = useCallback(async (addressHash: string) => {
    const requestId = ++requestIdRef.current;
    if (lastHashRef.current !== addressHash) {
      setBalances([]);
    }
    lastHashRef.current = addressHash;
    if (!addressHash) {
      setLoading(false);
      setError(null);
      return;
    }
    setLoading(true);
    setError(null);

    try {
      const result = await getAddressTokenBalances(addressHash);
      if (requestId !== requestIdRef.current) return;
      setBalances(result);
    } catch (err) {
      if (requestId !== requestIdRef.current) return;
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, []);

  const refresh = useCallback(async () => {
    if (lastHashRef.current) {
      await fetchData(lastHashRef.current);
    }
  }, [fetchData]);

  return { balances, loading, error, fetch: fetchData, refresh };
}
