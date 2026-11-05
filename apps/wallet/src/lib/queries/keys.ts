/** Shared identities for wallet reads, invalidations and reconciliation. */
import { scopedQueryKey } from './scope';

const lower = (addr: string | undefined | null) => (addr ?? '').toLowerCase();

export const queryKeys = {
  portfolio: (address: string, scope?: string) =>
    scopedQueryKey(['portfolio', lower(address)], scope),
  balance: (address: string, scope?: string) => scopedQueryKey(['balance', lower(address)], scope),
  paxPrice: (symbol = 'PAX', scope?: string) => scopedQueryKey(['paxPrice', symbol], scope),
  holdings: (address: string, scope?: string) =>
    scopedQueryKey(['holdings', lower(address)], scope),
  pnlHistory: (address: string, days: number, scope?: string) =>
    scopedQueryKey(['pnlHistory', lower(address), days], scope),
  portfolioChart: (address: string, period: string, scope?: string) =>
    scopedQueryKey(['portfolioChart', lower(address), period], scope),
  txHistory: (address: string, limit = 50, scope?: string) =>
    scopedQueryKey(['txHistory', lower(address), limit], scope),
  transfers: (address: string, scope?: string) =>
    scopedQueryKey(['transfers', lower(address)], scope),
  txCount: (address: string, scope?: string) => scopedQueryKey(['txCount', lower(address)], scope),
  txDetail: (hash: string, scope?: string) => scopedQueryKey(['txDetail', lower(hash)], scope),
  txTransfers: (hash: string, scope?: string) =>
    scopedQueryKey(['txTransfers', lower(hash)], scope),
  rankings: (category: string, limit = 20, scope?: string) =>
    scopedQueryKey(['rankings', category, limit], scope),
} as const;

export type WalletQueryKey = ReturnType<(typeof queryKeys)[keyof typeof queryKeys]>;
