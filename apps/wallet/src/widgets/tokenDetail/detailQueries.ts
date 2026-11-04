'use client';

import { useQuery } from '@tanstack/react-query';
import { fetchPaxHistory, fetchPortfolioTransactions, type OhlcCandle } from '@/lib/api';
import { PAXEER_CONFIG } from '@/lib/constants';
import type { TxHistoryRow } from '@/lib/queries';

const DAY_MS = 24 * 60 * 60 * 1000;
const CANDLE_MS = 15 * 60 * 1000;

export interface MarketWindow {
  from: number;
  to: number;
  intervalMs: number;
}

export interface MarketSeries {
  candles: OhlcCandle[];
  window: MarketWindow;
}

export function useMarketDayQuery(
  tokenId: string,
  chainId: number | null,
  symbol: 'PAX' | 'WPAX' | null
) {
  return useQuery<MarketSeries>({
    queryKey: ['tokenDetailMarketDay', chainId, tokenId.toLowerCase(), symbol],
    queryFn: async () => {
      if (!symbol) throw new Error('A market symbol is required');
      const data = await fetchPaxHistory(symbol, 15, 97);
      // Use the last complete candle boundary, including one extra bucket in
      // the request so the rolling 24h window is covered at any refresh time.
      const to = Math.floor(data.lastUpdated / CANDLE_MS) * CANDLE_MS;
      return {
        candles: data.ohlc,
        window: { from: to - DAY_MS, to, intervalMs: CANDLE_MS },
      };
    },
    enabled: symbol !== null,
    staleTime: 30_000,
  });
}

export interface TokenDetailHistoryRow extends Omit<TxHistoryRow, 'decimals'> {
  decimals: number | null;
  chainId: number;
  tokenAddress: string | null;
}

export interface TokenDetailHistory {
  transactions: TokenDetailHistoryRow[];
  transfers: TokenDetailHistoryRow[];
}

export function supportedTokenDecimals(value: unknown): number | null {
  // The amount consumers use ethers, whose fixed-point precision is 0..80.
  return typeof value === 'number' && Number.isInteger(value) && value >= 0 && value <= 80
    ? value
    : null;
}

export function useTokenDetailHistoryQuery(address: string | undefined, chainId: number | null) {
  return useQuery<TokenDetailHistory>({
    queryKey: ['tokenDetailHistory', chainId, address?.toLowerCase() ?? ''],
    queryFn: async () => {
      // The current explorer facade is a root-chain source. Never relabel its
      // rows with a selected virtual chain's identity.
      if (!address || chainId !== PAXEER_CONFIG.chainId) {
        throw new Error('Token history is unavailable for this network');
      }
      const data = await fetchPortfolioTransactions(address);
      return {
        transactions: data.transactions.map((tx) => ({
          hash: tx.tx_hash,
          fromAddress: tx.from_address,
          toAddress: tx.to_address ?? '',
          amountRaw: tx.value_raw,
          decimals: 18,
          symbol: 'PAX',
          timestamp: tx.timestamp || null,
          chainId,
          tokenAddress: null,
        })),
        transfers: data.token_transfers.map((transfer) => {
          const decimals = supportedTokenDecimals(transfer.token_decimals);
          return {
            hash: transfer.tx_hash,
            fromAddress: transfer.from_address,
            toAddress: transfer.to_address,
            amountRaw: transfer.amount_raw,
            decimals,
            symbol: transfer.token_symbol ?? '???',
            timestamp: transfer.timestamp || null,
            chainId,
            tokenAddress: transfer.token_address.toLowerCase(),
          };
        }),
      };
    },
    enabled: Boolean(address) && chainId === PAXEER_CONFIG.chainId,
  });
}
