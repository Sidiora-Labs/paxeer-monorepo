'use client';

import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { RECONCILE_DELAY_MS, subtractFromRawBalance } from '@/lib/optimistic';
import { formatBalance } from '@/lib/format';
import { fetchPortfolioFresh, fetchBalanceFresh } from '@/lib/api';
import { queryKeys } from '@/lib/queries';
import { PAXEER_CONFIG } from '@/lib/constants';
import { useActiveNetwork } from '@/components/NetworkSwitcher';
import { readTokenPrecision } from '@/widgets/send/useSendableTokens';
import { scopedPendingSendsRepository } from './pendingSends';

type PortfolioData = Awaited<ReturnType<typeof fetchPortfolioFresh>>;
const MAX_OPTIMISTIC_AGE_MS = 120_000;

export interface UseApplyPendingSendOptions {
  address: string | undefined;
  loading: boolean;
  optimisticBlockUntilRef: React.MutableRefObject<number>;
}

export function useApplyPendingSend({
  address,
  loading,
  optimisticBlockUntilRef,
}: UseApplyPendingSendOptions) {
  const client = useQueryClient();
  const { chainId } = useActiveNetwork();
  const appliedRef = useRef(new Set<string>());

  useEffect(() => {
    appliedRef.current = new Set();
    optimisticBlockUntilRef.current = 0;
    return () => {
      optimisticBlockUntilRef.current = 0;
    };
  }, [address, chainId, optimisticBlockUntilRef]);

  useEffect(() => {
    // The shared portfolio API and cache currently serve only the root indexer.
    // Keep virtual-chain entries durable until its owning domain adds chain keys.
    if (!address || chainId !== PAXEER_CONFIG.chainId) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const matching = () =>
      Object.entries(scopedPendingSendsRepository.read()).filter(
        ([, { event }]) =>
          event.sender.toLowerCase() === address.toLowerCase() && event.chainId === chainId
      );

    for (const [key, recovery] of matching()) {
      const { event } = recovery;
      if (recovery.reconcileAt !== null || loading || appliedRef.current.has(key)) continue;
      const previous = client.getQueryData<PortfolioData>(queryKeys.portfolio(address));
      if (!previous) continue;
      const reconcileAt = Date.now() + RECONCILE_DELAY_MS;
      scopedPendingSendsRepository.update((records) => ({
        ...records,
        [key]: { event, reconcileAt },
      }));
      appliedRef.current.add(key);
      // Stale events reconcile without subtracting from a potentially fresh balance.
      if (Date.now() - event.timestamp > MAX_OPTIMISTIC_AGE_MS) continue;
      client.setQueryData<PortfolioData>(queryKeys.portfolio(address), (prev) => {
        if (!prev) return prev;
        if (!event.tokenAddress) {
          const raw = subtractFromRawBalance(prev.native_balance.balance_raw, event.amount, 18);
          return {
            ...prev,
            native_balance: {
              ...prev.native_balance,
              balance_raw: raw,
              balance: formatBalance(raw, 18, 8),
            },
          };
        }
        return {
          ...prev,
          token_holdings: prev.token_holdings.map((holding) => {
            if (holding.contract_address.toLowerCase() !== event.tokenAddress?.toLowerCase())
              return holding;
            const decimals = readTokenPrecision(holding.decimals ?? null);
            if (decimals === null || decimals !== event.decimals) return holding;
            const raw = subtractFromRawBalance(holding.balance_raw, event.amount, decimals);
            return {
              ...holding,
              balance_raw: raw,
              balance: decimals === 0 ? raw : formatBalance(raw, decimals, 8),
            };
          }),
        };
      });
    }

    const schedule = () => {
      if (cancelled) return;
      const entries = matching().filter(([, value]) => value.reconcileAt !== null);
      if (!entries.length) {
        optimisticBlockUntilRef.current = 0;
        return;
      }
      const dueAt = Math.min(...entries.map(([, value]) => value.reconcileAt!));
      optimisticBlockUntilRef.current =
        Math.max(...entries.map(([, value]) => value.reconcileAt!)) + 5_000;
      timer = setTimeout(
        async () => {
          const dueKeys = new Set(
            entries.filter(([, value]) => value.reconcileAt! <= Date.now()).map(([key]) => key)
          );
          try {
            const [portfolio, balance] = await Promise.all([
              fetchPortfolioFresh(address),
              fetchBalanceFresh(address),
            ]);
            if (cancelled) return;
            client.setQueryData(queryKeys.portfolio(address), portfolio);
            client.setQueryData(queryKeys.balance(address), balance);
            scopedPendingSendsRepository.update((records) =>
              Object.fromEntries(Object.entries(records).filter(([key]) => !dueKeys.has(key)))
            );
          } catch {
            if (cancelled) return;
            // A failed reconciliation stays durable and gets another bounded delay.
            scopedPendingSendsRepository.update((records) =>
              Object.fromEntries(
                Object.entries(records).map(([key, value]) => [
                  key,
                  dueKeys.has(key)
                    ? { ...value, reconcileAt: Date.now() + RECONCILE_DELAY_MS }
                    : value,
                ])
              )
            );
          }
          schedule();
        },
        Math.max(0, dueAt - Date.now())
      );
    };
    schedule();
    return () => {
      cancelled = true;
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [address, chainId, loading, client, optimisticBlockUntilRef]);
}
