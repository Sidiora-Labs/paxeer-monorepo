'use client';

import { fetchTxDetail, fetchTxTokenTransfers } from '@/lib/api';
import { queryKeys } from './keys';
import { useScopedQuery } from './scope';

type TransactionTransfers = Awaited<ReturnType<typeof fetchTxTokenTransfers>>;
export type TransferStatus = 'pending' | 'complete' | 'unavailable';

export interface TxDetailResult {
  tx: Awaited<ReturnType<typeof fetchTxDetail>>;
  tokenTransfers: TransactionTransfers['items'];
  transferStatus: TransferStatus;
  transferError: Error | null;
}

export function useTxTokenTransfersQuery(txHash: string | undefined) {
  return useScopedQuery<TransactionTransfers>({
    queryKey: queryKeys.txTransfers(txHash ?? ''),
    queryFn: async () => {
      const result = await fetchTxTokenTransfers(txHash as string);
      if (!result || !Array.isArray(result.items)) {
        throw new TypeError('Invalid transaction token transfer response');
      }
      return result;
    },
    enabled: Boolean(txHash),
    staleTime: 30_000,
    retry: 1,
  });
}

export function useTxDetailQuery(txHash: string | undefined) {
  const transaction = useScopedQuery({
    queryKey: queryKeys.txDetail(txHash ?? ''),
    queryFn: () => fetchTxDetail(txHash as string),
    enabled: Boolean(txHash),
    staleTime: 30_000,
    retry: 1,
  });
  const transfers = useTxTokenTransfersQuery(txHash);
  const transferStatus: TransferStatus = transfers.isError
    ? 'unavailable'
    : transfers.isPending
      ? 'pending'
      : 'complete';
  const data: TxDetailResult | undefined = transaction.data
    ? {
        tx: transaction.data,
        tokenTransfers: transfers.data?.items ?? [],
        transferStatus,
        transferError: transfers.error,
      }
    : undefined;
  return {
    ...transaction,
    data,
    error: transaction.error ?? transfers.error,
    isError: transaction.isError || transfers.isError,
    isLoading: transaction.isLoading || transfers.isLoading,
    refetchTransfers: transfers.refetch,
  };
}
