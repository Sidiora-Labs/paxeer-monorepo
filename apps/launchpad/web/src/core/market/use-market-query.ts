'use client';
import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient, type QueryClient, type QueryKey, type UseQueryOptions, type UseQueryResult } from '@tanstack/react-query';
import { marketRetryPace } from '@/core/retry-pace';
import { isNetworkClassError, retryHint } from '@/core/transient-retry';

export function useMarketQuery<TQueryFnData = unknown, TError = Error, TData = TQueryFnData, TQueryKey extends QueryKey = QueryKey>(options: UseQueryOptions<TQueryFnData, TError, TData, TQueryKey>, client?: QueryClient): UseQueryResult<TData, TError> {
  const query = useQuery(options, client);
  const cache = useQueryClient(client);
  const key = JSON.stringify(options.queryKey);
  const retainedError = useRef<{ key: string; error: TError } | null>(null);
  const currentError = query.error ?? query.failureReason;
  if (currentError && marketRetryPace.state(currentError)) retainedError.current = { key, error: currentError };
  else if (query.isSuccess || retainedError.current?.key !== key) retainedError.current = null;
  const error = currentError ?? (query.isFetching ? retainedError.current?.error ?? null : null);
  const waiting = marketRetryPace.waiting(error);
  const [, refresh] = useState(0);
  useEffect(() => {
    if (!waiting) return;
    const timer = setTimeout(() => refresh(value => value + 1), waiting);
    return () => clearTimeout(timer);
  }, [error, waiting]);
  const refetch: typeof query.refetch = async refetchOptions => {
    await cache.cancelQueries({ queryKey: options.queryKey, exact: true });
    marketRetryPace.retryNow(error);
    return query.refetch(refetchOptions);
  };
  useEffect(() => {
    const visible = () => {
      if (document.visibilityState === 'visible' && options.enabled !== false && error) {
        void refetch({ cancelRefetch: false });
      }
    };
    document.addEventListener('visibilitychange', visible);
    return () => document.removeEventListener('visibilitychange', visible);
  }, [error, options.enabled, query.refetch]);
  const shortServerRetry = retryHint(error).owner === 'network' && !isNetworkClassError(error);
  if (query.data === undefined && error && marketRetryPace.state(error) && !shortServerRetry) {
    return { ...query, refetch, status: waiting ? 'pending' : 'error', isPending: waiting > 0, isError: waiting === 0, isSuccess: false, isLoading: waiting > 0 && query.isFetching, isInitialLoading: waiting > 0 && query.isFetching, error: waiting ? null : error } as UseQueryResult<TData, TError>;
  }
  return { ...query, refetch };
}
