'use client';

import { useSyncExternalStore } from 'react';
import {
  useQuery,
  useQueryClient,
  type QueryKey,
  type UseQueryOptions,
} from '@tanstack/react-query';
import { getActiveRpcUrl, PAXEER_CONFIG, RPC_CHANGED_EVENT } from '@/lib/constants';
import { networkForChainId } from '@/lib/networks';
import { preferencesRepository } from '@/platform/storage/repositories';

export function getWalletQueryScope(): string {
  const preferences = preferencesRepository.read();
  const network = networkForChainId(preferences.activeChainId, preferences.networks);
  return JSON.stringify([network.chainId, network.kind, getActiveRpcUrl()]);
}

const serverScope = () => JSON.stringify([PAXEER_CONFIG.chainId, 'root', PAXEER_CONFIG.rpcUrl]);

export function scopedQueryKey<const T extends QueryKey>(key: T, scope = getWalletQueryScope()) {
  return ['wallet', scope, ...key] as const;
}

export async function readInQueryScope<T>(
  scope: string,
  signal: AbortSignal,
  producer: () => Promise<T>
): Promise<T> {
  signal.throwIfAborted();
  if (getWalletQueryScope() !== scope) throw new Error('Wallet query network changed');
  let abort: () => void = () => {};
  const cancelled = new Promise<never>((_, reject) => {
    abort = () => reject(signal.reason);
    signal.addEventListener('abort', abort, { once: true });
  });
  try {
    const result = await Promise.race([producer(), cancelled]);
    signal.throwIfAborted();
    if (getWalletQueryScope() !== scope) throw new Error('Wallet query network changed');
    return result;
  } finally {
    signal.removeEventListener('abort', abort);
  }
}

export function useWalletQueryScope() {
  const client = useQueryClient();
  return useSyncExternalStore(
    (notify) => {
      let previous = getWalletQueryScope();
      const refresh = () => {
        const next = getWalletQueryScope();
        if (next !== previous) {
          void client.cancelQueries({ queryKey: ['wallet', previous] });
          previous = next;
          notify();
        }
      };
      window.addEventListener(RPC_CHANGED_EVENT, refresh);
      window.addEventListener('storage', refresh);
      return () => {
        window.removeEventListener(RPC_CHANGED_EVENT, refresh);
        window.removeEventListener('storage', refresh);
      };
    },
    getWalletQueryScope,
    serverScope
  );
}

export function useScopedQuery<T>(options: UseQueryOptions<T>) {
  const scope = useWalletQueryScope();
  const producer = options.queryFn;
  return useQuery<T>({
    ...options,
    queryKey: scopedQueryKey(
      options.queryKey[0] === 'wallet' ? options.queryKey.slice(2) : options.queryKey,
      scope
    ),
    queryFn: (context) => {
      if (typeof producer !== 'function') throw new TypeError('Wallet query requires a producer');
      return readInQueryScope(scope, context.signal, async () => producer(context));
    },
  });
}
