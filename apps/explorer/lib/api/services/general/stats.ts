import type { HomeStats } from 'types/api/stats';
import type { TransactionsStats } from 'types/api/transaction';

export type ExplorerStats = HomeStats & {
  secondary_coin_market_cap?: string | null;
  secondary_coin_price_change_percentage?: number | null;
};

export type ExplorerTransactionsStats = TransactionsStats & {
  total_transactions_count?: string;
  price_updates_count_24h?: string;
  total_price_updates_count?: string;
};
