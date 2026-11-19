import { MARKET_DATA_URL } from "./core/feeds";
import { fetchJsonWithPolicy } from "./core/net/fetch";
import { priceWindowChange } from "./core/perps/priceWindow";
import {
  marketFundingHourly,
  type MarketSnapshot,
} from "./hooks/perps/useMarketsStream";

export interface FeedTicker {
  paxeerSymbol: string;
  symbol: string;
  mark_price: number | null;
  index_price: number | null;
  price_24h: number | null;
  open_interest: number | null;
  last_funding_rate: number | null;
  last_funding_rate_hourly: string | null;
  updated_at: number | null;
  "24h_open": number | null;
  "24h_close": number | null;
  "24h_high": number | null;
  "24h_low": number | null;
  "24h_amount": number;
  "24h_volume": number;
}
export type FeedMarket = Omit<
  MarketSnapshot,
  "volume_7d" | "volume_30d" | "trades_count"
>;

export function normalizeFeedMarkets(
  rows: FeedTicker[],
  now = Date.now(),
): FeedMarket[] {
  return rows.flatMap((row) => {
    if (
      row.mark_price === null ||
      !Number.isFinite(row.mark_price) ||
      row.mark_price <= 0 ||
      row.updated_at == null ||
      !Number.isFinite(row.updated_at) ||
      now - row.updated_at > 30_000 ||
      row.updated_at > now + 5_000
    )
      return [];
    const hourly = marketFundingHourly(
      row.paxeerSymbol,
      "last_funding_rate_hourly",
      row.last_funding_rate_hourly,
    );
    return [
      {
        symbol: row.paxeerSymbol,
        symbol_full: row.symbol,
        price: row.mark_price,
        index_price: row.index_price,
        price_updated_at: row.updated_at,
        last_updated: row.updated_at,
        change_24h: priceWindowChange(row.mark_price, row.price_24h),
        volume_24h: row["24h_volume"],
        reported_volume_24h: row["24h_volume"],
        open_interest: row.open_interest ?? 0,
        reported_open_interest: row.open_interest ?? 0,
        funding_8h: hourly === null ? null : hourly * 8n,
        perp_funding_rate: hourly,
        last_funding_rate_hourly: hourly,
        price_24h:
          row.price_24h != null && row.price_24h > 0 ? row.price_24h : 0,
        high_24h: row["24h_high"],
        low_24h: row["24h_low"],
        amount_24h: row["24h_amount"],
      },
    ];
  });
}

let pending: Promise<FeedMarket[]> | null = null;
export function readAuthenticatedFeedMarkets(): Promise<FeedMarket[]> {
  if (!pending) {
    const request = fetchJsonWithPolicy<{
      success: boolean;
      data: FeedTicker[];
    }>(`${MARKET_DATA_URL}/tickers`)
      .then((body) => {
        if (!body.success || !Array.isArray(body.data))
          throw new Error("Ticker feed unavailable");
        return normalizeFeedMarkets(body.data);
      })
      .finally(() => {
        pending = null;
      });
    pending = request;
  }
  return pending ?? Promise.reject(new Error("Ticker feed unavailable"));
}
