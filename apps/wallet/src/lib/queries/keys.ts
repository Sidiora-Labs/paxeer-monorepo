/**
 * Centralised query keys for the wallet app.
 *
 * Always import keys from here so cache reads / invalidations / prefetches
 * stay aligned across files. Keys are typed as `as const` so TypeScript
 * narrows them to literal tuples — TanStack Query uses structural equality
 * on these tuples, so consistency matters.
 */

import { selectedWalletChainId } from '../wallet-read-contract';

const lower = (addr: string | undefined | null) => (addr ?? '').toLowerCase();

export const queryKeys = {
  portfolio: (address: string) => ['portfolio', lower(address), selectedWalletChainId()] as const,
  balance: (address: string) => ['balance', lower(address), selectedWalletChainId()] as const,
  paxPrice: (symbol = 'PAX') => ['paxPrice', symbol, selectedWalletChainId()] as const,

  holdings: (address: string) => ['holdings', lower(address), selectedWalletChainId()] as const,
  pnlHistory: (address: string, days: number) =>
    ['pnlHistory', lower(address), days, selectedWalletChainId()] as const,
  portfolioChart: (address: string, period: string) =>
    ['portfolioChart', lower(address), period, selectedWalletChainId()] as const,
  txHistory: (address: string, limit = 50) =>
    ['txHistory', lower(address), selectedWalletChainId(), limit] as const,
  transfers: (address: string) => ['transfers', lower(address), selectedWalletChainId()] as const,
  txCount: (address: string) => ['txCount', lower(address), selectedWalletChainId()] as const,
  txDetail: (hash: string) => ['txDetail', hash.toLowerCase(), selectedWalletChainId()] as const,
  rankings: (category: string) => ['rankings', category] as const,
} as const;

export type WalletQueryKey =
  | ReturnType<typeof queryKeys.portfolio>
  | ReturnType<typeof queryKeys.balance>
  | ReturnType<typeof queryKeys.paxPrice>
  | ReturnType<typeof queryKeys.holdings>
  | ReturnType<typeof queryKeys.pnlHistory>
  | ReturnType<typeof queryKeys.portfolioChart>
  | ReturnType<typeof queryKeys.txHistory>
  | ReturnType<typeof queryKeys.transfers>
  | ReturnType<typeof queryKeys.txCount>
  | ReturnType<typeof queryKeys.txDetail>
  | ReturnType<typeof queryKeys.rankings>;
