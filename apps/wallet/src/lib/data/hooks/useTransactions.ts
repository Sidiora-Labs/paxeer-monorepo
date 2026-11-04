/** React hook — fetch paginated transactions for an address from Blockscout */

import { useState, useCallback, useRef, useEffect } from 'react';
import { getAddressTransactions } from '../api/addresses';
import type { AddressTxsParams } from '../api/addresses';
import type { PaginatedResponse } from '../types/common';
import type { Transaction } from '../types/transaction';

export interface UseTransactionsResult {
  transactions: Transaction[];
  nextPageParams: Record<string, unknown> | null;
  loading: boolean;
  error: Error | null;
  fetch: (addressHash: string, params?: AddressTxsParams) => Promise<void>;
  fetchNextPage: () => Promise<void>;
  refresh: () => Promise<void>;
}

export function useTransactions(): UseTransactionsResult {
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [nextPageParams, setNextPageParams] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const lastHashRef = useRef<string>('');
  const lastParamsRef = useRef<AddressTxsParams | undefined>(undefined);
  const requestIdRef = useRef(0);
  const nextPageRef = useRef<Record<string, unknown> | null>(null);
  const pendingRef = useRef(false);

  useEffect(
    () => () => {
      requestIdRef.current += 1;
    },
    []
  );

  const fetchData = useCallback(async (addressHash: string, params?: AddressTxsParams) => {
    const requestId = ++requestIdRef.current;
    if (lastHashRef.current !== addressHash) {
      setTransactions([]);
    }
    lastHashRef.current = addressHash;
    lastParamsRef.current = params;
    nextPageRef.current = null;
    setNextPageParams(null);
    pendingRef.current = Boolean(addressHash);
    if (!addressHash) {
      setLoading(false);
      setError(null);
      return;
    }
    setLoading(true);
    setError(null);

    try {
      const result: PaginatedResponse<Transaction> = await getAddressTransactions(
        addressHash,
        params
      );
      if (requestId !== requestIdRef.current) return;
      setTransactions(result.items);
      nextPageRef.current = result.next_page_params;
      setNextPageParams(result.next_page_params);
    } catch (err) {
      if (requestId !== requestIdRef.current) return;
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (requestId === requestIdRef.current) {
        pendingRef.current = false;
        setLoading(false);
      }
    }
  }, []);

  const fetchNextPage = useCallback(async () => {
    const pageParams = nextPageRef.current;
    if (!lastHashRef.current || !pageParams || pendingRef.current) return;
    const requestId = ++requestIdRef.current;
    pendingRef.current = true;
    setLoading(true);
    setError(null);

    try {
      const result: PaginatedResponse<Transaction> = await getAddressTransactions(
        lastHashRef.current,
        { ...lastParamsRef.current, ...pageParams } as AddressTxsParams
      );
      if (requestId !== requestIdRef.current) return;
      setTransactions((prev) => [...prev, ...result.items]);
      nextPageRef.current = result.next_page_params;
      setNextPageParams(result.next_page_params);
    } catch (err) {
      if (requestId !== requestIdRef.current) return;
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (requestId === requestIdRef.current) {
        pendingRef.current = false;
        setLoading(false);
      }
    }
  }, []);

  const refresh = useCallback(async () => {
    if (lastHashRef.current) {
      await fetchData(lastHashRef.current, lastParamsRef.current);
    }
  }, [fetchData]);

  return { transactions, nextPageParams, loading, error, fetch: fetchData, fetchNextPage, refresh };
}
