'use client';

import { useQueryClient, type UseQueryOptions } from '@tanstack/react-query';
import { getWalletQueryScope, useScopedQuery, useWalletQueryScope } from './scope';
import {
  fetchPortfolio,
  fetchPortfolioFresh,
  fetchBalance,
  fetchBalanceFresh,
  fetchPaxPriceLatest,
} from '@/lib/api';
import type { BalanceResponse } from '@/lib/api';
import { queryKeys } from './keys';
import { selectedWalletChainId } from '../wallet-read-contract';

type PortfolioData = Awaited<ReturnType<typeof fetchPortfolio>>;
type PaxPriceData = Awaited<ReturnType<typeof fetchPaxPriceLatest>>;

/**
 * Wallet portfolio (token holdings + native balance). Powers the bento grid
 * on `PortfolioPage`. Disabled when no address is connected.
 */
export function usePortfolioQuery(
  address: string | undefined,
  options?: Partial<UseQueryOptions<PortfolioData>>
) {
  return useScopedQuery<PortfolioData>({
    queryKey: queryKeys.portfolio(address ?? ''),
    queryFn: () => fetchPortfolio(address as string),
    enabled: Boolean(address),
    ...options,
  });
}

/**
 * Latest PAX price (USD). Refreshes every 60s while mounted so the hero
 * card stays accurate without flooding the price API.
 */
export function usePaxPriceQuery(symbol: string = 'PAX') {
  return useScopedQuery<PaxPriceData>({
    queryKey: queryKeys.paxPrice(symbol),
    queryFn: () => fetchPaxPriceLatest(symbol),
    staleTime: 60_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
  });
}

/**
 * Daily PnL + USD totals from the wallet balance endpoint.
 */
export function useBalanceQuery(
  address: string | undefined,
  options?: Partial<UseQueryOptions<BalanceResponse | null>>
) {
  return useScopedQuery<BalanceResponse | null>({
    queryKey: queryKeys.balance(address ?? ''),
    queryFn: () => fetchBalance(address as string),
    enabled: Boolean(address),
    ...options,
  });
}

/**
 * Manual reconcile helper used after a send: bypass any cache layers and
 * push the fresh result back into the query cache so the UI snaps to truth.
 *
 * Returned function is stable for the lifetime of the calling component.
 */
export function useReconcilePortfolio() {
  const qc = useQueryClient();
  const scope = useWalletQueryScope();

  return async (address: string) => {
    if (getWalletQueryScope() !== scope) throw new Error('Wallet query network changed');
    const chainId = selectedWalletChainId();
    const portfolioKey = queryKeys.portfolio(address, scope);
    const balanceKey = queryKeys.balance(address, scope);
    const [freshPortfolio, freshBalance] = await Promise.all([
      fetchPortfolioFresh(address, { chainId }).catch(() => null),
      fetchBalanceFresh(address, { chainId }).catch(() => null),
    ]);

    if (getWalletQueryScope() !== scope) return;
    if (freshPortfolio) {
      qc.setQueryData(portfolioKey, freshPortfolio);
    }
    if (freshBalance) {
      qc.setQueryData(balanceKey, freshBalance);
    }
  };
}

/**
 * Apply an optimistic mutation to the cached portfolio without refetching.
 * Caller supplies a producer that returns the new portfolio shape.
 */
export function useOptimisticPortfolioUpdate() {
  const qc = useQueryClient();
  const scope = useWalletQueryScope();

  return (
    address: string,
    updater: (prev: PortfolioData | undefined) => PortfolioData | undefined
  ) => {
    if (getWalletQueryScope() !== scope) throw new Error('Wallet query network changed');
    qc.setQueryData<PortfolioData | undefined>(
      queryKeys.portfolio(address, scope),
      (prev: PortfolioData | undefined) => updater(prev)
    );
  };
}
