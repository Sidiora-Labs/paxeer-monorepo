/** React hook — fetch paginated token transfers for an address from Blockscout */

import { useState, useCallback, useRef, useEffect } from 'react';
import { getAddressTokenTransfers } from '../api/addresses';
import type { AddressTokenTransfersParams } from '../api/addresses';
import type { PaginatedResponse } from '../types/common';
import type { TokenTransfer } from '../types/token';

export interface UseTokenTransfersResult {
  transfers: TokenTransfer[];
  nextPageParams: Record<string, unknown> | null;
  loading: boolean;
  error: Error | null;
  fetch: (addressHash: string, params?: AddressTokenTransfersParams) => Promise<void>;
  fetchNextPage: () => Promise<void>;
  refresh: () => Promise<void>;
}

export function useTokenTransfers(): UseTokenTransfersResult {
  const [transfers, setTransfers] = useState<TokenTransfer[]>([]);
  const [nextPageParams, setNextPageParams] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const lastHashRef = useRef<string>('');
  const lastParamsRef = useRef<AddressTokenTransfersParams | undefined>(undefined);
  const requestIdRef = useRef(0);
  const nextPageRef = useRef<Record<string, unknown> | null>(null);
  const pendingRef = useRef(false);

  useEffect(
    () => () => {
      requestIdRef.current += 1;
    },
    []
  );

  const fetchData = useCallback(
    async (addressHash: string, params?: AddressTokenTransfersParams) => {
      const requestId = ++requestIdRef.current;
      if (lastHashRef.current !== addressHash) {
        setTransfers([]);
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
        const result: PaginatedResponse<TokenTransfer> = await getAddressTokenTransfers(
          addressHash,
          params
        );
        if (requestId !== requestIdRef.current) return;
        setTransfers(result.items);
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
    },
    []
  );

  const fetchNextPage = useCallback(async () => {
    const pageParams = nextPageRef.current;
    if (!lastHashRef.current || !pageParams || pendingRef.current) return;
    const requestId = ++requestIdRef.current;
    pendingRef.current = true;
    setLoading(true);
    setError(null);

    try {
      const result: PaginatedResponse<TokenTransfer> = await getAddressTokenTransfers(
        lastHashRef.current,
        { ...lastParamsRef.current, ...pageParams } as AddressTokenTransfersParams
      );
      if (requestId !== requestIdRef.current) return;
      setTransfers((prev) => [...prev, ...result.items]);
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

  return { transfers, nextPageParams, loading, error, fetch: fetchData, fetchNextPage, refresh };
}
