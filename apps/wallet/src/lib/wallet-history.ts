import type { AddressTxsParams, AddressTokenTransfersParams } from './data/api/addresses';
import { selectedWalletChainId } from './wallet-read-contract';
import { CHAIN_ID } from './swap/sdk/addresses';

export interface WalletHistoryCursor {
  transactions?: Record<string, unknown> | null;
  transfers?: Record<string, unknown> | null;
}

export function boundedHistoryParams(
  limit: number,
  cursor?: Record<string, unknown> | null
): Record<string, string | number | boolean> {
  if (!Number.isSafeInteger(limit) || limit < 1 || limit > 50) {
    throw new RangeError('Wallet history page size must be between 1 and 50');
  }
  const result: Record<string, string | number | boolean> = {};
  const entries = Object.entries(cursor ?? {});
  if (entries.length > 16) throw new TypeError('History cursor is too large');
  for (const [key, value] of entries) {
    if (
      !/^[a-z][a-z0-9_]{0,63}$/.test(key) ||
      ['chain_id', 'fresh', 'filter', 'sort', 'order', 'type', 'token'].includes(key)
    ) {
      throw new TypeError('History cursor contains a reserved or invalid key');
    }
    if (key === 'limit') continue;
    if (
      !(
        (typeof value === 'string' && value.length <= 512) ||
        (typeof value === 'number' && Number.isSafeInteger(value)) ||
        typeof value === 'boolean'
      )
    ) {
      throw new TypeError('History cursor value is invalid');
    }
    result[key] = value;
  }
  return { ...result, limit };
}

export function walletHistoryParams(
  limit: number,
  cursor?: WalletHistoryCursor
): { transactions: AddressTxsParams; transfers: AddressTokenTransfersParams } {
  if (
    cursor !== undefined &&
    (typeof cursor !== 'object' || cursor === null || Array.isArray(cursor))
  ) {
    throw new TypeError(
      'Wallet history uses independent transaction and transfer cursors, not offsets'
    );
  }
  return {
    transactions: boundedHistoryParams(limit, cursor?.transactions),
    transfers: boundedHistoryParams(limit, cursor?.transfers),
  };
}

export function selectedSidChainId(): number {
  const chainId = selectedWalletChainId();
  if (chainId !== CHAIN_ID) throw new Error('SID is not registered on the selected chain');
  return chainId;
}
