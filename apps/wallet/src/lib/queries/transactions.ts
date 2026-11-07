'use client';

/**
 * Transaction-history queries.
 *
 * Backed by `fetchPortfolioTransactions`, which returns both native txns and
 * token transfers in one call. We split the response into two derived shapes
 * the UI consumes directly so widgets don't have to re-shape on render.
 */

import { type UseQueryOptions } from '@tanstack/react-query';
import { useScopedQuery } from './scope';
import { fetchPortfolioTransactions } from '@/lib/api';
import type { WalletTransactionsResponse } from '../data';
import { queryKeys } from './keys';
import type { WalletTransaction, WalletTokenTransferItem } from '@/lib/data';

export interface TxHistoryRow {
  hash: string;
  fromAddress: string;
  toAddress: string;
  amountRaw: string;
  decimals: number;
  symbol: string;
  timestamp: string | null;
}

export interface TxHistoryResult {
  transactions: TxHistoryRow[];
  transfers: TxHistoryRow[];
  nextPageParams: WalletTransactionsResponse['next_page_params'];
  nextTransferPageParams: WalletTransactionsResponse['next_transfer_page_params'];
}

const PAX_DECIMALS = 18;
const PAX_SYMBOL = 'PAX';

function integerAmount(raw: string): string {
  if (typeof raw !== 'string' || !/^\d+$/.test(raw)) {
    throw new TypeError('Transaction amount_raw must be an unsigned integer string');
  }
  return raw;
}

function transferDecimals(decimals: number | null | undefined): number {
  if (decimals == null) return PAX_DECIMALS;
  if (!Number.isInteger(decimals) || decimals < 0 || decimals > 255) {
    throw new TypeError('Token decimals must be an integer from 0 to 255');
  }
  return decimals;
}

const normalizeTx = (tx: WalletTransaction): TxHistoryRow => ({
  hash: tx.tx_hash,
  fromAddress: tx.from_address,
  toAddress: tx.to_address ?? '',
  amountRaw: integerAmount(tx.value_raw),
  decimals: PAX_DECIMALS,
  symbol: PAX_SYMBOL,
  timestamp: tx.timestamp || null,
});

const normalizeTransfer = (transfer: WalletTokenTransferItem): TxHistoryRow => ({
  hash: transfer.tx_hash,
  fromAddress: transfer.from_address,
  toAddress: transfer.to_address,
  amountRaw: integerAmount(transfer.amount_raw),
  decimals: transferDecimals(transfer.token_decimals),
  symbol: transfer.token_symbol || '???',
  timestamp: transfer.timestamp || null,
});

/**
 * Native transactions + token transfers for the active wallet.
 * Disabled when no address is connected.
 */
export function useTxHistoryQuery(
  address: string | undefined,
  limit: number = 50,
  options?: Partial<UseQueryOptions<TxHistoryResult>>
) {
  return useScopedQuery<TxHistoryResult>({
    queryKey: queryKeys.txHistory(address ?? '', limit),
    queryFn: async () => {
      const data = await fetchPortfolioTransactions(address as string, limit);
      return {
        transactions: (data.transactions || []).map(normalizeTx),
        transfers: (data.token_transfers || []).map(normalizeTransfer),
        nextPageParams: data.next_page_params,
        nextTransferPageParams: data.next_transfer_page_params,
      };
    },
    enabled: Boolean(address),
    ...options,
  });
}
