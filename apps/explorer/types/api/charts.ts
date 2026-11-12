export interface ChartTransactionItem {
  date: string;
  transactions_count: number | null;
}

export interface ChartMarketItem {
  date: string;
  closing_price: string | null;
  market_cap?: string | null;
  tvl?: string | null;
}

export interface ChartTransactionResponse {
  chart_data: Array<ChartTransactionItem>;
}

export interface ChartMarketResponse {
  available_supply: string;
  chart_data: Array<ChartMarketItem>;
}

export type ChartSecondaryCoinMarketItem = Pick<ChartMarketItem, 'date' | 'closing_price' | 'market_cap'>;

export interface ChartSecondaryCoinMarketResponse {
  available_supply: string;
  chart_data: Array<ChartSecondaryCoinMarketItem>;
}
